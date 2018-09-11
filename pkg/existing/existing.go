// Package existing handles the checking of Graffiti rules against already existing objects within Kubernetes.
package existing

import (
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"stash.hcom/run/kube-graffiti/pkg/config"
	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

const (
	componentName = "existing"
	// itemLimit is used to constrain the number of items returned in a kubernetes List call.
	itemLimit = 100
)

var (
	// package level discovery client to share when looking up available kubernetes objects/versions/resources
	discoveryClient     apiDiscoverer
	discoveredAPIGroups = make(map[string]metav1.APIGroup)
	discoveredResources = make(map[string][]metav1.APIResource)
	dynamicClient       mockableDynamicInterface
	nsCache             namespaceCache
)

// interface used to mock out the client-go discovery client for testing...
type apiDiscoverer interface {
	ServerGroups() (apiGroupList *metav1.APIGroupList, err error)
	ServerResources() ([]*metav1.APIResourceList, error)
}

// interfaces used to mock out a cut down set of client-go dynamic interfaces...
type mockableDynamicInterface interface {
	Resource(resource schema.GroupVersionResource) mockableNamespaceableResourceInterface
}

type mockableNamespaceableResourceInterface interface {
	Namespace(string) mockableResourceInterface
	mockableResourceInterface
}

type mockableResourceInterface interface {
	List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*unstructured.Unstructured, error)
}

// concrete types that implement the above interfaces by encapsulating the real client-go types within them...
type realKubeDynamicInterface struct {
	client dynamic.Interface
}

type realKubeNamespaceableResourceInterface struct {
	nri dynamic.NamespaceableResourceInterface
}

type realKubeResourceInterface struct {
	ri dynamic.ResourceInterface
}

func (r realKubeDynamicInterface) Resource(resource schema.GroupVersionResource) mockableNamespaceableResourceInterface {
	nri := r.client.Resource(resource)
	return realKubeNamespaceableResourceInterface{
		nri: nri,
	}
}

func (nri realKubeNamespaceableResourceInterface) Namespace(ns string) mockableResourceInterface {
	ri := nri.nri.Namespace(ns)
	return realKubeResourceInterface{ri: ri}
}

func (nri realKubeNamespaceableResourceInterface) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nri.nri.List(opts)
}

func (nri realKubeNamespaceableResourceInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*unstructured.Unstructured, error) {
	return nri.nri.Patch(name, pt, data, subresources...)
}

func (ri realKubeResourceInterface) List(opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return ri.ri.List(opts)
}

func (ri realKubeResourceInterface) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*unstructured.Unstructured, error) {
	return ri.ri.Patch(name, pt, data, subresources...)
}

// InitKubeClients sets up the package for working with kubernetes api and discovers
// and caches known api groups/versions and resource types
func InitKubeClients(rest *rest.Config) error {
	mylog := log.ComponentLogger(componentName, "InitKubeClients")
	mylog.Debug().Msg("setting up kubernetes discovery and dynamic clients")
	var err error

	discoveryClient, err = discovery.NewDiscoveryClientForConfig(rest)
	if err != nil {
		return fmt.Errorf("can't get a kubernetes discovery client: %v", err)
	}
	dc, err := dynamic.NewForConfig(rest)
	if err != nil {
		return fmt.Errorf("can't get a kubernetes dynamic client: %v", err)
	}
	dynamicClient = realKubeDynamicInterface{
		client: dc,
	}
	nsCache, err = NewNamespaceCache(rest)
	if err != nil {
		return fmt.Errorf("could not create the namespace cache: %v", err)
	}

	return discoverAPIsAndResources()
}

// discoverAPIsAndResources loads information about known apis and resources
// into our cache so we can use them without making lots of calls to kubernetes
func discoverAPIsAndResources() error {
	mylog := log.ComponentLogger(componentName, "discoverAPIsAndResources")

	mylog.Debug().Msg("discovering kubernetes api groups")
	sg, err := discoveryClient.ServerGroups()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to look up kubernetes apigroups")
		return fmt.Errorf("failed to discover kubernetes api groups/versions: %v", err)
	}
	for _, group := range sg.Groups {
		discoveredAPIGroups[group.Name] = group
	}

	mylog.Debug().Msg("discovering kubernetes resources")
	sliceOfResourceLists, err := discoveryClient.ServerResources()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to look up kubernetes resources")
		return fmt.Errorf("failed to discover kubernetes api resources: %v", err)
	}
	for _, gv := range sliceOfResourceLists {
		discoveredResources[gv.GroupVersion] = gv.APIResources
	}

	return nil
}

// CheckRulesAgainstExistingState interates over the graffiti rules and targets, checking each one.
func CheckRulesAgainstExistingState(rules []config.Rule) {
	mylog := log.ComponentLogger(componentName, "CheckRulesAgainstExistingState")

	// start the namespace cache reflector to populate it with values
	stop := make(chan struct{})
	defer close(stop)
	nsCache.StartNamespaceReflector(stop)
	mylog.Info().Msg("checking existing objects against graffiti rules")
	for _, rule := range rules {
		for _, target := range rule.Registration.Targets {
			checkTarget(&rule, target)
		}
	}
}

// checkTarget starts evaluating a target by getting a list of APIGroups which are listed.
// If the target APIGroups is ["*"] then we will check through *all* discoverd apigroups.
func checkTarget(rule *config.Rule, target webhook.Target) {
	mylog := log.ComponentLogger(componentName, "checkTarget")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("target-apigroups", strings.Join(target.APIGroups, ",")).Str("target-versions", strings.Join(target.APIVersions, ",")).Str("target-resources", strings.Join(target.Resources, ",")).Logger()
	rlog.Info().Msg("evaluating target")

	// handle wildcard '*'
	var targetGroups []string
	if len(target.APIGroups) == 1 && target.APIGroups[0] == "*" {
		rlog.Debug().Msg("found target with APIGroup * wildcard")
		// check *all* discovered groups
		for _, g := range discoveredAPIGroups {
			targetGroups = append(targetGroups, g.Name)
		}
	} else {
		targetGroups = target.APIGroups
	}

	// check each group/version is targetted and check
	for _, g := range targetGroups {
		if isTargetted(discoveredAPIGroups[g].PreferredVersion.Version, target.APIVersions) {
			checkGroupVersion(rule, target, discoveredAPIGroups[g].PreferredVersion)
		} else {
			rlog.Warn().Str("group", g).Str("preffered-version", discoveredAPIGroups[g].PreferredVersion.Version).Msg("targetted APIVersions do not match either wildcard or the preferred api version - therefore we will not use this rule to update existing objects for this group")
		}
	}
}

// isTargetted checks that an element is present in a target list or matches a wildcard '*'
func isTargetted(element string, targets []string) bool {
	for _, target := range targets {
		if target == "*" || element == target {
			return true
		}
	}
	return false
}

// checkGroupVersion checks all the resources in an group/version that are targetted.
// If the target is ["*"] then all resources are checked, otherwise each discovered resource is
// checked against the target list.
func checkGroupVersion(rule *config.Rule, target webhook.Target, gv metav1.GroupVersionForDiscovery) {
	mylog := log.ComponentLogger(componentName, "checkGroupVersion")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group-version", gv.GroupVersion).Str("version", gv.Version).Logger()
	rlog.Debug().Msg("evaluating group version")

	if len(target.Resources) == 1 && (target.Resources[0] == "*" || target.Resources[0] == "*/*") {
		rlog.Debug().Msg("found target with Resources * wildcard")
		for _, r := range discoveredResources[gv.GroupVersion] {
			checkResourceType(rule, gv.GroupVersion, r)
		}
		return
	}

	// create a list of resources without any subtypes
	var resourceTargets []string
	for _, r := range target.Resources {
		x, _ := splitSlashedResourceString(r)
		if x == "*" {
			rlog.Error().Msg("you shouldn't have a wildcard '*' in a list of resources, ignoring")
			continue
		}
		resourceTargets = append(resourceTargets, x)
	}

	// for each resource in the group/version check if it is targetted list and check
	for _, resource := range discoveredResources[gv.GroupVersion] {
		if isTargetted(resource.Name, resourceTargets) {
			checkResourceType(rule, gv.GroupVersion, resource)
		} else {
			rlog.Debug().Str("resource", resource.Name).Msg("resource is not targetted")
		}
	}
}

func splitSlashedResourceString(s string) (first, second string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func splitGroupVersionString(s string) (group, version string) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}

// checkResourceType checks all of the resources of particular group/version type.
// It lists the resources in batches of itemLimit in order to preserve memory when there are
// many kubernetes objects of the type in the cluster.
func checkResourceType(rule *config.Rule, gv string, resource metav1.APIResource) {
	mylog := log.ComponentLogger(componentName, "checkResourceType")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group-version", gv).Str("resource", resource.Name).Logger()
	rlog.Info().Msg("looking at resources of type")

	g, v := splitGroupVersionString(gv)
	// get a dynamic client resource interface
	grv := schema.GroupVersionResource{
		Group:    g,
		Version:  v,
		Resource: resource.Name,
	}
	ri := dynamicClient.Resource(grv)

	// get first list of items up to our limit
	list, err := ri.List(metav1.ListOptions{Limit: itemLimit})
	if err != nil {
		rlog.Error().Err(err).Msg("failed to list resources")
		return
	}
	if list == nil {
		rlog.Info().Msg("no resources found")
		return
	}
	rlog.Debug().Int("number-resources", len(list.Items)).Msg("processing batch of resources")
	for _, item := range list.Items {
		checkObject(rule, gv, resource.Name, item)
	}

	// if we only got a partial list we need to continue until we have seen them all
	meta := list.Object["metadata"].(map[string]interface{})
	for cont, ok := meta["continue"]; ok; {
		list, err = ri.List(metav1.ListOptions{Limit: itemLimit, Continue: cont.(string)})
		if err != nil {
			rlog.Error().Err(err).Msg("failed to list resources")
			return
		}
		if list == nil {
			rlog.Info().Msg("no resources found")
			return
		}
		rlog.Debug().Int("number-resources", len(list.Items)).Msg("processing batch of resources")
		for _, item := range list.Items {
			checkObject(rule, gv, resource.Name, item)
		}
		meta = list.Object["metadata"].(map[string]interface{})
		cont, ok = meta["continue"]
	}
}

// checkObject takes a single kubernete object and decides whether to graffiti it or not.
func checkObject(rule *config.Rule, gv, resource string, object unstructured.Unstructured) {
	mylog := log.ComponentLogger(componentName, "checkObject")
	kind := object.GetKind()
	name := object.GetName()
	namespace := object.GetNamespace()
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group-version", gv).Str("kind", kind).Str("name", name).Str("namespace", namespace).Logger()
	rlog.Info().Msg("checking object")

	// match against optional rule namespace selector
	if rule.Registration.NamespaceSelector != "" {
		match, err := matchNamespaceSelector(object.Object, rule.Registration.NamespaceSelector, nsCache)
		if err != nil {
			rlog.Error().Err(err).Msg("error checking object against namespace selector")
		}
		if !match {
			rlog.Info().Msg("object does not match namespace selector")
			return
		}
	}

	rlog.Info().Msg("applying graffiti mutate rule to existing object")
	gr := graffiti.Rule{
		Name:      rule.Registration.Name,
		Matchers:  rule.Matchers,
		Additions: rule.Additions,
	}
	raw, err := json.Marshal(object.Object)
	if err != nil {
		rlog.Error().Err(err).Msg("could not marshal object")
		return
	}
	// call the graffiti package to evaluation the graffiti rule...
	patch, err := gr.Mutate(raw)
	if err != nil {
		rlog.Error().Err(err).Msg("could not mutate object")
		return
	}
	if patch == nil {
		rlog.Info().Msg("mutate did not create a patch")
		return
	}

	rlog.Debug().Str("patch", string(patch)).Msg("mutate produced a patch")
	g, v := splitGroupVersionString(gv)
	grv := schema.GroupVersionResource{
		Group:    g,
		Version:  v,
		Resource: resource,
	}
	ri := dynamicClient.Resource(grv)
	if namespace == "" {
		rlog.Debug().Msg("patching cluster level object")
		_, err = ri.Patch(name, types.JSONPatchType, patch)
	} else {
		rlog.Debug().Msg("patching namespaced object")
		nri := ri.Namespace(namespace)
		_, err = nri.Patch(name, types.JSONPatchType, patch)
	}
	if err != nil {
		rlog.Error().Err(err).Msg("failed to patch object")
		return
	}
	rlog.Info().Str("patch", string(patch)).Msg("successfully patched object")
}
