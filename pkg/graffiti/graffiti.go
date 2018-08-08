package graffiti

import (
	//"stash.hcom/run/istio-namespace-webhook/pkg/log"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/cameront/go-jsonpatch"
	// "github.com/davecgh/go-spew/spew"
	"github.com/getlantern/deepcopy"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

const (
	componentName = "grafitti"
)

type Rule struct {
	AnnotationSelector string
	LabelSelector      string
	Annotations        map[string]string
	Labels             map[string]string
}

// genericObject is used only for pulling out object metadata
type genericObject struct {
	Meta *metav1.ObjectMeta `json:"metadata"`
}

func (r Rule) Mutate(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	mylog := log.ComponentLogger(componentName, "Mutate")
	var (
		noSelectorMatch = false
		labMatch        = false
		annoMatch       = false
		err             error
		object          genericObject
	)

	if err := json.Unmarshal(req.Object.Raw, &object); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal generic object metadata from the admission request")
		return admissionResponseError(err)
	}
	if object.Meta == nil {
		return admissionResponseError(fmt.Errorf("can not apply selectors because the review object contains no metadata"))
	}
	if object.Meta.Labels == nil {
		object.Meta.Labels = make(map[string]string)
	}
	object.Meta.Labels["name"] = object.Meta.Name
	object.Meta.Labels["namespace"] = object.Meta.Namespace

	if r.LabelSelector == "" && r.AnnotationSelector == "" {
		noSelectorMatch = true
	} else {
		// test if we matched the label selector
		if r.LabelSelector != "" {
			mylog.Debug().Str("label-selector", r.LabelSelector).Msg("rule has a label selector")
			labMatch, err = matchSelector(r.LabelSelector, object.Meta.Labels)
			if err != nil {
				return admissionResponseError(err)
			}
		}

		if r.AnnotationSelector != "" {
			mylog.Debug().Str("annotation-selector", r.LabelSelector).Msg("rule has an annotation selector")
			annoMatch, err = matchSelector(r.AnnotationSelector, object.Meta.Annotations)
			if err != nil {
				return admissionResponseError(err)
			}
		}
	}

	reviewResponse := admission.AdmissionResponse{}
	reviewResponse.Allowed = true

	if noSelectorMatch || labMatch || annoMatch {
		mylog.Debug().Msg("no selectors or matched one or more selectors modifying request object")
		if len(r.Labels) == 0 && len(r.Annotations) == 0 {
			return admissionResponseError(fmt.Errorf("rule does contain any labels or annotations to add"))
		}
		patch, err := r.createObjectPatch(object)
		if err != nil {
			return admissionResponseError(fmt.Errorf("could not create the json patch"))
		}
		mylog.Debug().Str("patch", string(patch)).Msg("created json patch")
		reviewResponse.Patch = patch
		pt := admission.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	} else {
		mylog.Info().Str("name", object.Meta.Name).Str("namespace", object.Meta.Namespace).Msg("rule did not match, no modifications made")
		reviewResponse.Result = &metav1.Status{Message: "no selectors matched object"}
	}

	return &reviewResponse
}

func admissionResponseError(err error) *admission.AdmissionResponse {
	return &admission.AdmissionResponse{
		Allowed: true,
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

// matchSelector will apply a kubernetes labels.Selector to a map[string]string and return a matched bool and error.
func matchSelector(selector string, target map[string]string) (bool, error) {
	mylog := log.ComponentLogger(componentName, "matchSelector")
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

// createJSONPatch will generate a JSON patch of the difference between the source object and one with
// added labels and/or annotations
func (r Rule) createObjectPatch(obj genericObject) ([]byte, error) {
	mylog := log.ComponentLogger(componentName, "createJSONPatch")

	// make a deep copy of the request object and append any labels or annotations from the rule.
	var modified genericObject
	if err := deepcopy.Copy(&modified, &obj); err != nil {
		mylog.Error().Err(err).Msg("failed to deep copy the request object")
		return []byte{}, err
	}

	if len(r.Labels) > 0 {
		for k, v := range r.Labels {
			mylog.Debug().Str(k, v).Msg("adding label")
			modified.Meta.Labels[k] = v
		}
	}
	if len(r.Annotations) > 0 {
		for k, v := range r.Annotations {
			mylog.Debug().Str(k, v).Msg("adding annotation")
			modified.Meta.Annotations[k] = v
		}
	}

	return genericJSONPatch(obj, modified)
}

func genericJSONPatch(src, dst interface{}) ([]byte, error) {
	mylog := log.ComponentLogger(componentName, "genericJSONPatch")

	// marshal the objects to json
	srcJSON, err := json.Marshal(src)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal source object")
		return []byte{}, err
	}
	dstJSON, err := json.Marshal(dst)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal destination object")
		return []byte{}, err
	}

	// unmarshal them back to map[string]interface{} objects
	var srcmap map[string]interface{}
	var dstmap map[string]interface{}
	if err := json.Unmarshal(srcJSON, &srcmap); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal source json again")
		return []byte{}, err
	}
	if err := json.Unmarshal(dstJSON, &dstmap); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal source json again")
		return []byte{}, err
	}

	// generate a patch and return as json
	patch, err := jsonpatch.MakePatch(srcmap, dstmap)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to make json patch")
		return []byte{}, err
	}

	json, err := patch.MarshalJSON()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to marshal patch json")
		return []byte{}, err
	}
	return json, nil
}
