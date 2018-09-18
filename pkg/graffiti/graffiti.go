// Package graffiti decideds whether an object should be graffiti'd, according to a rule, and then produces and JSON patch with the desired modification.
// Mutate either kubernetes admission request objects or plain raw objects.
package graffiti

import (
	//"stash.hcom/run/istio-namespace-webhook/pkg/log"

	"bytes"
	"encoding/json"
	"fmt"

	// "github.com/davecgh/go-spew/spew"

	"github.com/rs/zerolog"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Rule contains a single graffiti rule and contains matchers for choosing which objects to change and payload containing the change.
// It does not have mapstructure tags because it is not directly marshalled from config
type Rule struct {
	Name     string
	Matchers Matchers
	Payload  Payload
}

// metaObject is used only for pulling out object metadata
type metaObject struct {
	Meta metav1.ObjectMeta `json:"metadata"`
}

// Validate - validates the matchers and payload of a graffiti rule
func (r Rule) Validate(rulelog zerolog.Logger) (err error) {
	if err = r.Matchers.validate(rulelog); err != nil {
		return fmt.Errorf("rule '%s' failed validation: %v", r.Name, err)
	}
	if err = r.Payload.validate(); err != nil {
		return fmt.Errorf("rule '%s' failed validation: %v", r.Name, err)
	}
	return nil
}

// MutateAdmission takes an admission request and generates an admission response based on the response from Mutate.
// It implements the graffitiMutator interface and so can be added to the webhook handler's tagmap
func (r Rule) MutateAdmission(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	mylog := log.ComponentLogger(componentName, "MutateAdmission")
	mylog = mylog.With().Str("rule", r.Name).Str("kind", req.Kind.String()).Str("name", req.Name).Str("namespace", req.Namespace).Logger()

	object, err := extractObject(req)
	if err != nil {
		admissionResponseError(fmt.Errorf("failed to extract object from admission request: %v", err))
	}

	patch, err := r.Mutate(object)
	if err != nil {
		return admissionResponseError(fmt.Errorf("failed to mutate object: %v", err))
	}

	return patchResult(patch, r.Name)
}

func extractObject(req *admission.AdmissionRequest) (result []byte, err error) {
	// make sure that name and namespace fields are populated in the metadata object
	object := make(map[string]interface{})
	if err = json.Unmarshal(req.Object.Raw, &object); err != nil {
		return result, err
	}
	if req.Name != "" {
		addMetadata(object, "name", req.Name)
	}
	if req.Namespace != "" {
		addMetadata(object, "namespace", req.Namespace)
	}
	return json.Marshal(object)
}

func patchResult(patch []byte, name string) *admission.AdmissionResponse {
	if patch == nil {
		return &admission.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: "rule didn't match",
			},
		}
	}

	// handle a rule which blocks instead of patching...
	if bytes.Equal(patch, []byte("BLOCK")) {
		return &admission.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Reason:  metav1.StatusReasonForbidden,
				Message: fmt.Sprintf("blocked by kube-graffiti rule: %s", name),
			},
			Patch: nil,
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
	if obj == nil {
		return
	}
	if _, ok := obj["metadata"]; ok {
		if meta, ok := obj["metadata"].(map[string]interface{}); ok {
			meta[k] = v
		}
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
	mylog = mylog.With().Str("rule", r.Name).Logger()
	var metaObject metaObject

	if err := json.Unmarshal(object, &metaObject); err != nil {
		return nil, fmt.Errorf("failed to unmarshal generic object metadata from the admission request: %v", err)
	}

	// create the field map for use with field matchers and addition templating.
	fieldMap, err := makeFieldMapFromRawObject(object)
	if err != nil {
		return nil, err
	}

	match, err := r.Matchers.matches(metaObject, fieldMap, mylog)
	if err != nil {
		return nil, err
	}
	if match {
		mylog.Info().Msg("rule matched - painting object")
		return r.Payload.paintObject(metaObject, fieldMap, mylog)
	}

	mylog.Info().Msg("rule didn't match - not painting object")
	return nil, nil
}
