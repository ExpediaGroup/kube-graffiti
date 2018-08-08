package graffiti

import (
	"encoding/json"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReviewObjectDoesNotHaveMetaData(t *testing.T) {
	rule := Rule{LabelSelector: "author = stephen"}

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

	resp := rule.Mutate(review.Request)
	assert.NotNil(t, resp)
	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.Nil(t, resp.Patch)
	assert.Equal(t, "can not apply selectors because the review object contains no metadata", resp.Result.Message)
}

func TestRuleWithNoSelectorsIsMatched(t *testing.T) {
	// create a Rule
	rule := Rule{
		Labels: map[string]string{"modified-by-graffiti": "abc123"},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	spew.Dump(resp)
	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"}]`, string(resp.Patch))
}

func TestWithoutMatchingLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{LabelSelector: "author = stephen"}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.Nil(t, resp.Patch)
}

func TestMatchingLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		LabelSelector: "author = david",
		Labels:        map[string]string{"modified-by-graffiti": "abc123"},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"}]`, string(resp.Patch))
}

func TestInvalidLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		LabelSelector: "this is not a valid selector",
		Labels:        map[string]string{"modified-by-graffiti": "abc123"},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch, "nothing is patched")
}

func TestMatchingSelectorWithoutLablesOrAnnotationsProducesNoPatch(t *testing.T) {
	// create a Rule
	rule := Rule{LabelSelector: "author=david"}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestLabelSelectorMatchesName(t *testing.T) {
	// create a Rule
	rule := Rule{
		LabelSelector: "name=test-namespace",
		Labels:        map[string]string{"modified-by-graffiti": "abc123"},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"}]`, string(resp.Patch))
}

func TestHandlesNoSourceObjectLabels(t *testing.T) {
	// create a Rule
	rule := Rule{
		LabelSelector: "name=test-namespace",
		Labels:        map[string]string{"modified-by-graffiti": "abc123"},
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
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"}]`, string(resp.Patch))
}
