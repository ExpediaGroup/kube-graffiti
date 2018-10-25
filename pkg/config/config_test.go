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

package config

import (
	"testing"

	"github.com/HotelsDotCom/kube-graffiti/pkg/graffiti"
	yaml "gopkg.in/yaml.v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testConfig = `---
log-level: debug
check-existing: true
health-checker:
  port: 9999
  path: /am-i-healthy
server:
  port: 1010
  namespace: test-namespace
  service: graffiti-service
  company-domain: acme.com
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
	var config Configuration
	err := yaml.Unmarshal([]byte(testConfig), &config)
	require.NoError(t, err, "the test configuration should unmarshal")

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

func TestNoRulesThrowsAnError(t *testing.T) {
	var source = `---
log-level: debug
check-existing: true
health-checker:
  port: 9999
  path: /am-i-healthy
server:
  port: 1010
  namespace: test-namespace
  service: graffiti-service
  company-domain: acme.com
  ca-cert-path: /my-ca-path
  cert-path: /my-cert-path
  key-path: /my-key-path
`
	var config Configuration
	err := yaml.Unmarshal([]byte(source), &config)
	require.NoError(t, err, "the test configuration should unmarshal")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "no rules found")
}

func TestServerNamespaceAndServiceAreRequired(t *testing.T) {
	var source = `---
log-level: debug
check-existing: true
health-checker:
  port: 9999
  path: /am-i-healthy
server:
  port: 1010
  company-domain: acme.com
  ca-cert-path: /my-ca-path
  cert-path: /my-cert-path
  key-path: /my-key-path
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
	var config Configuration
	err := yaml.Unmarshal([]byte(source), &config)
	require.NoError(t, err, "the test configuration should unmarshal")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "missing required parameter server.namespace")
}

func TestMultipleRulesCanNotHaveTheSameName(t *testing.T) {
	var source = `---
log-level: debug
check-existing: true
health-checker:
  port: 9999
  path: /am-i-healthy
server:
  port: 1010
  namespace: test-namespace
  service: graffiti-service
  company-domain: acme.com
  ca-cert-path: /my-ca-path
  cert-path: /my-cert-path
  key-path: /my-key-path
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
	var config Configuration
	err := yaml.Unmarshal([]byte(source), &config)
	require.NoError(t, err, "the test configuration should unmarshal")
	err = config.ValidateConfig()
	assert.EqualError(t, err, "rule my-rule is invalid - found duplicate rules with the same name, they must be unique", "two rules with the same name should cause a validation error")
}
