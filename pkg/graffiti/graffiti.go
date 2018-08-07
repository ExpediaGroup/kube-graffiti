package graffiti

import (
	//"stash.hcom/run/istio-namespace-webhook/pkg/log"
	"encoding/json"
	"fmt"

	admission "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"stash.hcom/run/kube-graffiti/pkg/blacklist"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

const (
	componentName = "grafitti"
)

type Rule struct {
	LabelSelector metav1.LabelSelector
	Annotations   []map[string]string
	Labels        []map[string]string
}

func (r Rule) Mutate(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	mylog := log.ComponentLogger(componentName, "mutate")
	var ns corev1.Namespace

	// ensure that only namespaces are passed to the webhook
	nsResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	if req.Resource != nsResource {
		mylog.Error().Str("resource-type:", req.Resource.String()).Msg("expected a namespace")

		return admissionResponseError(fmt.Errorf("expecting a namespace but got a %s", req.Resource))
	}

	if err := json.Unmarshal(req.Object.Raw, &ns); err != nil {
		mylog.Error().Err(err).Msg("failed to unmarshal namespace from admission request object")
		return admissionResponseError(err)
	}

	// patch the namespace by adding the label
	reviewResponse := admission.AdmissionResponse{}
	reviewResponse.Allowed = true
	if !blacklist.SharedBlacklist.InList(ns.Name) {
		mylog.Info().Str("namespace", ns.Name).Msg("mutating namespace with istio label")
		reviewResponse.Patch = []byte("")
		pt := admission.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	} else {
		mylog.Info().Str("namespace", ns.Name).Msg("leaving namespace unchanged - without istio label")
	}

	return &reviewResponse
}

func admissionResponseError(err error) *admission.AdmissionResponse {
	return &admission.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
