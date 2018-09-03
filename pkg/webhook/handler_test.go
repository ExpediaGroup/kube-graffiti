package webhook

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admission "k8s.io/api/admission/v1beta1"
)

type mockMutator struct {
	mock.Mock
}

func (m *mockMutator) Mutate(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	args := m.Called(req)
	return args.Get(0).(*admission.AdmissionResponse)
}

func TestMethodNotPost(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err, "We created a valid http request")
	rr := httptest.NewRecorder()

	handler := newGraffitiHandler()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "invalid http method", string(respBody))
}

func TestWithNonJsonRequest(t *testing.T) {
	reqBody := strings.NewReader("This is not json")
	req, err := http.NewRequest("POST", "/", reqBody)
	assert.NoError(t, err, "We created a valid http request")
	rr := httptest.NewRecorder()
	handler := newGraffitiHandler()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "invalid request - payload is not json", string(respBody))
}

func TestRequestIsNotAnAdmissionReviewObject(t *testing.T) {
	reqBody := strings.NewReader(`{"message": "this is not a valid admission review object"}`)
	req, err := http.NewRequest("POST", "/", reqBody)
	req.Header.Set("Content-Type", "application/json")
	assert.NoError(t, err, "We created a valid http request")
	rr := httptest.NewRecorder()
	handler := newGraffitiHandler()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "The request does not contain a valid AdmissionReview object", string(respBody))
}

func TestCallsMutateWhenPathIsHandled(t *testing.T) {
	// Set up a GrafittiMutator mock
	fake := new(mockMutator)
	// no error when returning tiller role means that there is one.
	fake.On("Mutate", mock.AnythingOfType("*v1beta1.AdmissionRequest")).Return(&admission.AdmissionResponse{})

	rr := httptest.NewRecorder()
	handler := newGraffitiHandler()
	handler.addRule("/graffiti/test-rule", fake)

	reqBody := strings.NewReader("{\"kind\":\"AdmissionReview\",\"apiVersion\":\"admission.k8s.io/v1beta1\",\"request\":{\"uid\":\"69f7d25a-963e-11e8-a77c-08002753edac\",\"kind\":{\"group\":\"\",\"version\":\"v1\",\"kind\":\"Namespace\"},\"resource\":{\"group\":\"\",\"version\":\"v1\",\"resource\":\"namespaces\"},\"operation\":\"CREATE\",\"userInfo\":{\"username\":\"minikube-user\",\"groups\":[\"system:masters\",\"system:authenticated\"]},\"object\":{\"metadata\":{\"name\":\"test-namespace\",\"creationTimestamp\":null},\"spec\":{},\"status\":{\"phase\":\"Active\"}},\"oldObject\":null}}\n")
	req, err := http.NewRequest("POST", "/graffiti/test-rule", reqBody)
	req.Header.Set("Content-Type", "application/json")
	assert.NoError(t, err, "We created a valid http request")
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "{\"response\":{\"uid\":\"69f7d25a-963e-11e8-a77c-08002753edac\",\"allowed\":false}}", string(respBody))
}

func TestHandlerAllowsRequestWithMissingHandler(t *testing.T) {
	rr := httptest.NewRecorder()
	handler := newGraffitiHandler()

	reqBody := strings.NewReader("{\"kind\":\"AdmissionReview\",\"apiVersion\":\"admission.k8s.io/v1beta1\",\"request\":{\"uid\":\"69f7d25a-963e-11e8-a77c-08002753edac\",\"kind\":{\"group\":\"\",\"version\":\"v1\",\"kind\":\"Namespace\"},\"resource\":{\"group\":\"\",\"version\":\"v1\",\"resource\":\"namespaces\"},\"operation\":\"CREATE\",\"userInfo\":{\"username\":\"minikube-user\",\"groups\":[\"system:masters\",\"system:authenticated\"]},\"object\":{\"metadata\":{\"name\":\"test-namespace\",\"creationTimestamp\":null},\"spec\":{},\"status\":{\"phase\":\"Active\"}},\"oldObject\":null}}\n")
	req, err := http.NewRequest("POST", "/graffiti/missing-rule", reqBody)
	req.Header.Set("Content-Type", "application/json")
	assert.NoError(t, err, "We created a valid http request")
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	respBody, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "{\"response\":{\"uid\":\"69f7d25a-963e-11e8-a77c-08002753edac\",\"allowed\":true}}", string(respBody))
}
