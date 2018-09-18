package graffiti

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/cameront/go-jsonpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestWithoutMatchingLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{Matchers: Matchers{
		LabelSelectors: []string{"author = stephen"},
	}}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.Nil(t, resp.Patch)
}

func TestMatchingLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{Matchers: Matchers{
		LabelSelectors: []string{"author = david"},
	},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"modified-by-graffiti": "abc123"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.EqualValues(t, desired.Operations, actual.Operations)
}

func TestInvalidLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"this is not a valid selector"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"modified-by-graffiti": "abc123"},
			},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch, "nothing is patched")
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

func TestLabelSelectorMatchesName(t *testing.T) {
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

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "modified-by-graffiti": "abc123", "group": "runtime", "author": "david" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.EqualValues(t, desired.Operations, actual.Operations)
}

func TestMultipleLabelSelectorsAreORed(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"name=not-a-name-that-matches", "author = david"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"modified-by-graffiti": "abc123"},
			},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.EqualValues(t, desired.Operations, actual.Operations)
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

func TestSimpleFieldSelectorMiss(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestMatchingSimpleFieldSelectorHit(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"metadata.annotations.level=v.special"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "group": "runtime", "modified-by-graffiti": "abc123", "author": "david" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "prometheus.io/path": "/metrics", "level": "v.special", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.EqualValues(t, desired.Operations, actual.Operations)
}

func TestMatchingNegativeSimpleFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"metadata.annotations.level!=elvis"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "group": "runtime", "author": "david", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "level": "v.special", "prometheus.io/path": "/metrics", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestSuccessfullCombinedFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"status.phase=Active,metadata.annotations.level=v.special"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "level": "v.special", "flash": "saviour of the universe", "prometheus.io/path": "/metrics" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestCombinedFieldSelectorShouldANDTheCommaSeparatedSelectors(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"status.phase=Active,metadata.annotations.level=not-very-special"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestInvalidFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"this is not a valid selector"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"modified-by-graffiti": "abc123"},
			},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch, "nothing is patched")
}

func TestORMultipleFieldSelectors(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"metadata.name=not-matching", "status.phase=Active"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "level": "v.special", "prometheus.io/path": "/metrics", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestMatchingComplexFieldSelectorHit(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			FieldSelectors: []string{"metadata.annotations.prometheus.io/path=/metrics"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "group": "runtime", "modified-by-graffiti": "abc123", "author": "david" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "level": "v.special", "prometheus.io/path": "/metrics", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsANDTogetherByDefault(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"a-label=which-will-not-match"},
			FieldSelectors: []string{"metadata.annotations.prometheus.io/path=/metrics"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestLabelAndFieldSelectorsANDSpecified(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: AND,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

// If a selector is not specified we want to match on the single selector therefore a match ANDed with an empty selector should always be true!
func TestAnEmptySelectorAlwaysMatchesWithAND(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"name=test-namespace"},
			FieldSelectors:  []string{},
			BooleanOperator: AND,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "prometheus.io/path": "/metrics", "level": "v.special", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsORSelected(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: OR,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "prometheus.io/path": "/metrics", "level": "v.special", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

// If only one selector exists then the match needs to be based on that selector alone when results are being OR'd, ie. an empty selector needs always be false.
func TestAnEmptySelectorNeverMatchesWithOR(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"name=a-non-matching-label-selector"},
			FieldSelectors:  []string{},
			BooleanOperator: OR,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestLabelAndFieldSelectorsXORSelectedWithSingleMatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: XOR,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "author": "david", "group": "runtime", "modified-by-graffiti": "abc123" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "prometheus.io/path": "/metrics", "level": "v.special", "flash": "saviour of the universe" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsXORWithBothMatchedIsFalse(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"name=test-namespace"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: XOR,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestLabelAndFieldSelectorsXORanEmptySelectorIsNotAMatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors:  []string{"name=test-xxx"},
			FieldSelectors:  []string{},
			BooleanOperator: XOR,
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"modified-by-graffiti": "abc123"},
				Annotations: map[string]string{"flash": "saviour of the universe"},
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

func TestDeleteALabel(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Deletions: Deletions{
				Labels: []string{"author"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "group": "runtime" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations, "the author=david label should have been removed")
}

func TestDeleteAllLabels(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Deletions: Deletions{
				Labels: []string{"author", "group"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "delete", "path": "/metadata/labels" } ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations, "the whole /metadata/labels path should be removed")
}

func TestAddingAndDeletingLabelsCancelOut(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels: map[string]string{"added": "label"},
			},
			Deletions: Deletions{
				Labels: []string{"added"},
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
	assert.Nil(t, resp.Patch, "adding and removing a label produces no patch, adds are processed before deletes")
}

func TestDeleteAnAnnotation(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Deletions: Deletions{
				Annotations: []string{"level"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/annotations", "value": { "prometheus.io/path": "/metrics" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations, "the author=david label should have been removed")
}

func TestDeleteAllAnnotations(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Deletions: Deletions{
				Annotations: []string{"level", "prometheus.io/path"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "delete", "path": "/metadata/annotations" } ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations, "the whole /metadata/annotations path should be removed")
}

func TestMultiAddAndDelete(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Additions: Additions{
				Labels:      map[string]string{"new-label": "attached"},
				Annotations: map[string]string{"new-annotation": "made"},
			},
			Deletions: Deletions{
				Labels:      []string{"author"},
				Annotations: []string{"level"},
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
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": { "group": "runtime", "new-label": "attached" }}, { "op": "replace", "path": "/metadata/annotations", "value": { "new-annotation": "made", "prometheus.io/path": "/metrics" }} ]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations, "we should see adds and deletes of both labels and annotations")
}

func TestUserProvidedPatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			JSONPatch: "[ This is a user supplied patch ]",
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, []byte(rule.Payload.JSONPatch), resp.Patch, "the patch should be the user supplied one")
}

func TestRuleBlocksObject(t *testing.T) {
	// create a Rule
	rule := Rule{
		Name: "I-dont-like-david",
		Matchers: Matchers{
			LabelSelectors: []string{"author = david"},
		},
		Payload: Payload{
			Block: true,
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.MutateAdmission(review.Request)
	assert.Equal(t, false, resp.Allowed, "the request should not be allowed to proceed")
	assert.Nil(t, resp.Patch, "the patch should be empty")
	assert.Equal(t, metav1.StatusReasonForbidden, resp.Result.Reason, "the graffiti rule should forbid the create/update of the object")
	assert.Equal(t, "blocked by kube-graffiti rule: I-dont-like-david", resp.Result.Message, "we should be able to see why the request has been blocked and by which rule")
}
