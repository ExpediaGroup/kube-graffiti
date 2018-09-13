package config

import (
	"bytes"
	"testing"

	"stash.hcom/run/kube-graffiti/pkg/graffiti"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spf13/viper"
	// "github.com/davecgh/go-spew/spew"
)

var testConfig = `log-level: debug
check-existing: true
health-checker:
  port: 9999
  path: /am-i-healthy
server:
  port: 1010
  namespace: test-namespace
  service: graffiti-service
  ca-cert-path: /my-ca-path
  cert-path: /my-cert-path
  key-path: /my-key-path
rules:
- registration:
    name: label-namespaces-called-dave
    namespace-selector:
    failure-policy: Fail
    targets:
    - api-groups:
      - ""
      api-versions:
      - v1
      resources:
      - namespaces
  matchers:
    label-selectors:
    - "name = dave"
    - "dave = true"
  payload:
    additions:
      labels:
        result: "this_is_indeed_daveish"
- registration:
    name: annotate-everything-except-kube-system
    failure-policy: Ignore
    targets:
    - api-groups:
      - "*"
      api-versions:
      - "*"
      resources:
      - "*"
  matchers:
    field-selectors:
    -  "metadata.namespace != kube-system"
  payload:
    additions:
      annotations:
        graffiti: "woz_'ere_2018"
`

func TestParseConfig(t *testing.T) {
	// read viper config from our test config file
	setDefaults()
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(testConfig)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// assert that we can correctly load values via viper.GetXXX methods
	assert.Equal(t, "debug", viper.GetString("log-level"), "the log-level should have been set away from default by our config")
	assert.Equal(t, 9999, viper.GetInt("health-checker.port"))
	assert.Equal(t, "/am-i-healthy", viper.GetString("health-checker.path"))
	assert.Equal(t, "test-namespace", viper.GetString("server.namespace"), "should have set our namespace")
	assert.True(t, viper.GetBool("check-existing"))

	// assert that we can marshal the config into a Configuration struct
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err)

	assert.Equal(t, 2, len(config.Rules), "there should be two graffiti rules loaded")
	assert.Equal(t, "annotate-everything-except-kube-system", config.Rules[1].Registration.Name)
	assert.Equal(t, "Fail", config.Rules[0].Registration.FailurePolicy)
	defaultOperator, _ := graffiti.BooleanOperatorString("AND")
	assert.IsType(t, defaultOperator, config.Rules[0].Matchers.BooleanOperator)
	assert.Equal(t, defaultOperator, config.Rules[0].Matchers.BooleanOperator, "the boolean-operator needs to be the correct type and default to its AND/0 value")

	// check that config validates ok
	err = config.ValidateConfig()
	assert.NoError(t, err)

	// set an invalid log-level and validate should fail
	config.LogLevel = "craaazzy"
	err = config.ValidateConfig()
	assert.Error(t, err)
	assert.EqualError(t, err, "craaazzy is not a valid log-level")
}

func TestUnmarshalBooleanOperatorOR(t *testing.T) {
	var source = `---
rules:
- registration:
    name: boolean-or-between-label-and-field-selectors
  matchers:
    label-selectors:
    - "name=dave"
    - "dave=true"
    field-selectors:
    - "spec.status=bingbong-a-bing-bing-bong"
    boolean-operator: OR
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// assert that we can marshal the config into a Configuration struct
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err)

	assert.Equal(t, graffiti.BooleanOperator(1), config.Rules[0].Matchers.BooleanOperator, "the OR operator is represented internaly as integer 1")
}

func TestUnmarshalBooleanOperatorXOR(t *testing.T) {
	var source = `---
rules:
- registration:
    name: boolean-or-between-label-and-field-selectors
  matchers:
    label-selectors:
    - "name=dave"
    - "dave=true"
    field-selectors:
    - "spec.status=bingbong-a-bing-bing-bong"
    boolean-operator: XOR
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// assert that we can marshal the config into a Configuration struct
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err)

	assert.Equal(t, graffiti.BooleanOperator(2), config.Rules[0].Matchers.BooleanOperator, "the OR operator is represented internaly as integer 2")
}

func TestUnknownConfigurationFieldsThrowAnError(t *testing.T) {
	var source = `elvis: "thank-you very much"`
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading into viper - it's perfectly valid to load anything")

	// assert that we can marshal the config into a Configuration struct
	_, err = unmarshalFromViperStrict()
	require.Error(t, err, "when unmarshaling into a strict Configuration it is, however, not ok to have unknown fields in viper")
}

func TestNoRulesThrowsAnError(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// assert that we can marshal the config into a Configuration struct
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err)

	err = config.ValidateConfig()
	assert.Errorf(t, err, "no rules found")
}

func TestServerNamespaceAndServiceAreRequired(t *testing.T) {
	var source = `---
rules:
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "name=test-pod"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err)
	err = config.ValidateConfig()
	assert.Errorf(t, err, "missing required parameter")
}

func TestAllRulesMustHaveAdditions(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "name=test-pod"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.Errorf(t, err, "rule my-rule is invalid - it does not contain any additional labels or annotations", "rules without additions should cause ValidateConfig to fail")
}

func TestMultipleRulesCanNotHaveTheSameName(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "name=test-pod"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "name=another-test-pod"
  payload:
    additions:
      labels:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.Errorf(t, err, "rule my-rule is invalid - found duplicate rules with the same name, they must be unique", "two rules with the same name should cause a validation error")
}

func TestRulesContainingInvalidLabelSelectorsFailValidation(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "i don't know what you hope this label selector will do?"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.Errorf(t, err, "rule my-rule is invalid - contains invalid label selector 'i don't know what you hope this label selector will do?': unable to parse requirement: found 'don't', expected: '=', '!=', '==', 'in', notin'", "invalid rules generate a validation error")
}

func TestASetBasedLabelSelectorsAreValid(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    label-selectors:
    -  "namespace notin (default,kube-system,kube-public)"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.NoErrorf(t, err, "this is a valid label selector and so should not fail our validation checks")
}

func TestASimpleFieldSelectorIsValid(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    field-selectors:
    -  "metadata.name = dave"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.NoErrorf(t, err, "this is a valid field selector and so should not fail our validation checks")
}

func TestRulesContainingInvalidFieldSelectorsFailValidation(t *testing.T) {
	// this would be a valid label selector but field selectors are more limited
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  matchers:
    field-selectors:
    -  "namespace notin (default,kube-system,kube-public)"
  payload:
    additions:
      annotations:
        graffiti: "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.Error(t, err, "this complex label-selector rule is not a valid field selector rule")
}

func TestValidAdditionalLabel(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      labels:
        add-me: "true"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.NoError(t, err)
}

func TestInvalidAdditionalLabelKey(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      labels:
        "dave.com/multiple/slashes": "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule contains invalid label key: a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}

func TestInvalidLongAdditionalLabelKey(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      labels:
        "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx": "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule contains invalid label key: name part must be no more than 63 characters")
}

func TestInvalidAdditionalLabelValue(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      labels:
        valid-label: "label values can't contain spaces"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule contains invalid label value: a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')")
}

func TestInvalidLongAdditionalLabelValue(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      labels:
        add-me: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule contains invalid label value: must be no more than 63 characters")
}

func TestValidAdditionalAnnotation(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      annotations:
        ming-industries.com/mercy: "never on my watch!"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.NoError(t, err)
}

func TestInvalidAdditionalAnnotationKey(t *testing.T) {
	var source = `---
server:
  namespace: test
  service: test
rules:
- registration:
    name: my-rule
  payload:
    additions:
      annotations:
        "dave.com/multiple/slashes": "painted this object"
`
	// read viper config from our test config file
	setDefaults()
	viper.Set("log-level", "debug")
	viper.SetConfigType("yaml")
	err := viper.ReadConfig(bytes.NewBuffer([]byte(source)))
	require.NoError(t, err, "there shouldn't be a failure loading the configuration")

	// check that config validates ok
	config, err := unmarshalFromViperStrict()
	require.NoError(t, err, "errors are caught during validation not unmarshalling")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule contains invalid annotations: metadata.annotations: Invalid value: \"dave.com/multiple/slashes\": a qualified name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]') with an optional DNS subdomain prefix and '/' (e.g. 'example.com/MyName')")
}
