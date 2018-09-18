package graffiti

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
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
