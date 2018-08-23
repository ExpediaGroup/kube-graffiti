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
  matcher:
    label-selectors:
    - "name = dave"
    - "dave = true"
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
  matcher:
    field-selectors:
    -  "metadata.namespace != kube-system"
  additions:
    annotations:
      graffiti: "woz_'ere_2018"
`

func TestParseConfig(t *testing.T) {
	// read viper config from our test config file
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
	var config Configuration
	err = viper.Unmarshal(&config)
	require.NoError(t, err)

	assert.Equal(t, 2, len(config.Rules), "there should be two graffiti rules loaded")
	assert.Equal(t, "annotate-everything-except-kube-system", config.Rules[1].Registration.Name)
	assert.Equal(t, "Fail", config.Rules[0].Registration.FailurePolicy)
	defaultOperator, _ := graffiti.BooleanOperatorString("AND")
	assert.IsType(t, defaultOperator, config.Rules[0].Matcher.BooleanOperator)
	assert.Equal(t, defaultOperator, config.Rules[0].Matcher.BooleanOperator, "the boolean-operator needs to be the correct type and default to its AND/0 value")

	// check that config validates ok
	err = config.ValidateConfig()
	assert.NoError(t, err)

	// set an invalid log-level and validate should fail
	config.LogLevel = "craaazzy"
	err = config.ValidateConfig()
	assert.Error(t, err)
	assert.EqualError(t, err, "craaazzy is not a valid log-level")
}
