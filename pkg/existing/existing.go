// existing handles the checking of Graffiti rules against already existing objects within Kubernetes.
package existing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"stash.hcom/run/kube-graffiti/pkg/config"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

const (
	componentName = "existing"
)

var (
	// package level discovery client to share when looking up available kubernetes objects/versions/resources
	discoveryClient     *discovery.DiscoveryClient
	discoveredAPIGroups = make(map[string]metav1.APIGroup)
	discoveredResources = make(map[string][]metav1.APIResource)
	dynamicClient       dynamic.Interface
)

// genericObject is used only for pulling out object metadata
type metaObject struct {
	Meta metav1.ObjectMeta `json:"metadata"`
}

// CheckExistingObjects kicks off the check of existing objects by discovering which apigroups, versions and resources
// that the kube apiserver knows about.  It then starts interating over each rule and each target within.
func CheckExistingObjects(rules []config.Rule, rest *rest.Config) error {
	mylog := log.ComponentLogger(componentName, "CheckExistingObjects")
	var err error

	// populate our api group/versions information map
	mylog.Debug().Msg("discovering kubernetes api groups and versions")
	discoveryClient, err = discovery.NewDiscoveryClientForConfig(rest)
	if err != nil {
		return fmt.Errorf("can't get a kubernetes discovery client: %v", err)
	}
	sg, err := discoveryClient.ServerGroups()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to look up kubernetes apigroups")
		return fmt.Errorf("failed to discover kubernetes api groups/versions: %v", err)
	}
	for _, group := range sg.Groups {
		discoveredAPIGroups[group.Name] = group
	}

	dynamicClient, err = dynamic.NewForConfig(rest)
	if err != nil {
		return fmt.Errorf("can't get a kubernetes dynamic client: %v", err)
	}

	// populate resources map
	mylog.Debug().Msg("discovering kubernetes resources")
	sliceOfResourceLists, err := discoveryClient.ServerResources()
	if err != nil {
		mylog.Error().Err(err).Msg("failed to look up kubernetes resources")
		return fmt.Errorf("failed to discover kubernetes api resources: %v", err)
	}
	for _, gv := range sliceOfResourceLists {
		discoveredResources[gv.GroupVersion] = gv.APIResources
	}

	mylog.Info().Msg("checking existing objects against graffiti rules")
	for _, rule := range rules {
		for _, target := range rule.Registration.Targets {
			checkTarget(&rule, target)
		}
	}

	return nil
}

// checkTarget handles and interates over APIGroup settings in order to further break these down into
func checkTarget(rule *config.Rule, target webhook.Target) {
	mylog := log.ComponentLogger(componentName, "checkTarget")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("target-apigroups", strings.Join(target.APIGroups, ",")).Str("target-versions", strings.Join(target.APIVersions, ",")).Str("target-resources", strings.Join(target.Resources, ",")).Logger()
	rlog.Info().Msg("evaluating target")

	if len(target.APIGroups) == 1 && target.APIGroups[0] == "*" {
		rlog.Debug().Msg("found target with APIGroup * wildcard")
		// check *all* groups
		for _, g := range discoveredAPIGroups {
			checkGroupVersion(rule, target, g.PreferredVersion)
		}
		return
	}

	// check each target apigroup if the target version wildcard '*' is used or the current preffered version is included in the list
	// of targetted versions.
	for _, g := range target.APIGroups {
		if g == "*" {
			rlog.Warn().Msg("you can not use wildcard * and a list of groups")
		} else {
			matched := false
			for _, vers := range target.APIVersions {
				if vers == discoveredAPIGroups[g].PreferredVersion.Version {
					matched = true
					checkGroupVersion(rule, target, discoveredAPIGroups[g].PreferredVersion)
				}
			}
			if !matched {
				rlog.Warn().Str("group", g).Str("preffered-version", discoveredAPIGroups[g].PreferredVersion.Version).Msg("targetted APIVersions do not match either wildcard or the preferred api version - therefore we will not use this rule to update existing objects for this group")
			}
		}
	}
}

func checkGroupVersion(rule *config.Rule, target webhook.Target, version metav1.GroupVersionForDiscovery) {
	mylog := log.ComponentLogger(componentName, "checkGroupVersion")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group-version", version.GroupVersion).Str("version", version.Version).Logger()
	rlog.Debug().Msg("evaluating group version")

	if len(target.Resources) == 1 && target.Resources[0] == "*" {
		rlog.Debug().Msg("found target with Resources * wildcard")
		// check *all* known resources
		for _, r := range discoveredResources[version.GroupVersion] {
			checkResource(rule, version.GroupVersion, r)
		}
		return
	}

	// check each target resources
	for _, r := range target.Resources {
		if r == "*" {
			rlog.Error().Msg("you can not have * and a list of versions")
			return
		}
		// resources can be specified as object/sub-object
		x, s := splitSlashedResourceString(r)
		rlog.Debug().Str("resource", x).Str("sub", s).Msg("looking at resource")
		// find APIResourec object for this resource
		var found = false
		for _, resource := range discoveredResources[version.GroupVersion] {
			if resource.Name == x {
				found = true
				checkResource(rule, version.GroupVersion, resource)
			}
		}
		if !found {
			rlog.Debug().Str("resource", x).Msg("resource did not match any discovered resources")
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

func checkResource(rule *config.Rule, gv string, resource metav1.APIResource) {
	mylog := log.ComponentLogger(componentName, "checkResource")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group-version", gv).Str("resource", resource.Name).Logger()
	rlog.Info().Msg("looking at resources of type")

	g, v := splitGroupVersionString(gv)
	// get a dynamic client resource interface
	grv := schema.GroupVersionResource{
		Group:    g,
		Version:  v,
		Resource: resource.Name,
	}
	spew.Dump(grv)
	ri := dynamicClient.Resource(grv)

	list, err := ri.List(metav1.ListOptions{})
	if err != nil {
		rlog.Error().Err(err).Msg("failed to list resources")
	}
	var mo metaObject
	if list == nil {
		rlog.Info().Msg("no resources found")
		return
	}
	for _, item := range list.Items {
		mo = metaObject{}
		// Unstructured has a single field, Object, which is a map
		// populated by K8s containing all the info needed for a Helm Deploy
		// Simply convert the map into bytes
		b, err := json.Marshal(item.Object)
		if err != nil {
			rlog.Error().Err(err).Msg("failed to marshall unstructured object")
			continue
		}
		if err := json.Unmarshal(b, &mo); err != nil {
			rlog.Error().Err(err).Msg("failed to unmarshall json into a metadata object")
		}
		spew.Dump(mo)
	}
}
