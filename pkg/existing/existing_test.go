package existing

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"stash.hcom/run/kube-graffiti/pkg/config"
)

// mockDiscoveryClient implements our apiDiscoverer interface and so allows testing of methods which call kube discovery api.
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

// mockDynamicInterface can be used in place of dynamic.Interface objects
type mockDynamicInterface struct {
	mock.Mock
}

func (mdc *mockDynamicInterface) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := mdc.Called(resource)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

// mockDynamicNamespaceableResourceInterface can be used in place of dynamic.NamespaceableResourceInterface objects
type mockDynamicNamespaceableResourceInterface struct {
	mock.Mock
	mockDynamicResourceInterface
}

func (nrc *mockDynamicNamespaceableResourceInterface) Namespace(ns string) dynamic.ResourceInterface {
	args := nrc.Called(ns)
	return args.Get(0).(dynamic.ResourceInterface)
}

// mockDynamicResourceInterface can be used in place of dynamic.ResourceInterface objects
type mockDynamicResourceInterface struct {
	mock.Mock
}

func (rc *mockDynamicResourceInterface) Create(obj *unstructured.Unstructured, subresources ...string) (*unstructured.Unstructured, error) {
	args := rc.Called(obj, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (rc *mockDynamicResourceInterface) Update(obj *unstructured.Unstructured, subresources ...string) (*unstructured.Unstructured, error) {
	args := rc.Called(obj, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (rc *mockDynamicResourceInterface) UpdateStatus(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	args := rc.Called(obj)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (rc *mockDynamicResourceInterface) Delete(name string, options *metav1.DeleteOptions, subresources ...string) error {
	args := rc.Called(name, options, subresources)
	return args.Error(0)
}

func (rc *mockDynamicResourceInterface) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	args := rc.Called(options, listOptions)
	return args.Error(0)
}

func (rc *mockDynamicResourceInterface) Get(name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := rc.Called(name, options, subresources)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

func (rc *mockDynamicResourceInterface) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	args := rc.Called(opts)
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (rc *mockDynamicResourceInterface) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	args := rc.Called(opts)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (rc *mockDynamicResourceInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*unstructured.Unstructured, error) {
	args := rc.Called(name, pt, data, subresources)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
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

func defaultTestDiscoveryClient(t *testing.T) apiDiscoverer {
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
	dc.On("ServerGroups").Return(&sg, nil)
	dc.On("ServerResources").Return(srp, nil)
	return dc
}
func TestCachingDiscoveredAPISandResources(t *testing.T) {
	dc := defaultTestDiscoveryClient(t)
	discoveryClient = dc

	// call the discovery method
	err := discoverAPIsAndResources()
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

func TestCheckRuleModifiesANamespaceObject(t *testing.T) {
	// create a rule which adds a label to a namespace with label fruit=apple
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

	// the test resource to apply the rule against.
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

	// set up the mock dynamic client to receive the expected patch request
	nri := mockDynamicNamespaceableResourceInterface{}
	// because both mockDynamicNamespaceableResourceInterface and the embedded type mockDynamicResourceInterface both have On methods, we need to
	// make sure that we correctly set On for the embedded type otherwise we end up calling it on the parent and then getting an unexpected call error.
	nri.mockDynamicResourceInterface.On("Patch", "test-namespace", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)
	dc := mockDynamicInterface{}
	dc.On("Resource", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}).Return(&nri)
	// set the package to use the mocked client
	dynamicClient = &dc

	// finally, call the checkObject method - the one we're testing...
	result := checkObject(&rule, "v1", "namespaces", resourceObject)
	nri.AssertExpectations(t)
	dc.AssertExpectations(t)
	assert.Equal(t, true, result, "checkObject should have patched the object")
}

func TestCheckRuleDoesNotMatchObject(t *testing.T) {
	// create a rule which adds a label to a namespace with label fruit=banana
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
  - "fruit=banana"
additions:
  labels:
    added: 'by-graffiti'
`
	var rule config.Rule
	err := yaml.Unmarshal([]byte(ruleYaml), &rule)
	require.NoError(t, err, "yaml unmarshalling of rule should not fail")

	// the test resource to apply the rule against - this does not have fruit=banana and so should not be patched.
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

	// finally, call the checkObject method - the one we're testing...
	result := checkObject(&rule, "v1", "namespaces", resourceObject)
	assert.Equal(t, false, result, "checkObject should not have patched the object")
}

func TestCheckRulePatchesNamespacedObjectOk(t *testing.T) {
	// create a rule which adds a label to a namespace with label fruit=banana
	var ruleYaml = `---
registration:
  name: add-a-label
  targets:
  - api-groups:
    - apps
    api-versions:
    - v1
    resources:
    - deployments
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

	// the test deploy that should match our rule
	var deployJSON = `{
		"apiVersion": "extensions/v1beta1",
		"kind": "Deployment",
		"metadata": {
			"annotations": {
				"deployment.kubernetes.io/revision": "1"
			},
			"creationTimestamp": "2018-09-10T20:22:29Z",
			"generation": 1,
			"labels": {
				"run": "nginx",
				"fruit": "apple"
			},
			"name": "nginx",
			"namespace": "test-namespace",
			"resourceVersion": "38611",
			"selfLink": "/apis/extensions/v1beta1/namespaces/test-namespace/deployments/nginx",
			"uid": "3d542468-b537-11e8-990c-08002722bfc3"
		},
		"spec": {
			"progressDeadlineSeconds": 600,
			"replicas": 1,
			"revisionHistoryLimit": 2,
			"selector": {
				"matchLabels": {
					"run": "nginx"
				}
			},
			"strategy": {
				"rollingUpdate": {
					"maxSurge": "25%",
					"maxUnavailable": "25%"
				},
				"type": "RollingUpdate"
			},
			"template": {
				"metadata": {
					"creationTimestamp": null,
					"labels": {
						"run": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"imagePullPolicy": "Always",
							"name": "nginx",
							"resources": {},
							"terminationMessagePath": "/dev/termination-log",
							"terminationMessagePolicy": "File"
						}
					],
					"dnsPolicy": "ClusterFirst",
					"restartPolicy": "Always",
					"schedulerName": "default-scheduler",
					"securityContext": {},
					"terminationGracePeriodSeconds": 30
				}
			}
		},
		"status": {
			"availableReplicas": 1,
			"conditions": [
				{
					"lastTransitionTime": "2018-09-10T20:22:39Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "Deployment has minimum availability.",
					"reason": "MinimumReplicasAvailable",
					"status": "True",
					"type": "Available"
				},
				{
					"lastTransitionTime": "2018-09-10T20:22:29Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "ReplicaSet \"nginx-65899c769f\" has successfully progressed.",
					"reason": "NewReplicaSetAvailable",
					"status": "True",
					"type": "Progressing"
				}
			],
			"observedGeneration": 1,
			"readyReplicas": 1,
			"replicas": 1,
			"updatedReplicas": 1
		}
	}`
	var resourceObject unstructured.Unstructured
	err = json.Unmarshal([]byte(deployJSON), &resourceObject.Object)
	require.NoError(t, err, "json unmarshalling of namespace resource should not fail")

	// set up the mock dynamic client to receive the expected patch request
	ri := mockDynamicResourceInterface{}
	ri.On("Patch", "nginx", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)

	nri := mockDynamicNamespaceableResourceInterface{}
	nri.On("Namespace", "test-namespace").Return(&ri)

	dc := mockDynamicInterface{}
	dc.On("Resource", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Return(&nri)
	// set the package to use the mocked client
	dynamicClient = &dc

	// finally, call the checkObject method - the one we're testing...
	result := checkObject(&rule, "apps/v1", "deployments", resourceObject)
	assert.Equal(t, true, result, "checkObject should have patched the object")

	dc.AssertExpectations(t)
	nri.AssertExpectations(t)
	ri.AssertExpectations(t)
}

func TestCheckRulePatchesNamespacedObjectWithMatchingNamespaceSelector(t *testing.T) {
	// create a rule which adds a label to a deployment with label run=nginx if namespace has label match fruit=apple
	var ruleYaml = `---
registration:
  name: add-a-label
  targets:
  - api-groups:
    - apps
    api-versions:
    - v1
    resources:
    - deployments
  failure-policy: Ignore
  namespace-selector: fruit=apple
matchers:
  label-selectors:
  - "run=nginx"
additions:
  labels:
    added: 'by-graffiti'
`
	var rule config.Rule
	err := yaml.Unmarshal([]byte(ruleYaml), &rule)
	require.NoError(t, err, "yaml unmarshalling of rule should not fail")

	// the test deploy that should match our rule
	var deployJSON = `{
		"apiVersion": "extensions/v1beta1",
		"kind": "Deployment",
		"metadata": {
			"annotations": {
				"deployment.kubernetes.io/revision": "1"
			},
			"creationTimestamp": "2018-09-10T20:22:29Z",
			"generation": 1,
			"labels": {
				"run": "nginx",
				"fruit": "apple"
			},
			"name": "nginx",
			"namespace": "test-namespace",
			"resourceVersion": "38611",
			"selfLink": "/apis/extensions/v1beta1/namespaces/test-namespace/deployments/nginx",
			"uid": "3d542468-b537-11e8-990c-08002722bfc3"
		},
		"spec": {
			"progressDeadlineSeconds": 600,
			"replicas": 1,
			"revisionHistoryLimit": 2,
			"selector": {
				"matchLabels": {
					"run": "nginx"
				}
			},
			"strategy": {
				"rollingUpdate": {
					"maxSurge": "25%",
					"maxUnavailable": "25%"
				},
				"type": "RollingUpdate"
			},
			"template": {
				"metadata": {
					"creationTimestamp": null,
					"labels": {
						"run": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"imagePullPolicy": "Always",
							"name": "nginx",
							"resources": {},
							"terminationMessagePath": "/dev/termination-log",
							"terminationMessagePolicy": "File"
						}
					],
					"dnsPolicy": "ClusterFirst",
					"restartPolicy": "Always",
					"schedulerName": "default-scheduler",
					"securityContext": {},
					"terminationGracePeriodSeconds": 30
				}
			}
		},
		"status": {
			"availableReplicas": 1,
			"conditions": [
				{
					"lastTransitionTime": "2018-09-10T20:22:39Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "Deployment has minimum availability.",
					"reason": "MinimumReplicasAvailable",
					"status": "True",
					"type": "Available"
				},
				{
					"lastTransitionTime": "2018-09-10T20:22:29Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "ReplicaSet \"nginx-65899c769f\" has successfully progressed.",
					"reason": "NewReplicaSetAvailable",
					"status": "True",
					"type": "Progressing"
				}
			],
			"observedGeneration": 1,
			"readyReplicas": 1,
			"replicas": 1,
			"updatedReplicas": 1
		}
	}`
	var resourceObject unstructured.Unstructured
	err = json.Unmarshal([]byte(deployJSON), &resourceObject.Object)
	require.NoError(t, err, "json unmarshalling of namespace resource should not fail")

	// set up the mock dynamic client to receive the expected patch request
	ri := mockDynamicResourceInterface{}
	ri.On("Patch", "nginx", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)

	nri := mockDynamicNamespaceableResourceInterface{}
	nri.On("Namespace", "test-namespace").Return(&ri)

	dc := mockDynamicInterface{}
	dc.On("Resource", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Return(&nri)
	// set the package to use the mocked client
	dynamicClient = &dc

	// use the helper function in namespace_cache_test.go to set up the package level namespace cache
	nsCache = defaultTestNamespaceCache(t)

	// finally, call the checkObject method - the one we're testing...
	result := checkObject(&rule, "apps/v1", "deployments", resourceObject)
	assert.Equal(t, true, result, "checkObject should have patched the object")

	dc.AssertExpectations(t)
	nri.AssertExpectations(t)
	ri.AssertExpectations(t)
}

func TestCheckRulePatchesNamespacedObjectWithNonMatchingNamespaceSelector(t *testing.T) {
	// create a rule which adds a label to a deploy with label run=nginx and namespace has label fruit=banana
	var ruleYaml = `---
registration:
  name: add-a-label
  targets:
  - api-groups:
    - apps
    api-versions:
    - v1
    resources:
    - deployments
  failure-policy: Ignore
  namespace-selector: fruit=banana
matchers:
  label-selectors:
  - "run=nginx"
additions:
  labels:
    added: 'by-graffiti'
`
	var rule config.Rule
	err := yaml.Unmarshal([]byte(ruleYaml), &rule)
	require.NoError(t, err, "yaml unmarshalling of rule should not fail")

	// the test deploy that should match our rule
	var deployJSON = `{
		"apiVersion": "extensions/v1beta1",
		"kind": "Deployment",
		"metadata": {
			"annotations": {
				"deployment.kubernetes.io/revision": "1"
			},
			"creationTimestamp": "2018-09-10T20:22:29Z",
			"generation": 1,
			"labels": {
				"run": "nginx",
				"fruit": "apple"
			},
			"name": "nginx",
			"namespace": "test-namespace",
			"resourceVersion": "38611",
			"selfLink": "/apis/extensions/v1beta1/namespaces/test-namespace/deployments/nginx",
			"uid": "3d542468-b537-11e8-990c-08002722bfc3"
		},
		"spec": {
			"progressDeadlineSeconds": 600,
			"replicas": 1,
			"revisionHistoryLimit": 2,
			"selector": {
				"matchLabels": {
					"run": "nginx"
				}
			},
			"strategy": {
				"rollingUpdate": {
					"maxSurge": "25%",
					"maxUnavailable": "25%"
				},
				"type": "RollingUpdate"
			},
			"template": {
				"metadata": {
					"creationTimestamp": null,
					"labels": {
						"run": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"imagePullPolicy": "Always",
							"name": "nginx",
							"resources": {},
							"terminationMessagePath": "/dev/termination-log",
							"terminationMessagePolicy": "File"
						}
					],
					"dnsPolicy": "ClusterFirst",
					"restartPolicy": "Always",
					"schedulerName": "default-scheduler",
					"securityContext": {},
					"terminationGracePeriodSeconds": 30
				}
			}
		},
		"status": {
			"availableReplicas": 1,
			"conditions": [
				{
					"lastTransitionTime": "2018-09-10T20:22:39Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "Deployment has minimum availability.",
					"reason": "MinimumReplicasAvailable",
					"status": "True",
					"type": "Available"
				},
				{
					"lastTransitionTime": "2018-09-10T20:22:29Z",
					"lastUpdateTime": "2018-09-10T20:22:39Z",
					"message": "ReplicaSet \"nginx-65899c769f\" has successfully progressed.",
					"reason": "NewReplicaSetAvailable",
					"status": "True",
					"type": "Progressing"
				}
			],
			"observedGeneration": 1,
			"readyReplicas": 1,
			"replicas": 1,
			"updatedReplicas": 1
		}
	}`
	var resourceObject unstructured.Unstructured
	err = json.Unmarshal([]byte(deployJSON), &resourceObject.Object)
	require.NoError(t, err, "json unmarshalling of namespace resource should not fail")

	// use the helper function in namespace_cache_test.go to set up the package level namespace cache
	nsCache = defaultTestNamespaceCache(t)

	// finally, call the checkObject method - the one we're testing...
	result := checkObject(&rule, "apps/v1", "deployments", resourceObject)
	assert.Equal(t, false, result, "checkObject should not have patched the object")
}

var unstructuredNamespaceListJSON = `{
	"kind":"NamespaceList",
	"metadata":{  
	   "resourceVersion":"93475",
	   "selfLink":"/api/v1/namespaces"
	},
	"items":[  
	   {  
		  "apiVersion":"v1",
		  "kind":"Namespace",
		  "metadata":{  
			 "creationTimestamp":"2018-09-10T09:34:31Z",
			 "name":"default",
			 "resourceVersion":"561",
			 "selfLink":"/api/v1/namespaces/default",
			 "uid":"b8337c4c-b4dc-11e8-990c-08002722bfc3"
		  },
		  "spec":{  
			 "finalizers":[  
				"kubernetes"
			 ]
		  },
		  "status":{  
			 "phase":"Active"
		  }
	   },
	   {  
		  "apiVersion":"v1",
		  "kind":"Namespace",
		  "metadata":{  
			 "annotations":{  
				"iam.amazonaws.com/permitted":".*",
				"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Namespace\",\"metadata\":{\"annotations\":{},\"name\":\"kube-graffiti\",\"namespace\":\"\"}}\n"
			 },
			 "creationTimestamp":"2018-09-10T09:36:21Z",
			 "name":"kube-graffiti",
			 "resourceVersion":"93117",
			 "selfLink":"/api/v1/namespaces/kube-graffiti",
			 "uid":"fa0bd159-b4dc-11e8-990c-08002722bfc3"
		  },
		  "spec":{  
			 "finalizers":[  
				"kubernetes"
			 ]
		  },
		  "status":{  
			 "phase":"Active"
		  }
	   },
	   {  
		  "apiVersion":"v1",
		  "kind":"Namespace",
		  "metadata":{  
			 "creationTimestamp":"2018-09-10T09:34:35Z",
			 "name":"kube-public",
			 "resourceVersion":"563",
			 "selfLink":"/api/v1/namespaces/kube-public",
			 "uid":"baa8ff7c-b4dc-11e8-990c-08002722bfc3"
		  },
		  "spec":{  
			 "finalizers":[  
				"kubernetes"
			 ]
		  },
		  "status":{  
			 "phase":"Active"
		  }
	   },
	   {  
		  "apiVersion":"v1",
		  "kind":"Namespace",
		  "metadata":{  
			 "annotations":{  
				"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Namespace\",\"metadata\":{\"annotations\":{},\"name\":\"kube-system\",\"namespace\":\"\"}}\n"
			 },
			 "creationTimestamp":"2018-09-10T09:34:31Z",
			 "name":"kube-system",
			 "resourceVersion":"564",
			 "selfLink":"/api/v1/namespaces/kube-system",
			 "uid":"b836fe94-b4dc-11e8-990c-08002722bfc3"
		  },
		  "spec":{  
			 "finalizers":[  
				"kubernetes"
			 ]
		  },
		  "status":{  
			 "phase":"Active"
		  }
	   },
	   {  
		  "apiVersion":"v1",
		  "kind":"Namespace",
		  "metadata":{  
			 "annotations":{  
				"iam.amazonaws.com/permitted":".*"
			 },
			 "creationTimestamp":"2018-09-10T20:20:03Z",
			 "labels":{  
				"fruit": "apple"
			 },
			 "name":"test-namespace",
			 "resourceVersion":"38415",
			 "selfLink":"/api/v1/namespaces/test-namespace",
			 "uid":"e67c4503-b536-11e8-990c-08002722bfc3"
		  },
		  "spec":{  
			 "finalizers":[  
				"kubernetes"
			 ]
		  },
		  "status":{  
			 "phase":"Active"
		  }
	   }
	]
 }
 `

func TestTraverseKubePatchingAllNamespaces(t *testing.T) {
	// A.K.A - the BIG one! :)
	// create a rule which adds a label to all namespace with label fruit=apple
	var rulesYaml = `---
- registration:
    name: add-a-label
    targets:
    - api-groups:
      - ""
      api-versions:
      - "*"
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
	var rules []config.Rule
	err := yaml.Unmarshal([]byte(rulesYaml), &rules)
	require.NoError(t, err, "yaml unmarshalling of rules should not fail")

	// set up package to use canned discovery client and load the canned discovered groups and resources
	discoveryClient = defaultTestDiscoveryClient(t)
	err = discoverAPIsAndResources()
	require.NoError(t, err, "we should not get an error loading in canned resource groups and resources")

	// use the helper function in namespace_cache_test.go to set up the package level namespace cache
	nsCache = defaultTestNamespaceCache(t)

	// unstructured list of namespaces
	ulns := new(unstructured.UnstructuredList)
	err = json.Unmarshal([]byte(unstructuredNamespaceListJSON), ulns)
	fmt.Printf("error: %v", err)
	require.NoError(t, err, "we should be able to unmarshal our canned namespace list into an UnstructuedList")

	// set up the mock dynamic client to receive the expected patch request
	nri := mockDynamicNamespaceableResourceInterface{}
	// because both mockDynamicNamespaceableResourceInterface and the embedded type mockDynamicResourceInterface both have On methods, we need to
	// make sure that we correctly set On for the embedded type otherwise we end up calling it on the parent and then getting an unexpected call error.
	nri.mockDynamicResourceInterface.On("List", mock.AnythingOfType("v1.ListOptions")).Return(ulns, nil)
	nri.mockDynamicResourceInterface.On("Patch", "test-namespace", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)
	dc := mockDynamicInterface{}
	dc.On("Resource", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}).Return(&nri)
	// set the package to use the mocked client
	dynamicClient = &dc

	CheckRulesAgainstExistingState(rules)
	nri.AssertExpectations(t)
	dc.AssertExpectations(t)
}

var unstructuredDeployListJSON = `{
	"apiVersion":"apps/v1",
	"items":[
	   {
		  "apiVersion":"apps/v1",
		  "kind":"Deployment",
		  "metadata":{
			 "annotations":{
				"deployment.kubernetes.io/revision":"1"
			 },
			 "creationTimestamp":"2018-09-10T20:22:29Z",
			 "generation":1,
			 "labels":{
				"fruit": "apple",
				"run":"nginx"
			 },
			 "name":"nginx",
			 "namespace":"test-namespace",
			 "resourceVersion":"38611",
			 "selfLink":"/apis/apps/v1/namespaces/test-namespace/deployments/nginx",
			 "uid":"3d542468-b537-11e8-990c-08002722bfc3"
		  },
		  "spec":{
			 "progressDeadlineSeconds":600,
			 "replicas":1,
			 "revisionHistoryLimit":2,
			 "selector":{
				"matchLabels":{
				   "run":"nginx"
				}
			 },
			 "strategy":{
				"rollingUpdate":{
				   "maxSurge":"25%",
				   "maxUnavailable":"25%"
				},
				"type":"RollingUpdate"
			 },
			 "template":{
				"metadata":{
				   "creationTimestamp":null,
				   "labels":{
					  "run":"nginx"
				   }
				},
				"spec":{
				   "containers":[
					  {
						 "image":"nginx",
						 "imagePullPolicy":"Always",
						 "name":"nginx",
						 "resources":{
 
						 },
						 "terminationMessagePath":"/dev/termination-log",
						 "terminationMessagePolicy":"File"
					  }
				   ],
				   "dnsPolicy":"ClusterFirst",
				   "restartPolicy":"Always",
				   "schedulerName":"default-scheduler",
				   "securityContext":{
 
				   },
				   "terminationGracePeriodSeconds":30
				}
			 }
		  },
		  "status":{
			 "availableReplicas":1,
			 "conditions":[
				{
				   "lastTransitionTime":"2018-09-10T20:22:39Z",
				   "lastUpdateTime":"2018-09-10T20:22:39Z",
				   "message":"Deployment has minimum availability.",
				   "reason":"MinimumReplicasAvailable",
				   "status":"True",
				   "type":"Available"
				},
				{
				   "lastTransitionTime":"2018-09-10T20:22:29Z",
				   "lastUpdateTime":"2018-09-10T20:22:39Z",
				   "message":"ReplicaSet \"nginx-65899c769f\" has successfully progressed.",
				   "reason":"NewReplicaSetAvailable",
				   "status":"True",
				   "type":"Progressing"
				}
			 ],
			 "observedGeneration":1,
			 "readyReplicas":1,
			 "replicas":1,
			 "updatedReplicas":1
		  }
	   }
	],
	"kind":"DeploymentList",
	"metadata":{
	   "resourceVersion":"93475",
	   "selfLink":"/apis/apps/v1/deployments"
	}
 }`

func TestTraverseKubePatchingAllNamespacesWildcardsInRegistration(t *testing.T) {
	// A.K.A - the BIG one! :)
	// create a rule which adds a label to all namespace with label fruit=apple
	var rulesYaml = `---
- registration:
    name: add-a-label
    targets:
    - api-groups:
      - "*"
      api-versions:
      - "*"
      resources:
      - "*"
    failure-policy: Ignore
  matchers:
    label-selectors:
    - "fruit=apple"
  additions:
    labels:
      added: 'by-graffiti'
`
	var rules []config.Rule
	err := yaml.Unmarshal([]byte(rulesYaml), &rules)
	require.NoError(t, err, "yaml unmarshalling of rules should not fail")

	// set up package to use canned discovery client and load the canned discovered groups and resources
	discoveryClient = defaultTestDiscoveryClient(t)
	err = discoverAPIsAndResources()
	require.NoError(t, err, "we should not get an error loading in canned resource groups and resources")

	// use the helper function in namespace_cache_test.go to set up the package level namespace cache
	nsCache = defaultTestNamespaceCache(t)

	// unstructured list of namespaces
	ulns := new(unstructured.UnstructuredList)
	err = json.Unmarshal([]byte(unstructuredNamespaceListJSON), ulns)
	require.NoError(t, err, "we should be able to unmarshal our canned namespace list into an UnstructuedList")

	// set up the mock dynamic client to receive the expected patch request
	nri := mockDynamicNamespaceableResourceInterface{}
	// because both mockDynamicNamespaceableResourceInterface and the embedded type mockDynamicResourceInterface both have On methods, we need to
	// make sure that we correctly set On for the embedded type otherwise we end up calling it on the parent and then getting an unexpected call error.
	nri.mockDynamicResourceInterface.On("List", mock.AnythingOfType("v1.ListOptions")).Return(ulns, nil)
	nri.mockDynamicResourceInterface.On("Patch", "test-namespace", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)

	// unstructured list of deployments
	dl := new(unstructured.UnstructuredList)
	err = json.Unmarshal([]byte(unstructuredDeployListJSON), dl)
	require.NoError(t, err, "we should be able to unmarshal our canned deployment list into an UnstructuedList")

	// set up the mock dynamic client to receive the expected patch request
	dri := mockDynamicResourceInterface{}
	dri.On("Patch", "nginx", types.JSONPatchType, mock.AnythingOfType("[]uint8"), mock.AnythingOfType("[]string")).Return(nil, nil)
	dnri := mockDynamicNamespaceableResourceInterface{}
	// List is called on the namespaceable interface but patch is called on the namespaced interface!
	dnri.mockDynamicResourceInterface.On("List", mock.AnythingOfType("v1.ListOptions")).Return(dl, nil)
	dnri.On("Namespace", "test-namespace").Return(&dri)

	dc := mockDynamicInterface{}
	dc.On("Resource", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}).Return(&nri)
	dc.On("Resource", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces/finalize"}).Return(&nri)
	dc.On("Resource", schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces/status"}).Return(&nri)
	dc.On("Resource", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Return(&dnri)
	dc.On("Resource", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments/scale"}).Return(&dnri)
	dc.On("Resource", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments/status"}).Return(&dnri)
	// set the package to use the mocked client
	dynamicClient = &dc

	CheckRulesAgainstExistingState(rules)
	nri.AssertExpectations(t)
	dri.AssertExpectations(t)
	dnri.AssertExpectations(t)
	dc.AssertExpectations(t)
}
