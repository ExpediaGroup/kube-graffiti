package healthcheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Allowing us to mock out our calls to list kubernetes namespaces
type kubernetesClientMock struct {
	mock.Mock
}

func (k *kubernetesClientMock) namespaces() kubernetesNamespaceAccessor {
	args := k.Called()
	return args.Get(0).(*kubernetesNamespaceAccessorMock)
}

type kubernetesNamespaceAccessorMock struct {
	mock.Mock
}

func (r *kubernetesNamespaceAccessorMock) List(options metav1.ListOptions) (*corev1.NamespaceList, error) {
	args := r.Called(options)
	return args.Get(0).(*corev1.NamespaceList), args.Error(1)
}

func TestHealthlyCheck(t *testing.T) {
	// set up the mocks
	lister := new(kubernetesNamespaceAccessorMock)
	lister.On("List", mock.AnythingOfType("v1.ListOptions")).Return(&corev1.NamespaceList{}, nil)
	kclient := new(kubernetesClientMock)
	kclient.On("namespaces").Return(lister)

	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/healthz", nil)
	assert.Nil(t, err, "We created a valid http request")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	checker := NewHealthChecker(kclient, 80, "/healthz")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	checker.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusOK)
	kclient.AssertExpectations(t)
	lister.AssertExpectations(t)

	// Check the response body is what we expect.
	expected := `{"healthy": true}`
	assert.Equal(t, rr.Body.String(), expected)
}

func TestUnHealthlyCheck(t *testing.T) {
	// set up the mocks
	lister := new(kubernetesNamespaceAccessorMock)
	lister.On("List", mock.AnythingOfType("v1.ListOptions")).Return(&corev1.NamespaceList{}, fmt.Errorf("test error"))
	kclient := new(kubernetesClientMock)
	kclient.On("namespaces").Return(lister)

	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/healthz", nil)
	assert.Nil(t, err, "We created a valid http request")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	checker := NewHealthChecker(kclient, 80, "/healthz")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	checker.ServeHTTP(rr, req)

	assert.Equal(t, rr.Code, http.StatusInternalServerError)
	kclient.AssertExpectations(t)
	lister.AssertExpectations(t)

	// Check the response body is what we expect.
	expected := `{"healthy": false}`
	assert.Equal(t, rr.Body.String(), expected)
}
