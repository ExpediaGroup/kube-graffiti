package healthcheck

import (
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"stash.hcom/run/istio-namespace-webhook/pkg/log"
)

const componentName = "healthcheck"

// Abstract kubernetes client to cut down amount to mock, we only need to list namespaces.
type kubernetesClient interface {
	namespaces() kubernetesNamespaceAccessor
}

type kubernetesNamespaceAccessor interface {
	List(options metav1.ListOptions) (*corev1.NamespaceList, error)
}

type realKubernetesClient struct {
	client *kubernetes.Clientset
}

// In our kubernetes client type we know how to get namespaces without needing to know that
// namespaces are in fact in CoreV1.
func (real realKubernetesClient) namespaces() kubernetesNamespaceAccessor {
	return real.client.CoreV1().Namespaces()
}

// We store our kubernetes client at the package level so that it can be shared
// with our healthCheckHandler functions.
var healthCheckClient kubernetesClient

// healthCheckHandler pulls in a kubernetes client of our interface type kubernetesClient
// and attempts to get a list of namespaces, return unhealthy if we get an error from the
// kubernetes api server.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	mylog := log.ComponentLogger(componentName, "healthCheckHandler")
	reqLog := mylog.With().Str("url", r.URL.String()).Str("host", r.Host).Str("method", r.Method).Str("ua", r.UserAgent()).Str("remote", r.RemoteAddr).Logger()
	reqLog.Debug().Msg("health check triggered, listing namespaces via kubernetes api")
	_, err := healthCheckClient.namespaces().List(metav1.ListOptions{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"healthy": false}`)
		mylog.Error().Err(err).Int("status", http.StatusInternalServerError).Msg("returning failed")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"healthy": true}`)
	reqLog.Debug().Int("status", http.StatusOK).Msg("returning ok")
}

// AddHealthCheckHandler adds a health checker on the URL path specified in the shared webserver
// and shares the kubernetes client supplied.
func AddHealthCheckHandler(path string, client *kubernetes.Clientset) {
	mylog := log.ComponentLogger(componentName, "AddHealthCheckHandler")
	healthCheckClient = realKubernetesClient{client: client}
	http.HandleFunc(path, healthCheckHandler)
	mylog.Info().Str("path", path).Msg("health-check handler added")
}
