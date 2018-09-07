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

var (
	store     cache.Indexer
	reflector *cache.Reflector
)

// Implement a cache.ListerWatcher for namespace objects.  We need to implement this interface
// in order to create an indexer(store) and reflector.
type namespaceListerWatcher struct {
	ns clientcorev1.NamespaceInterface
}

func (lw namespaceListerWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	return lw.ns.List(metav1.ListOptions{})
}

func (lw namespaceListerWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lw.ns.Watch(metav1.ListOptions{})
}

func newNamespaceListerWatcher(rest *rest.Config) (namespaceListerWatcher, error) {
	coreClient, err := clientcorev1.NewForConfig(rest)
	if err != nil {
		return namespaceListerWatcher{}, err
	}
	return namespaceListerWatcher{ns: coreClient.Namespaces()}, nil
}

// initNamespaceLookupCache starts a client-go cache and reflector which watches and updates namespaces when they change.
// We pass a stop channel to signal the reflector to shutdown when we no longer need it.
func initNamespaceCache(rest *rest.Config) error {
	mylog := log.ComponentLogger(componentName, "cachingLookupNamespace")
	mylog.Info().Msg("starting the namespace cache")

	lw, err := newNamespaceListerWatcher(rest)
	if err != nil {
		mylog.Error().Err(err).Msg("failed to create the namespace lister-watcher")
		return fmt.Errorf("could not create namespace listerwatcher: %v", err)
	}
	var ns *corev1.Namespace
	store, reflector = cache.NewNamespaceKeyedIndexerAndReflector(lw, ns, time.Duration(refreshPeriodSeconds*time.Second))
	mylog.Debug().Msg("starting the namespace cache reflector")

	return nil
}

func startNamespaceReflector(stop <-chan struct{}) {
	go reflector.Run(stop)
}

func lookupNamespace(name string) (*corev1.Namespace, error) {
	mylog := log.ComponentLogger(componentName, "cachingLookupNamespace").With().Str("namespace", name).Logger()
	mylog.Debug().Msg("looking up namespace")

	ns, exists, err := store.GetByKey(name)
	if err != nil {
		mylog.Error().Err(err).Msg("error looking up namespace in cache")
		return &corev1.Namespace{}, fmt.Errorf("error looking up namespace from store: %v", err)
	}
	if !exists {
		mylog.Error().Msg("namespace does not exist")
		return &corev1.Namespace{}, fmt.Errorf("namespace %s does not exist", name)
	}

	return ns.(*corev1.Namespace), nil
}

func removeNamespaceCache() {
	store = nil
	reflector = nil
}
