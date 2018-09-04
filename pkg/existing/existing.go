// existing handles the checking of Graffiti rules against already existing objects within Kubernetes.
package existing

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
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
)

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

func checkTarget(rule *config.Rule, target webhook.Target) {
	mylog := log.ComponentLogger(componentName, "checkTarget")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("target-apigroups", strings.Join(target.APIGroups, ",")).Str("target-versions", strings.Join(target.APIVersions, ",")).Str("target-resources", strings.Join(target.Resources, ",")).Logger()
	rlog.Info().Msg("evaluating target")

	if len(target.APIGroups) == 1 && target.APIGroups[0] == "*" {
		rlog.Debug().Msg("found target with APIGroup * wildcard")
		// check *all* groups
		for _, g := range discoveredAPIGroups {
			checkAPIGroup(rule, target, g)
		}
		return
	}

	// check each supplied group
	for _, g := range target.APIGroups {
		if g == "*" {
			rlog.Warn().Msg("you can not use wildcard * and a list of groups")
		} else {
			checkAPIGroup(rule, target, discoveredAPIGroups[g])
		}
	}
}

func checkAPIGroup(rule *config.Rule, target webhook.Target, group metav1.APIGroup) {
	mylog := log.ComponentLogger(componentName, "checkAPIGroup")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("apigroup", group.Name).Logger()
	rlog.Debug().Msg("evaluating api group")

	if len(target.APIVersions) == 1 && target.APIVersions[0] == "*" {
		rlog.Debug().Msg("found target with APIVersion * wildcard")
		// check *all* known versions
		for _, v := range group.Versions {
			checkGroupVersion(rule, target, v)
		}
		return
	}

	// check each supplied version
	for _, v := range target.APIVersions {
		if v == "*" {
			rlog.Error().Msg("you can not have * and a list of versions")
			return
		}
		checkGroupVersion(rule, target, metav1.GroupVersionForDiscovery{
			GroupVersion: group.Name + "/" + v,
			Version:      v,
		})
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
			checkResource(rule, r)
		}
		return
	}

	// check each supplied version
	for _, r := range target.Resources {
		if r == "*" {
			rlog.Error().Msg("you can not have * and a list of versions")
			return
		}
		x, s := splitResource(r)
		rlog.Debug().Str("resource", x).Str("sub", s).Msg("looking at resource")
		// find APIResourec object for this resource
		var found = false
		for _, resource := range discoveredResources[version.Version] {
			if resource.Name == x {
				found = true
				checkResource(rule, resource)
			}
		}
		if !found {
			rlog.Error().Str("resource", x).Msg("resource did not match any discovered resources")
		}
	}
}

func splitResource(resSub string) (res, sub string) {
	parts := strings.SplitN(resSub, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func checkResource(rule *config.Rule, resource metav1.APIResource) {
	mylog := log.ComponentLogger(componentName, "checkResource")
	rlog := mylog.With().Str("rule", rule.Registration.Name).Str("group", resource.Group).Str("version", resource.Version).Str("resource", resource.Name).Logger()
	rlog.Debug().Msg("evaluating resource")
}
