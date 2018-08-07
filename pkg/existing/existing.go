package existing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"stash.hcom/run/istio-namespace-webhook/pkg/blacklist"
	"stash.hcom/run/istio-namespace-webhook/pkg/log"
)

const (
	componentName = "existing-checks"
	istioLabel    = "istio-injection"
)

func CheckExistingNamespaces(k *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "CheckExistingNamespaces")
	mylog.Debug().Msg("listing namespaces")

	client := k.CoreV1().Namespaces()
	namespaces, err := client.List(metav1.ListOptions{})
	if err != nil {
		mylog.Fatal().Err(err).Msg("failed to list namespaces")
	}
	for _, ns := range namespaces.Items {
		nslog := mylog.With().Str("namespace", ns.Name).Logger()
		if blacklist.SharedBlacklist.InList(ns.Name) {
			nslog.Info().Msg("skipping blacklisted namespace")
		} else {
			nslog.Debug().Msg("checking namespace for istio label")
			if _, ok := ns.GetLabels()[istioLabel]; ok != true {
				nslog.Debug().Msg("updating namespace with istio label")

			} else {
				nslog.Info().Msg("namespace already has istio label")
			}
		}
	}

	return nil
}
