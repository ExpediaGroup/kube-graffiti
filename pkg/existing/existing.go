package existing

import (
	"k8s.io/client-go/kubernetes"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

const (
	componentName = "existing-checks"
)

func CheckExistingObjects(_ *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "CheckExistingObjects")
	mylog.Warn().Msg("Not yet implemented...")

	return nil
}
