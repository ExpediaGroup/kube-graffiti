package graffiti

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/cameront/go-jsonpatch"
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

func TestReviewObjectDoesNotHaveMetaData(t *testing.T) {
	rule := Rule{Matcher: Matcher{LabelSelectors: []string{"author = stephen"}}}

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
	assert.Nil(t, resp.Patch, "there shouldn't be patch")
	assert.Equal(t, "rules did not match, object not updated", resp.Result.Message)
}

func TestNoSelectorsMeansMatchEverything(t *testing.T) {
	// create a Rule
	rule := Rule{
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	require.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)

	assert.Equal(t, true, resp.Allowed, "failed rules should not block the source api request")
	assert.NotNil(t, resp.Patch)
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"}]`, string(resp.Patch), "an absence of selectors is taken to mean always match")
}

func TestWithoutMatchingLabelSelector(t *testing.T) {
	// create a Rule
	rule := Rule{Matcher: Matcher{
		LabelSelectors: []string{"author = stephen"},
	}}

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
	rule := Rule{Matcher: Matcher{
		LabelSelectors: []string{"author = david"},
	},
		Additions: Additions{
			Labels: map[string]string{"modified-by-graffiti": "abc123"},
		},
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
		Matcher:   Matcher{LabelSelectors: []string{"this is not a valid selector"}},
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
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
	rule := Rule{Matcher: Matcher{LabelSelectors: []string{"author=david"}}}

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
		Matcher:   Matcher{LabelSelectors: []string{"name=test-namespace"}},
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
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

func TestMultipleLabelSelectorsAreORed(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher:   Matcher{LabelSelectors: []string{"name=not-a-name-that-matches", "author = david"}},
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
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
		Matcher:   Matcher{LabelSelectors: []string{"name=test-namespace"}},
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
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
	assert.Equal(t, `[{"op":"add","path":"/metadata/labels","value":{"modified-by-graffiti":"abc123"}}]`, string(resp.Patch))
}

func TestSimpleFieldSelectorMiss(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			FieldSelectors: []string{"author = david"},
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestMatchingSimpleFieldSelectorHit(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"metadata.annotations.level=v.special"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestMatchingNegativeSimpleFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"metadata.annotations.level!=elvis"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	spew.Dump(desired, actual)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestSuccessfullCombinedFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"status.phase=Active,metadata.annotations.level=v.special"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestCombinedFieldSelectorShouldANDTheCommaSeparatedSelectors(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"status.phase=Active,metadata.annotations.level=not-very-special"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestInvalidFieldSelector(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher:   Matcher{FieldSelectors: []string{"this is not a valid selector"}},
		Additions: Additions{Labels: map[string]string{"modified-by-graffiti": "abc123"}},
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

func TestORMultipleFieldSelectors(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"metadata.name=not-matching", "status.phase=Active"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestMatchingComplexFieldSelectorHit(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{FieldSelectors: []string{"metadata.annotations.prometheus.io/path=/metrics"}},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)

	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsANDTogetherByDefault(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors: []string{"a-label=which-will-not-match"},
			FieldSelectors: []string{"metadata.annotations.prometheus.io/path=/metrics"},
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestLabelAndFieldSelectorsANDSpecified(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: AND,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

// If a selector is not specified we want to match on the single selector therefore a match ANDed with an empty selector should always be true!
func TestAnEmptySelectorAlwaysMatchesWithAND(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"name=test-namespace"},
			FieldSelectors:  []string{},
			BooleanOperator: AND,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsORSelected(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: OR,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

// If only one selector exists then the match needs to be based on that selector alone when results are being OR'd, ie. an empty selector needs always be false.
func TestAnEmptySelectorNeverMatchesWithOR(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"name=a-non-matching-label-selector"},
			FieldSelectors:  []string{},
			BooleanOperator: OR,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestLabelAndFieldSelectorsXORSelectedWithSingleMatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"a-label=which-will-not-match"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: XOR,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.NotNil(t, resp.Patch)
	// we have to test the patch objects because they have multiple values and can be ordered either way round preventing a simple string match.
	desired, _ := jsonpatch.FromString(`[{"op":"add","path":"/metadata/labels/modified-by-graffiti","value":"abc123"},{"op":"add","path":"/metadata/annotations/flash","value":"saviour of the universe"}]`)
	actual, err := jsonpatch.FromString(string(resp.Patch))
	assert.NoError(t, err)
	assert.ElementsMatch(t, desired.Operations, actual.Operations)
}

func TestLabelAndFieldSelectorsXORWithBothMatchedIsFalse(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"name=test-namespace"},
			FieldSelectors:  []string{"metadata.annotations.prometheus.io/path=/metrics"},
			BooleanOperator: XOR,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}

func TestLabelAndFieldSelectorsXORanEmptySelectorIsNotAMatch(t *testing.T) {
	// create a Rule
	rule := Rule{
		Matcher: Matcher{
			LabelSelectors:  []string{"name=test-xxx"},
			FieldSelectors:  []string{},
			BooleanOperator: XOR,
		},
		Additions: Additions{
			Labels:      map[string]string{"modified-by-graffiti": "abc123"},
			Annotations: map[string]string{"flash": "saviour of the universe"},
		},
	}

	// create a review request
	var review = admission.AdmissionReview{}
	err := json.Unmarshal([]byte(testReview), &review)
	assert.NoError(t, err, "couldn't marshall a valid admission review object from test json")

	// call Mutate
	resp := rule.Mutate(review.Request)
	assert.Equal(t, true, resp.Allowed, "the request should be successful")
	assert.Nil(t, resp.Patch)
}
