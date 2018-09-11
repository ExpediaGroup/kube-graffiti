package existing

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"stash.hcom/run/kube-graffiti/pkg/config"
)

type mockDiscoveryClient struct {
	mock.Mock
}

func (dc *mockDiscoveryClient) ServerGroups() (apiGroupList *metav1.APIGroupList, err error) {
	args := dc.Called()
	return args.Get(0).(*metav1.APIGroupList), args.Error(1)
}

func (dc *mockDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	args := dc.Called()
	return args.Get(0).([]*metav1.APIResourceList), args.Error(1)
}

// metaObject is used only for pulling out object metadata
type metaObject struct {
	Meta metav1.ObjectMeta `json:"metadata"`
}

var (
	// limit the api list to only core "" and apps
	testAPIList = `typemeta:
kind: APIGroupList
apiversion: v1
groups:
- typemeta:
    kind: ""
    apiversion: ""
  name: ""
  versions:
  - groupversion: v1
    version: v1
  preferredversion:
    groupversion: v1
    version: v1
  serveraddressbyclientcidrs: []
- typemeta:
    kind: ""
    apiversion: ""
  name: apps
  versions:
  - groupversion: apps/v1
    version: v1
  - groupversion: apps/v1beta2
    version: v1beta2
  - groupversion: apps/v1beta1
    version: v1beta1
  preferredversion:
    groupversion: apps/v1
    version: v1
  serveraddressbyclientcidrs: []
`
	// limit resources to just core/namespaces and apps/deployments
	testResourceList = `- typemeta:
    kind: APIResourceList
    apiversion: ""
  groupversion: v1
  apiresources:
  - name: namespaces
    singularname: ""
    namespaced: false
    group: ""
    version: ""
    kind: Namespace
    verbs:
    - create
    - delete
    - get
    - list
    - patch
    - update
    - watch
    shortnames:
    - ns
    categories: []
  - name: namespaces/finalize
    singularname: ""
    namespaced: false
    group: ""
    version: ""
    kind: Namespace
    verbs:
    - update
    shortnames: []
    categories: []
  - name: namespaces/status
    singularname: ""
    namespaced: false
    group: ""
    version: ""
    kind: Namespace
    verbs:
    - get
    - patch
    - update
    shortnames: []
    categories: []
- typemeta:
    kind: APIResourceList
    apiversion: v1
  groupversion: apps/v1
  apiresources:
  - name: deployments
    singularname: ""
    namespaced: true
    group: ""
    version: ""
    kind: Deployment
    verbs:
    - create
    - delete
    - deletecollection
    - get
    - list
    - patch
    - update
    - watch
    shortnames:
    - deploy
    categories:
    - all
  - name: deployments/scale
    singularname: ""
    namespaced: true
    group: autoscaling
    version: v1
    kind: Scale
    verbs:
    - get
    - patch
    - update
    shortnames: []
    categories: []
  - name: deployments/status
    singularname: ""
    namespaced: true
    group: ""
    version: ""
    kind: Deployment
    verbs:
    - get
    - patch
    - update
    shortnames: []
    categories: []
`

	testNamespace = `apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: 2018-09-10T09:34:31Z
  name: test-namespace
  labels:
    fruit: apple
    colour: green
  resourceVersion: "561"
  selfLink: /api/v1/namespaces/test-namespace
  uid: b8337c4c-b4dc-11e8-990c-08002722bfc3
spec:
  finalizers:
  - kubernetes
status:
  phase: Active`
)

func TestCachingDiscoveredAPISandResources(t *testing.T) {
	// set up some mock return values
	var sg metav1.APIGroupList
	err := yaml.Unmarshal([]byte(testAPIList), &sg)
	require.NoError(t, err)

	var sr []metav1.APIResourceList
	err = yaml.Unmarshal([]byte(testResourceList), &sr)
	require.NoError(t, err)
	// ugly need to covert our list of resources into a list of pointers to
	// those same resources
	var srp []*metav1.APIResourceList
	for i := range sr {
		srp = append(srp, &sr[i])
	}

	// set up the mock discovery client
	dc := &mockDiscoveryClient{}
	discoveryClient = dc
	dc.On("ServerGroups").Return(&sg, nil)
	dc.On("ServerResources").Return(srp, nil)

	// call the discovery method
	err = discoverAPIsAndResources()
	require.NoError(t, err)

	// now some tests that the data we think should be loaded is actually loaded
	assert.Equal(t, 2, len(discoveredAPIGroups), "there are two test api groups in our test data")
	assert.Equal(t, 2, len(discoveredResources), "there are two api resource lists in our test data")
	assert.Equal(t, 3, len(discoveredResources["v1"]), "the test core v1 api has three resource types")
	assert.Equal(t, 3, len(discoveredResources["apps/v1"]), "the test apps/v1 api has three resource types")
}

func TestServerAPIGroupLookupFailureReturnsAnError(t *testing.T) {
	// set up some mock return values
	var sg metav1.APIGroupList
	err := yaml.Unmarshal([]byte(testAPIList), &sg)
	require.NoError(t, err)

	// empty server resource list resturn value...
	var srl []*metav1.APIResourceList

	dc := &mockDiscoveryClient{}
	discoveryClient = dc
	dc.On("ServerGroups").Return(&sg, nil)
	dc.On("ServerResources").Return(srl, errors.New("something went wrong"))

	err = discoverAPIsAndResources()
	require.Error(t, err, "we should return an error when server api groups fail")
}

func TestServerAPIResourcesLookupFailureReturnsAnError(t *testing.T) {
	// set up the mock discovery client
	var sg metav1.APIGroupList
	dc := &mockDiscoveryClient{}
	discoveryClient = dc
	dc.On("ServerGroups").Return(&sg, errors.New("something went wrong"))

	err := discoverAPIsAndResources()
	require.Error(t, err, "we should return an error when server api groups fail")
}

func TestCheckObjectModifiesANamespace(t *testing.T) {
	// create a rule which adds a label to a namespace
	var ruleYaml = `---
registration:
  name: add-a-label
  targets:
  - api-groups:
    - ""
    api-versions:
    - v1
    resources:
    - namespaces
  failure-policy: Ignore
matchers:
  label-selectors:
  - "fruit=apple"
additions:
  labels:
    added: 'by-graffiti'
`
	var rule config.Rule
	err := yaml.Unmarshal([]byte(ruleYaml), &rule)
	require.NoError(t, err, "yaml unmarshalling of rule should not fail")

	// the resource to apply the rule against.
	var resourceJSON = `{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
			"creationTimestamp": "2018-09-10T09:34:31Z",
			"labels": {
				"fruit": "apple",
				"colour": "green"
			},
			"name": "test-namespace",
			"resourceVersion": "561",
			"selfLink": "/api/v1/namespaces/test-namespace",
			"uid": "b8337c4c-b4dc-11e8-990c-08002722bfc3"
		},
		"spec": {
			"finalizers": [
				"kubernetes"
			]
		},
		"status": {
			"phase": "Active"
		}
	}`
	var resourceObject unstructured.Unstructured
	err = json.Unmarshal([]byte(resourceJSON), &resourceObject.Object)
	require.NoError(t, err, "json unmarshalling of namespace resource should not fail")
	checkObject(&rule, "v1", "test-namespace", resourceObject)
	panic("boo!")
}
