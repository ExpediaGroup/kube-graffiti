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

package graffiti

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/cameront/go-jsonpatch"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	admission "k8s.io/api/admission/v1beta1"
)

func TestRulesContainingInvalidLabelSelectorsFailValidation(t *testing.T) {
	var source = `---
label-selectors:
-  "i don't know what you hope this label selector will do?"
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	spew.Dump(matchers)
	err = matchers.validate(mylog)
	assert.EqualError(t, err, "matcher contains an invalid label selector 'i don't know what you hope this label selector will do?': unable to parse requirement: found 'don't', expected: '=', '!=', '==', 'in', notin'")
}

func TestASetBasedLabelSelectorsAreValid(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
label-selectors:
-  "namespace notin (default,kube-system,kube-public)"
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	err = matchers.validate(mylog)
	assert.NoErrorf(t, err, "this is a valid label selector and so should not fail our validation checks")
}

func TestASimpleFieldSelectorIsValid(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
field-selectors:
-  "metadata.name = dave"
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	err = matchers.validate(mylog)
	assert.NoErrorf(t, err, "this is a valid field selector and so should not fail our validation checks")
}

func TestRulesContainingInvalidFieldSelectorsFailValidation(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
field-selectors:
-  "namespace notin (default,kube-system,kube-public)"
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	err = matchers.validate(mylog)
	assert.Error(t, err, "this complex label-selector rule is not a valid field selector rule")
}

func TestUnmarshalBooleanOperatorOR(t *testing.T) {
	var source = `---
label-selectors:
- "name=dave"
- "dave=true"
field-selectors:
- "spec.status=bingbong-a-bing-bing-bong"
boolean-operator: OR
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	err = matchers.validate(mylog)
	assert.Equal(t, BooleanOperator(1), matchers.BooleanOperator, "the OR operator is represented internaly as integer 1")
}

func TestUnmarshalBooleanOperatorXOR(t *testing.T) {
	var source = `---
label-selectors:
- "name=dave"
- "dave=true"
field-selectors:
- "spec.status=bingbong-a-bing-bing-bong"
boolean-operator: XOR
`
	mylog := log.Logger
	var matchers Matchers
	err := yaml.Unmarshal([]byte(source), &matchers)
	require.NoError(t, err, "the test matchers should unmarshal")
	err = matchers.validate(mylog)
	assert.Equal(t, BooleanOperator(2), matchers.BooleanOperator, "the OR operator is represented internaly as integer 2")
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
