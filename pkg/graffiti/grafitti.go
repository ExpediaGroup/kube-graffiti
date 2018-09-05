// Package graffiti decideds whether an object should be graffiti'd, according to a rule, and then produces and JSON patch with the desired modification.
// Mutate either kubernetes admission request objects or plain raw objects.
package graffiti

import (
	//"stash.hcom/run/istio-namespace-webhook/pkg/log"
	"encoding/json"
	"fmt"

	// "github.com/davecgh/go-spew/spew"

	"github.com/rs/zerolog"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	labels "k8s.io/apimachinery/pkg/labels"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

const (
	componentName = "grafitti"
)

// BooleanOperator defines the logical boolean operator applied to label and field selector results.
// It is AND by default, i.e. both label selector and field selector must match to
type BooleanOperator int

const (
	AND BooleanOperator = iota
	OR
	XOR
)

// Rule contains a single graffiti rule and contains matchers for choosing which objects to change and additions which are the fields we want to add.
// It does not have mapstructure tags because it is not directly marshalled from config
type Rule struct {
	Name      string
	Matchers  Matchers
	Additions Additions
}

// Matchers manages the rules of matching an object
// This type is directly marshalled from config and so has mapstructure tags
type Matchers struct {
	LabelSelectors  []string        `mapstructure:"label-selectors"`
	FieldSelectors  []string        `mapstructure:"field-selectors"`
	BooleanOperator BooleanOperator `mapstructure:"boolean-operator"`
}

// Additions contains the additional fields that we want to insert into the object
// This type is directly marshalled from config and so has mapstructure tags
type Additions struct {
	Annotations map[string]string `mapstructure:"annotations"`
	Labels      map[string]string `mapstructure:"labels"`
}

// metaObject is used only for pulling out object metadata
type metaObject struct {
	Meta metav1.ObjectMeta `json:"metadata"`
}

// MutateAdmission takes an admission request and generates an admission response based on the response from Mutate.
// It implements the graffitiMutator interface and so can be added to the webhook handler's tagmap
func (r Rule) MutateAdmission(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	mylog := log.ComponentLogger(componentName, "MutateAdmission")
	mylog = mylog.With().Str("rule", r.Name).Str("kind", req.Kind.String()).Str("name", req.Name).Str("namespace", req.Namespace).Logger()

	// make sure that name and namespace fields are populated in the metadata object
	object := make(map[string]interface{})
	if err := json.Unmarshal(req.Object.Raw, &object); err != nil {
		return admissionResponseError(fmt.Errorf("failed to unmarshal object from the admission request: %v", err))
	}
	if req.Name != "" {
		addMetadata(object, "name", req.Name)
	}
	if req.Namespace != "" {
		addMetadata(object, "namespace", req.Namespace)
	}
	updatedObject, err := json.Marshal(object)
	if err != nil {
		return admissionResponseError(fmt.Errorf("failed to marshal object: %v", err))
	}

	patch, err := r.Mutate(updatedObject)
	if err != nil {
		return admissionResponseError(fmt.Errorf("failed to mutate object: %v", err))
	}

	if patch == nil {
		return &admission.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: "rule didn't match",
			},
		}
	}

	pt := admission.PatchTypeJSONPatch
	return &admission.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: "object painted by kube-graffiti",
		},
		PatchType: &pt,
		Patch:     patch,
	}
}

// addMetadata adds/sets a metadata item, creating new metadata map if required.
func addMetadata(obj map[string]interface{}, k, v string) {
	if _, ok := obj["metadata"]; ok {
		meta := obj["metadata"].(map[string]interface{})
		meta[k] = v
	} else {
		obj["metadata"] = map[string]interface{}{k: v}
	}
}

func admissionResponseError(err error) *admission.AdmissionResponse {
	mylog := log.ComponentLogger(componentName, "admissionResponseError")
	mylog.Error().Err(err).Msg("admission response error, skipping any modification")
	return &admission.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

// Mutate takes a raw object and applies the graffiti rule against it, returning a JSON patch or an error.
// It performs the logic between selectors and the boolean-operator.
func (r Rule) Mutate(object []byte) (patch []byte, err error) {
	mylog := log.ComponentLogger(componentName, "Mutate")
	mylog = mylog.With().Str("rule", r.Name).Str("object", string(object)).Logger()
	var (
		paintIt      = false
		labelMatches = false
		fieldMatches = false
		metaObject   metaObject
	)

	if err := json.Unmarshal(object, &metaObject); err != nil {
		return nil, fmt.Errorf("failed to unmarshal generic object metadata from the admission request: %v", err)
	}

	// create the field map for use with field matchers and addition templating.
	fieldMap, err := makeFieldMapFromRawObject(object)
	if err != nil {
		return nil, err
	}

	if len(r.Matchers.LabelSelectors) == 0 && len(r.Matchers.FieldSelectors) == 0 {
		mylog.Debug().Msg("rule does not contain any label or field selectors so it matches ALL")
		paintIt = true
	} else {
		// match against all of the label selectors
		mylog.Debug().Int("count", len(r.Matchers.LabelSelectors)).Msg("matching against label selectors")
		labelMatches, err = r.matchLabelSelectors(metaObject)
		if err != nil {
			return nil, err
		}

		// test if we match any field selectors
		mylog.Debug().Int("count", len(r.Matchers.FieldSelectors)).Msg("matching against field selectors")
		fieldMatches, err = r.matchFieldSelectors(fieldMap)
		if err != nil {
			return nil, err
		}
	}

	mylog.Debug().Bool("paintIt", paintIt).Msg("boolean result of paintIt before boolean operator")

	// Combine selector booleans and decide to paint object or not
	if !paintIt {
		descisonLog := mylog.With().Int("label-selectors-length", len(r.Matchers.LabelSelectors)).Bool("labels-matched", labelMatches).Int("field-selector-length", len(r.Matchers.FieldSelectors)).Bool("fields-matched", fieldMatches).Logger()
		switch r.Matchers.BooleanOperator {
		case AND:
			paintIt = (len(r.Matchers.LabelSelectors) == 0 || labelMatches) && (len(r.Matchers.FieldSelectors) == 0 || fieldMatches)
			descisonLog.Debug().Str("boolean-operator", "AND").Bool("result", paintIt).Msg("performed label-selector AND field-selector")
		case OR:
			paintIt = (len(r.Matchers.LabelSelectors) != 0 && labelMatches) || (len(r.Matchers.FieldSelectors) != 0 && fieldMatches)
			descisonLog.Debug().Str("boolean-operator", "OR").Bool("result", paintIt).Msg("performed label-selector OR field-selector")
		case XOR:
			paintIt = labelMatches != fieldMatches
			descisonLog.Debug().Str("boolean-operator", "XOR").Bool("result", paintIt).Msg("performed label-selector XOR field-selector")
		default:
			paintIt = false
			descisonLog.Fatal().Str("boolean-operator", "UNKNOWN").Bool("result", paintIt).Msg("Boolean Operator isn't one of AND, OR, XOR")
		}
	}

	mylog.Debug().Bool("matches", paintIt).Msg("result of boolean operator match on selectors")

	if !paintIt {
		mylog.Info().Msg("rule didn't match")
		return nil, nil
	}

	mylog.Info().Msg("rule matched - painting object")
	return r.paintObject(metaObject, fieldMap, mylog)
}

// ValidateFieldSelector checks that a field selector parses correctly and is used when validating config
func ValidateFieldSelector(selector string) error {
	if _, err := fields.ParseSelector(selector); err != nil {
		return err
	}
	return nil
}

func (r Rule) matchLabelSelectors(object metaObject) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchLabelSelectors")
	// test if we matched any of the label selectors
	if len(r.Matchers.LabelSelectors) != 0 {
		sourceLabels := make(map[string]string)
		// make it so we can use name and namespace as label selectors
		sourceLabels["name"] = object.Meta.Name
		sourceLabels["namespace"] = object.Meta.Namespace
		for k, v := range object.Meta.Labels {
			sourceLabels[k] = v
		}

		for _, selector := range r.Matchers.LabelSelectors {
			mylog.Debug().Str("label-selector", selector).Msg("testing label selector")
			selectorMatch, err := matchLabelSelector(selector, sourceLabels)
			if err != nil {
				return false, err
			}
			if selectorMatch {
				mylog.Debug().Str("label-selector", selector).Msg("selector matches, will modify object")
				return true, nil
			}
		}
	}
	return false, nil
}

// matchSelector will apply a kubernetes labels.Selector to a map[string]string and return a matched bool and error.
func matchLabelSelector(selector string, target map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchLabelSelector")
	selLog := mylog.With().Str("selector", selector).Logger()

	realSelector, err := labels.Parse(selector)
	if err != nil {
		selLog.Error().Err(err).Msg("could not parse selector")
		return false, err
	}

	set := labels.Set(target)
	if !realSelector.Matches(set) {
		selLog.Debug().Msg("selector does not match")
		return false, nil
	}
	selLog.Debug().Msg("selector matches")
	return true, nil
}

// ValidateLabelSelector checks that a label selector parses correctly and is used when validating config
func ValidateLabelSelector(selector string) error {
	if _, err := labels.Parse(selector); err != nil {
		return err
	}
	return nil
}

func (r Rule) matchFieldSelectors(fm map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchFieldSelectors")
	if len(r.Matchers.FieldSelectors) != 0 {
		for _, selector := range r.Matchers.FieldSelectors {
			mylog.Debug().Str("field-selector", selector).Msg("testing field selector")
			selectorMatch, err := matchFieldSelector(selector, fm)
			if err != nil {
				return false, err
			}
			if selectorMatch {
				mylog.Debug().Str("field-selector", selector).Msg("selector matches, will modify object")
				return true, nil
			}
		}
	}
	return false, nil
}

// matchSelector will apply a kubernetes labels.Selector to a map[string]string and return a matched bool and error.
func matchFieldSelector(selector string, target map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchFieldSelector")
	selLog := mylog.With().Str("selector", selector).Logger()
	realSelector, err := fields.ParseSelector(selector)
	if err != nil {
		selLog.Error().Err(err).Msg("could not parse selector")
		return false, err
	}

	set := labels.Set(target)
	if !realSelector.Matches(set) {
		selLog.Debug().Msg("selector does not match")
		return false, nil
	}
	selLog.Debug().Msg("selector matches")
	return true, nil
}

func (r Rule) paintObject(object metaObject, fm map[string]string, logger zerolog.Logger) (patch []byte, err error) {
	mylog := logger.With().Str("func", "paintObject").Logger()

	if len(r.Additions.Labels) == 0 && len(r.Additions.Annotations) == 0 {
		return []byte{}, fmt.Errorf("graffiti rule has no additional labels or annotations")
	}
	patchString, err := r.createObjectPatch(object, fm)
	if err != nil {
		return []byte{}, fmt.Errorf("could not create json patch: %v", err)
	}
	mylog.Info().Str("patch", patchString).Msg("created json patch")
	return []byte(patchString), nil
}
