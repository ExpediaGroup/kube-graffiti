package existing

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	jsonNamespace = `{
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
	jsonDeploy = `{
		"apiVersion": "extensions/v1beta1",
		"kind": "Deployment",
		"metadata": {
			"annotations": {
				"deployment.kubernetes.io/revision": "1"
			},
			"creationTimestamp": "2018-09-10T20:22:29Z",
			"generation": 1,
			"labels": {
				"run": "nginx"
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
)

func TestGettingLabelsFromAYamlMapInterfaceInterface(t *testing.T) {
	var yamlNamespace = `apiVersion: v1
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

	var obj map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(yamlNamespace), &obj)
	require.NoError(t, err)

	labels := lookupLabels(obj["metadata"])
	assert.Equal(t, 2, len(labels), "there are two labels in the test namespace")
	col, ok := labels["colour"]
	assert.Equal(t, true, ok, "the label colour should be found")
	assert.Equal(t, "green", col, "the value of label colour should be green")
}

func TestHandleWhenObjectLabelsIsNotAMap(t *testing.T) {
	obj := make(map[string]string)
	obj["labels"] = "this is not a map"

	labels := lookupLabels(obj)
	assert.Equal(t, 0, len(labels), "there are no labels map, so no labels")
}

func TestHandleWhenObjectLabelsIsNotAStringOrInterface(t *testing.T) {
	ints := make(map[int]string)
	ints[100] = "david"

	obj := make(map[string]map[int]string)
	obj["labels"] = ints

	labels := lookupLabels(obj)
	assert.Equal(t, 0, len(labels), "there are labels, but the key is not a string")
}

func TestGettingLabelsFromAJSONMapStringInterface(t *testing.T) {
	var ns map[string]interface{}
	err := json.Unmarshal([]byte(jsonNamespace), &ns)
	require.NoError(t, err)

	labels := lookupLabels(ns["metadata"])
	assert.Equal(t, 2, len(labels), "there are two labels in the test namespace")
	col, ok := labels["colour"]
	assert.Equal(t, true, ok, "the label colour should be found")
	assert.Equal(t, "green", col, "the value of label colour should be green")
}

func TestLookupOfLabelsWithNonMapDoesNotPanicAndReturnsEmptyMap(t *testing.T) {
	var object struct{}
	var desiredType map[string]string

	labels := lookupLabels(object)
	assert.IsType(t, desiredType, labels, "lables should be a map[string]string")
	assert.Equal(t, 0, len(labels), "labels should be empty")
}

func TestNamespaceSelectorAgainstANamespaceMatchesItsLabelsTestSuccess(t *testing.T) {
	var ns map[string]interface{}
	err := json.Unmarshal([]byte(jsonNamespace), &ns)
	require.NoError(t, err)

	result, err := matchNamespaceSelector(ns, "fruit = apple", namespaceCache{})
	assert.NoError(t, err, "it should be able to match again the fruit label in this namespace")
	assert.Equal(t, true, result, "the match result should be true")
}

func TestNamespaceSelectorAgainstANamespaceMatchesItsLabelsTestFail(t *testing.T) {
	var ns map[string]interface{}
	err := json.Unmarshal([]byte(jsonNamespace), &ns)
	require.NoError(t, err)

	result, err := matchNamespaceSelector(ns, "fruit = banana", namespaceCache{})
	assert.NoError(t, err, "it should be able to match again the fruit label in this namespace")
	assert.Equal(t, false, result, "the match result should be false")
}

func TestNamespaceSelectorAgainstANamespaceInvalidSelector(t *testing.T) {
	var ns map[string]interface{}
	err := json.Unmarshal([]byte(jsonNamespace), &ns)
	require.NoError(t, err)

	result, err := matchNamespaceSelector(ns, "this is not a correct label selector", namespaceCache{})
	assert.Error(t, err, "we should get an error caused by the bad selector")
	assert.Equal(t, false, result, "the match result should be false")
}

func TestNamespaceSelectorAgainstObjectWithoutMetadata(t *testing.T) {
	ns := make(map[string]interface{})

	result, err := matchNamespaceSelector(ns, "fruit = apple", namespaceCache{})
	assert.Error(t, err, "we should get an error caused by the lack of metadata")
	assert.Errorf(t, err, "object has no metadata", "we should get the right error message")
	assert.Equal(t, false, result, "the match result should be false")
}

func TestLookupOfObjectWithoutKindIsHandled(t *testing.T) {
	var ns map[string]interface{}
	err := json.Unmarshal([]byte(jsonNamespace), &ns)
	require.NoError(t, err)
	delete(ns, "kind")

	result, err := matchNamespaceSelector(ns, "fruit = apple", namespaceCache{})
	assert.Error(t, err, "we should get an error caused by the lack of kind")
	assert.Errorf(t, err, "this object seems to have no kind", "we should get the right error message")
	assert.Equal(t, false, result, "the match result should be false")
}

func TestAClusterScopedObjectCanNotMatchANamespaceSelector(t *testing.T) {
	var jsonClusterRole = `{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind": "ClusterRole",
		"metadata": {
			"annotations": {
				"rbac.authorization.kubernetes.io/autoupdate": "true"
			},
			"creationTimestamp": "2018-09-10T09:34:31Z",
			"labels": {
				"kubernetes.io/bootstrapping": "rbac-defaults",
				"fruit": "apple"
			},
			"name": "cluster-admin",
			"resourceVersion": "11",
			"selfLink": "/apis/rbac.authorization.k8s.io/v1/clusterroles/cluster-admin",
			"uid": "b8399072-b4dc-11e8-990c-08002722bfc3"
		},
		"rules": [
			{
				"apiGroups": [
					"*"
				],
				"resources": [
					"*"
				],
				"verbs": [
					"*"
				]
			},
			{
				"nonResourceURLs": [
					"*"
				],
				"verbs": [
					"*"
				]
			}
		]
	}`
	role := make(map[string]interface{})
	err := json.Unmarshal([]byte(jsonClusterRole), &role)
	require.NoError(t, err)

	result, err := matchNamespaceSelector(role, "fruit = apple", namespaceCache{})
	assert.NoError(t, err, "we should not get an error when evaluating a cluster scoped object against a namespace selector")
	assert.Equal(t, false, result, "the match result should be false, the object is not namespaced or a namespace so shouldn't match")
}

func TestNamespaceSelectorAgainstAnObjectsNamespaceMatch(t *testing.T) {
	var deploy map[string]interface{}
	err := json.Unmarshal([]byte(jsonDeploy), &deploy)
	require.NoError(t, err)

	nl := new(corev1.NamespaceList)
	// use our testNamespace list we used to test the namespace cache...
	err = json.Unmarshal([]byte(testNamespaceList), nl)
	require.NoError(t, err)
	fw := watch.NewFake()

	// when we call our mock lister-watcher return our canned namespace list
	lw := new(mockNamespaceListerWatcherGetter)
	// lo := metav1.ListOptions{}
	lw.On("List", mock.AnythingOfType("v1.ListOptions")).Return(nl, nil)
	lw.On("Watch", mock.AnythingOfType("v1.ListOptions")).Return(fw, nil)

	// start the store with fake reflector
	var ns *corev1.Namespace
	store, reflector := cache.NewNamespaceKeyedIndexerAndReflector(lw, ns, time.Duration(0))

	mycache := namespaceCache{
		store:     store,
		reflector: reflector,
		getter:    lw,
	}
	stop := make(chan struct{})
	defer close(stop)
	mycache.StartNamespaceReflector(stop)

	// allow reflector to have started...
	time.Sleep(1 * time.Second)

	// finally check our deploy - which will invoke the looking up of its namespace
	result, err := matchNamespaceSelector(deploy, "fruit=apple", mycache)
	assert.NoError(t, err, "we should not get an error")
	assert.Equal(t, true, result, "the match result should be true because the namespace test-namespace does match the selector")
}

func TestNamespaceSelectorAgainstAnObjectsMiss(t *testing.T) {
	var deploy map[string]interface{}
	err := json.Unmarshal([]byte(jsonDeploy), &deploy)
	require.NoError(t, err)

	nl := new(corev1.NamespaceList)
	// use our testNamespace list we used to test the namespace cache...
	err = json.Unmarshal([]byte(testNamespaceList), nl)
	require.NoError(t, err)
	fw := watch.NewFake()

	// when we call our mock lister-watcher return our canned namespace list
	lw := new(mockNamespaceListerWatcherGetter)
	// lo := metav1.ListOptions{}
	lw.On("List", mock.AnythingOfType("v1.ListOptions")).Return(nl, nil)
	lw.On("Watch", mock.AnythingOfType("v1.ListOptions")).Return(fw, nil)

	// start the store with fake reflector
	var ns *corev1.Namespace
	store, reflector := cache.NewNamespaceKeyedIndexerAndReflector(lw, ns, time.Duration(0))

	mycache := namespaceCache{
		store:     store,
		reflector: reflector,
		getter:    lw,
	}
	stop := make(chan struct{})
	defer close(stop)
	mycache.StartNamespaceReflector(stop)

	// allow reflector to have started...
	time.Sleep(1 * time.Second)

	// finally check our deploy - which will invoke the looking up of its namespace
	result, err := matchNamespaceSelector(deploy, "fruit=elvis", mycache)
	assert.NoError(t, err, "we should not get an error")
	assert.Equal(t, false, result, "should be false, elvis does not match apple")
}
