package existing

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"stash.hcom/run/kube-graffiti/pkg/log"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	// refreshPeriodSeconds - an interval in which to refresh all the objects in the cache
	// the objects are updated inbetween refreshes by the reflector watching for changes
	refreshPeriodSeconds = 60
)

type namespaceCache struct {
	store     cache.Indexer
	reflector *cache.Reflector
	getter    namespaceGetter
}

// namespaceListerWatcherGetter implements the cache.ListerWatcher interface.
// This is used to create a cache that is able to list and cache namespaces.
// It also implements the namespaceGetter interface
type namespaceListerWatcherGetter struct {
	ns clientcorev1.NamespaceInterface
}

func (lwg namespaceListerWatcherGetter) List(options metav1.ListOptions) (runtime.Object, error) {
	return lwg.ns.List(metav1.ListOptions{})
}

func (lwg namespaceListerWatcherGetter) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lwg.ns.Watch(metav1.ListOptions{})
}

func (lwg namespaceListerWatcherGetter) Get(name string, options metav1.GetOptions) (*corev1.Namespace, error) {
	return lwg.ns.Get(name, options)
}

func newNamespaceListerWatcherGetter(rest *rest.Config) (namespaceListerWatcherGetter, error) {
	coreClient, err := clientcorev1.NewForConfig(rest)
	if err != nil {
		return namespaceListerWatcherGetter{}, err
	}
	return namespaceListerWatcherGetter{ns: coreClient.Namespaces()}, nil
}

// namespaceGetter allows us to abstract the operation of getting namespaces
type namespaceGetter interface {
	Get(name string, options metav1.GetOptions) (*corev1.Namespace, error)
}

// NewNamespaceCache creates client-go cache.Store and Reflector which watches and updates namespaces when they change.
func NewNamespaceCache(rest *rest.Config) (namespaceCache, error) {
	mylog := log.ComponentLogger(componentName, "NewNamespaceCache")
	mylog.Info().Msg("starting the namespace cache")

	lwg, err := newNamespaceListerWatcherGetter(rest)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to create the namespace lister-watcher")
		return namespaceCache{}, fmt.Errorf("could not create namespace listerwatcher: %v", err)
	}
	var ns *corev1.Namespace
	store, reflector := cache.NewNamespaceKeyedIndexerAndReflector(lwg, ns, time.Duration(refreshPeriodSeconds*time.Second))

	return namespaceCache{
		store:     store,
		reflector: reflector,
		getter:    lwg,
	}, nil
}

// StartNamespaceReflector starts the reflector for the Namespace cache.  The reflector is resposible for watching namespaces
// and updating the cached namespaces when they change in kubernetes.  It is started separately from the store so that we can
// pass it in a stop channel to instruct it to shutdown once it is no longer needed.
func (c namespaceCache) StartNamespaceReflector(stop <-chan struct{}) {
	go c.reflector.Run(stop)
}

func (c namespaceCache) LookupNamespace(name string) (*corev1.Namespace, error) {
	mylog := log.ComponentLogger(componentName, "cachingLookupNamespace").With().Str("namespace", name).Logger()
	mylog.Debug().Msg("looking up namespace")

	if c.store == nil {
		return &corev1.Namespace{}, fmt.Errorf("the store is nil - not initialized")
	}
	ns, exists, err := c.store.GetByKey(name)
	if err != nil {
		mylog.Error().Err(err).Msg("error looking up namespace in cache")
		return &corev1.Namespace{}, fmt.Errorf("error looking up namespace from store: %v", err)
	}
	if !exists {
		mylog.Warn().Msg("namespace not found in cache, falling back to api call")
		return c.getter.Get(name, metav1.GetOptions{})
	}

	return ns.(*corev1.Namespace), nil
}
