package graffiti

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/cameront/go-jsonpatch"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	admission "k8s.io/api/admission/v1beta1"
)

const testReview = `{
	"kind":"AdmissionReview",
	"apiVersion":"admission.k8s.io/v1beta1",
	"request":{
	   "uid":"69f7d25a-963e-11e8-a77c-08002753edac",
	   "kind":{
		  "group":"",
		  "version":"v1",
		  "kind":"Namespace"
	   },
	   "resource":{
		  "group":"",
		  "version":"v1",
		  "resource":"namespaces"
	   },
	   "operation":"CREATE",
	   "userInfo":{
		  "username":"minikube-user",
		  "groups":[
			 "system:masters",
			 "system:authenticated"
		  ]
	   },
	   "object":{
		  "metadata":{
			 "name":"test-namespace",
			 "creationTimestamp":null,
			 "labels":{
				 "author": "david",
				 "group": "runtime"
			 },
			 "annotations":{
				 "level": "v.special",
				 "prometheus.io/path": "/metrics"
			 }
		  },
		  "spec":{

		  },
		  "status":{
			 "phase":"Active"
		  }
	   },
	   "oldObject":null
	}
 }`

func TestAddMetadata(t *testing.T) {
	var a map[string]interface{}

	addMetadata(a, "b", "c")
	addMetadata(a, "x", "y")
}

func TestReviewObjectDoesNotHaveMetaData(t *testing.T) {
	rule := Rule{Matchers: Matchers{LabelSelectors: []string{"author = stephen"}}}

	var missingMetaData = `{
		"kind":"AdmissionReview",
		"apiVersion":"admission.k8s.io/v1beta1",
		"request":{
		   "uid":"69f7d25a-963e-11e8-a77c-08002753edac",
		   "kind":{
			  "group":"",
			  "version":"v1",
			  "kind":"Namespace"
		   },
		   "resource":{
			  "group":"",
			  "version":"v1",
			  "resource":"namespaces"
		   },
		   "operation":"CREATE",
		   "userInfo":{
			  "username":"minikube-user",
			  "groups":[
				 "system:masters",
				 "system:authenticated"
			  ]
		   },
		   "object":{
			  "spec":{

			  },
			  "status":{
				 "phase":"Active"
			  }
		   },
		   "oldObject":null
		}
	 }`

	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(missingMetaData), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	resp := rule.MutateAdmission(review.Request)
	assert.NotNil(t, resp)
	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.Nil(t, resp.Patch, "there shouldn't be patch")
	assert.Equal(t, "rule didn't match", resp.Result.Message)
}

func TestNoSelectorsMeansMatchEverything(t *testing.T) {
	// create a Rule
	rule := Rule{
		Payload: Payload{
			Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)

	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.EqualValues(t, desired.Operations, actual.Operations)
}

func TestMatchingSelectorWithoutLablesOrAnnotationsProducesNoPatch(t *testing.T) {
	// create a Rule
	rule := Rule{Matchers: Matchers{LabelSelectors: []string{"author=david"}}}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestHandlesNoSourceObjectLabelsOrAnnotations(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"name=test-namespace"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"modified-by-graffiti": "abc123"},
			},
		},
	}

	var noLabels = `{
		"kind":"AdmissionReview",
		"apiVersion":"admission.k8s.io/v1beta1",
		"request":{
		   "uid":"69f7d25a-963e-11e8-a77c-08002753edac",
		   "kind":{
			  "group":"",
			  "version":"v1",
			  "kind":"Namespace"
		   },
		   "resource":{
			  "group":"",
			  "version":"v1",
			  "resource":"namespaces"
		   },
		   "operation":"CREATE",
		   "userInfo":{
			  "username":"minikube-user",
			  "groups":[
				 "system:masters",
				 "system:authenticated"
			  ]
		   },
		   "object":{
			  "metadata":{
				 "name":"test-namespace",
				 "creationTimestamp":null
			  },
			  "spec":{

			  },
			  "status":{
				 "phase":"Active"
			  }
		   },
		   "oldObject":null
		}
	 }`

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(noLabels), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[ { "op": "add", "path": "/metadata/labels", "value": { "modified-by-graffiti": "abc123" }} ]`, string(resp.Patch))
}

func TestAllRulesMustHaveAPayload(t *testing.T) {
	var source = `---
name: "my-rule"
matchers:
  labelselectors:
  - "name=test-pod"
`
	mylog := log.Logger
	var rule Rule
	err := yaml.Unmarshal([]byte(source), &rule)
	assert.NoError(t, err, "couldn't marshall a valid rule object")
	err = rule.Validate(mylog)
	assert.EqualError(t, err, "rule 'my-rule' failed validation: a rule payload must specify either additions/deletions, a json-patch, or a block")
}

func TestWhenAdditionsAlreadyThereProducesNoPatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"author": "david"},
			},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}
