/*
Copyright (C) 2018 Expedia Group.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package healthcheck

import (
	"fmt"
	"io"
	"net/http"

	"github.com/HotelsDotCom/kube-graffiti/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const componentName = "healthcheck"

// HealthChecker is a http server that responds to http requests on http://0.0.0.0:port/path and returns 200 if it can read kubernetes api (list namespaces)
type HealthChecker struct {
	Port   int    `mapstructure:"port"`
	Path   string `mapstructure:"path"`
	client kubernetesClient
	server *http.Server
}

// Abstract kubernetes client to cut down amount to mock, we only need to list namespaces.
type kubernetesClient interface {
	namespaces() kubernetesNamespaceAccessor
}

type kubernetesNamespaceAccessor interface {
	List(options metav1.ListOptions) (*corev1.NamespaceList, error)
}

// CutDownKubernetesClient wraps a real kubernetes client and implements the kubernetesNamespaceAccessor interface
type cutDownKubernetesClient struct {
	client *kubernetes.Clientset
}

// In our kubernetes client type we know how to get namespaces without needing to know that
// namespaces are in fact in CoreV1.
func (real cutDownKubernetesClient) namespaces() kubernetesNamespaceAccessor {
	return real.client.CoreV1().Namespaces()
}

// NewCutDownNamespaceClient converts a full blown kubernetes clientset into a cut down one that implements the kubernetesNamespaceAccessor interface
func NewCutDownNamespaceClient(k *kubernetes.Clientset) cutDownKubernetesClient {
	return cutDownKubernetesClient{client: k}
}

func NewHealthChecker(k kubernetesClient, port int, path string) HealthChecker {
	mylog := log.ComponentLogger(componentName, "NewHealthChecker")
	mylog.Debug().Int("port", port).Msg("creating a new health-checker http server")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return HealthChecker{
		Port:   port,
		Path:   path,
		client: k,
		server: server,
	}
}

// StartHealthChecker starts the health-checker http server in a go-routine.
func (h HealthChecker) StartHealthChecker() {
	mylog := log.ComponentLogger(componentName, "StartHealthChecker")
	mylog.Info().Msg("starting the health-checker http server...")

	// add ourselves as the handler for http requests
	// rather than using HandleFunc we use Handle so that the Handler can use the health-checker
	// object as context and therefore have access to its embedded kubernetesClient.
	mux := h.server.Handler.(*http.ServeMux)
	mux.Handle(h.Path, h)

	// start the health-checker handler http server
	var err error
	go func() {
		if err = h.server.ListenAndServe(); err != nil {
			mylog.Fatal().Err(err).Msg("failed to start the webhook server")
		}
	}()

	return
}

// ServeHttp handles a mutating webhook review request
func (h HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mylog := log.ComponentLogger(componentName, "healthCheckHandler")
	reqLog := mylog.With().Str("url", r.URL.String()).Str("host", r.Host).Str("method", r.Method).Str("ua", r.UserAgent()).Str("remote", r.RemoteAddr).Logger()
	reqLog.Debug().Msg("health check triggered, listing namespaces via kubernetes api")
	_, err := h.client.namespaces().List(metav1.ListOptions{})
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
