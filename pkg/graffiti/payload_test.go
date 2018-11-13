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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	admission "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidAdditionalLabel(t *testing.T) {
	var source = `---
additions:
  labels:
    add-me: "true"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err)
}

func TestValidDeletionLabel(t *testing.T) {
	var source = `---
deletions:
  labels:
  - "delete-me"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err)
}

func TestValidDeletionAnnotationLabel(t *testing.T) {
	var source = `---
deletions:
  annotations:
  - "delete-me"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err)
}

func TestInvalidAdditionalLabelKey(t *testing.T) {
	var source = `---
additions:
  labels:
    "dave.com/multiple/slashes": "painted this object"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid additions: invalid label key \"dave.com/multiple/slashes\": a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}

func TestInvalidDeletionLabelKey(t *testing.T) {
	var source = `---
deletions:
  labels:
  - "dave.com/multiple/slashes"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid deletions: invalid key: a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}

func TestInvalidDeletionAnnotationKey(t *testing.T) {
	var source = `---
deletions:
  annotations:
  - "dave.com/multiple/slashes"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid deletions: invalid key: a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}

func TestInvalidLongAdditionalLabelKey(t *testing.T) {
	var source = `---
additions:
  labels:
    "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx": "painted this object"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid additions: invalid label key \"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\": name part must be no more than 63 characters")
}

func TestInvalidAdditionalLabelValue(t *testing.T) {
	var source = `---
additions:
  labels:
    valid-label: "label values can't contain spaces"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid additions: invalid label value \"label values can't contain spaces\": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')")
}

func TestInvalidLongAdditionalLabelValue(t *testing.T) {
	var source = `---
additions:
  labels:
    add-me: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid additions: invalid label value \"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\": must be no more than 63 characters")
}

func TestValidAdditionalAnnotation(t *testing.T) {
	var source = `---
additions:
  annotations:
    ming-industries.com/mercy: "never on my watch!"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err)
}

func TestInvalidAdditionalAnnotationKey(t *testing.T) {
	var source = `---
additions:
  annotations:
    "dave.com/multiple/slashes": "painted this object"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid additions: invalid annotations: metadata.annotations: Invalid value: \"dave.com/multiple/slashes\": a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}

func TestBlockIsAValidPayload(t *testing.T) {
	var source = `---
block: true
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err, "a payload of just block should be valid")
}

func TestValidJSONPatch(t *testing.T) {
	var source = `---
json-patch: "[ { \"op\": \"delete\", \"path\": \"/metadata/labels\" } ]"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.NoError(t, err, "a payload with a single valid-json patch should be valid")
}

func TestInvalidJSONPatch(t *testing.T) {
	var source = `---
json-patch: "[ { something that isn't valid json } ]"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "invalid json-patch: invalid character 's' looking for beginning of object key string")
}

func TestBlockPlusJSONPatchNotAllowed(t *testing.T) {
	var source = `---
block: true
json-patch: "[ { \"op\": \"delete\", \"path\": \"/metadata/labels\" } ]"
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "a rule payload can only specify additions/deletions, or a json-patch or a block, but not a combination of them")
}

func TestBlockPlusAdditionsDeletionsNotAllowed(t *testing.T) {
	var source = `---
block: true
additions:
  labels:
    added: label
`
	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "a rule payload can only specify additions/deletions, or a json-patch or a block, but not a combination of them")
}

func TestJSONPatchPlusAdditionsDeletionsNotAllowed(t *testing.T) {
	var source = `---
json-patch: "[ { \"op\": \"delete\", \"path\": \"/metadata/labels\" } ]"
additions:
  labels:
    added: label
`

	var payload Payload
	err := yaml.Unmarshal([]byte(source), &payload)
	spew.Dump(payload)
	require.NoError(t, err, "the test payload should unmarshal")
	err = payload.validate()
	assert.EqualError(t, err, "a rule payload can only specify additions/deletions, or a json-patch or a block, but not a combination of them")
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
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/labels", "value": {} } ]`)
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
	desired, _ := jsonpatch.FromString(`[ { "op": "replace", "path": "/metadata/annotations", "value": {} } ]`)
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
