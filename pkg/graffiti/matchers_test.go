package graffiti

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
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
