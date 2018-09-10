package existing

import (
	"errors"
	"testing"
	"time"

	"encoding/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	// "github.com/davecgh/go-spew/spew"
)

const (
	testNamespaceList = `{  
	"metadata":{  
		"selfLink":"/api/v1/namespaces",
		"resourceVersion":"43832"
	},
	"items":[  
		{  
			"metadata":{  
				"name":"test-namespace",
				"selfLink":"/api/v1/namespaces/test-namespace",
				"uid":"6edb500b-b1c9-11e8-979e-080027056a4c",
				"resourceVersion":"4954",
				"creationTimestamp":"2018-09-06T11:38:54Z",
				"labels":{  
				"fruit": "apple",
				"colour": "green"
				},
				"annotations":{  
				"iam.amazonaws.com/permitted":".*"
				}
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
			"metadata":{  
				"name":"default",
				"selfLink":"/api/v1/namespaces/default",
				"uid":"d40e6433-b1c0-11e8-979e-080027056a4c",
				"resourceVersion":"629",
				"creationTimestamp":"2018-09-06T10:37:18Z",
				"labels":{  
				"name":"default"
				}
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
			"metadata":{  
				"name":"kube-graffiti",
				"selfLink":"/api/v1/namespaces/kube-graffiti",
				"uid":"3d6bb610-b1c1-11e8-979e-080027056a4c",
				"resourceVersion":"3598",
				"creationTimestamp":"2018-09-06T10:40:15Z",
				"labels":{  
				"istio-injection":"enabled",
				"name":"kube-graffiti"
				},
				"annotations":{  
				"iam.amazonaws.com/permitted":".*",
				"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Namespace\",\"metadata\":{\"annotations\":{},\"name\":\"kube-graffiti\",\"namespace\":\"\"}}\n"
				}
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
			"metadata":{  
				"name":"kube-public",
				"selfLink":"/api/v1/namespaces/kube-public",
				"uid":"d676c659-b1c0-11e8-979e-080027056a4c",
				"resourceVersion":"636",
				"creationTimestamp":"2018-09-06T10:37:22Z",
				"labels":{  
				"name":"kube-public"
				}
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
			"metadata":{  
				"name":"kube-system",
				"selfLink":"/api/v1/namespaces/kube-system",
				"uid":"d4024ffa-b1c0-11e8-979e-080027056a4c",
				"resourceVersion":"637",
				"creationTimestamp":"2018-09-06T10:37:18Z",
				"labels":{  
				"name":"kube-system"
				},
				"annotations":{  
				"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Namespace\",\"metadata\":{\"annotations\":{},\"name\":\"kube-system\",\"namespace\":\"\"}}\n"
				}
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
			"metadata":{  
				"name":"mobile-team",
				"selfLink":"/api/v1/namespaces/mobile-team",
				"uid":"7c5dab5d-b1c3-11e8-979e-080027056a4c",
				"resourceVersion":"1873",
				"creationTimestamp":"2018-09-06T10:56:19Z",
				"labels":{  
				"istio-injection":"enabled",
				"name":"mobile-team"
				},
				"annotations":{  
				"iam.amazonaws.com/permitted":".*"
				}
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

	kubeSystem = `{
	"apiVersion": "v1",
	"kind": "Namespace",
	"metadata": {
		"annotations": {
			"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Namespace\",\"metadata\":{\"annotations\":{},\"name\":\"kube-system\",\"namespace\":\"\"}}\n"
		},
		"creationTimestamp": "2018-09-06T10:37:18Z",
		"labels": {
			"name": "kube-system"
		},
		"name": "kube-system",
		"resourceVersion": "637",
		"selfLink": "/api/v1/namespaces/kube-system",
		"uid": "d4024ffa-b1c0-11e8-979e-080027056a4c"
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
)

// create a mock that satisfies the lister-watcher interface
type mockNamespaceListerWatcherGetter struct {
	mock.Mock
}

func (lw *mockNamespaceListerWatcherGetter) List(options metav1.ListOptions) (runtime.Object, error) {
	args := lw.Called(options)
	return args.Get(0).(runtime.Object), args.Error(1)
}

func (lw *mockNamespaceListerWatcherGetter) Watch(options metav1.ListOptions) (watch.Interface, error) {
	args := lw.Called(options)
	return args.Get(0).(watch.Interface), args.Error(1)
}

func (lw *mockNamespaceListerWatcherGetter) Get(name string, options metav1.GetOptions) (*corev1.Namespace, error) {
	args := lw.Called(name, options)
	return args.Get(0).(*corev1.Namespace), args.Error(1)
}

func TestLookupOfNamespaceThroughReflector(t *testing.T) {
	nl := new(corev1.NamespaceList)
	err := json.Unmarshal([]byte(testNamespaceList), nl)
	require.NoError(t, err)
	fw := watch.NewFake()

	// when we call our mock lister-watcher return our canned namespace list
	lw := new(mockNamespaceListerWatcherGetter)
	// lo := metav1.ListOptions{}
	lw.On("List", mock.AnythingOfType("v1.ListOptions")).Return(nl, nil)
	lw.On("Watch", mock.AnythingOfType("v1.ListOptions")).Return(fw, nil)

	// start the store with reflector
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

	ns, err = mycache.LookupNamespace("kube-system")
	assert.NoError(t, err)
	assert.NotNil(t, ns)

	lw.AssertExpectations(t)
}

func TestCacheMissFallsBackToGetter(t *testing.T) {
	receivedNS := new(corev1.Namespace)
	err := json.Unmarshal([]byte(kubeSystem), receivedNS)
	require.NoError(t, err)

	// when we call our getter we are going to return our canned kube-system object
	lwg := new(mockNamespaceListerWatcherGetter)
	lwg.On("Get", "kube-system", mock.AnythingOfType("v1.GetOptions")).Return(receivedNS, nil)

	// create a store without a reflector...
	var ns *corev1.Namespace
	store := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})
	mycache := namespaceCache{
		store:     store,
		reflector: nil,
		getter:    lwg,
	}

	ns, err = mycache.LookupNamespace("kube-system")
	lwg.AssertExpectations(t)
	assert.NoError(t, err, "there should not be an error because the lookup should fall back to a successful api call")
	assert.NotNil(t, ns, "we get a namespace returned")
	assert.Equal(t, "kube-system", ns.Name, "we should have got the kube-system namespace back")
}

func TestLookupOfNonExistentNamespace(t *testing.T) {
	nl := new(corev1.NamespaceList)
	err := json.Unmarshal([]byte(testNamespaceList), nl)
	require.NoError(t, err)
	fw := watch.NewFake()

	// when we call our mock lister-watcher return our canned namespace list
	lwg := new(mockNamespaceListerWatcherGetter)
	// lo := metav1.ListOptions{}
	lwg.On("List", mock.Anything).Return(nl, nil)
	lwg.On("Watch", mock.Anything).Return(fw, nil)
	lwg.On("Get", "elvis", mock.AnythingOfType("v1.GetOptions")).Return(&corev1.Namespace{}, errors.New("elvis is not here"))

	// start the store with reflector
	var ns *corev1.Namespace
	store, reflector := cache.NewNamespaceKeyedIndexerAndReflector(lwg, ns, time.Duration(0))

	mycache := namespaceCache{
		store:     store,
		reflector: reflector,
		getter:    lwg,
	}
	stop := make(chan struct{})
	defer close(stop)
	mycache.StartNamespaceReflector(stop)

	// allow reflector to have started...
	time.Sleep(1 * time.Second)

	ns, err = mycache.LookupNamespace("elvis")
	lwg.AssertExpectations(t)
	assert.Error(t, err)
	assert.Errorf(t, err, "namespace elvis does not exist")
	assert.Equal(t, &corev1.Namespace{}, ns)
}
