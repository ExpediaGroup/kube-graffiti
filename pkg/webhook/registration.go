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

package webhook

import (
	"errors"
	"fmt"
	"strings"

	admissionreg "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

type Registration struct {
	Name              string   `mapstructure:"name" yaml:"name"`
	Targets           []Target `mapstructure:"targets" yaml:"targets"`
	NamespaceSelector string   `mapstructure:"namespace-selector" yaml:"namespace-selector,omitempty"`
	FailurePolicy     string   `mapstructure:"failure-policy" yaml:"failure-policy"`
}

// Target defines a kubernetes compatible admissionreg.Rule but with mapstructure tags so that we can
// unmarshal it as part of a Viper structured configuration.
type Target struct {
	APIGroups   []string `mapstructure:"api-groups" yaml:"api-groups"`
	APIVersions []string `mapstructure:"api-versions" yaml:"api-versions"`
	Resources   []string `mapstructure:"resources" yaml:"resources"`
}

// RegisterHook registers our webhook as MutatingWebhook with the kubernetes api.
func (s Server) RegisterHook(r Registration, clientset *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "RegisterHook")

	selector, err := metav1.ParseToLabelSelector(r.NamespaceSelector)
	if err != nil {
		mylog.Error().Err(err).Str("namespace-selector", r.NamespaceSelector).Msg("could not parse the namespace selector")
		return fmt.Errorf("could not parse the namespace selector: %v", err)
	}

	var failurePolicy admissionreg.FailurePolicyType
	failurePolicy = admissionreg.FailurePolicyType(strings.Title(r.FailurePolicy))
	if failurePolicy != admissionreg.Ignore && failurePolicy != admissionreg.Fail {
		mylog.Error().Err(err).Str("policy", strings.Title(r.FailurePolicy)).Msg("invalid admission registration failure policy type, must be 'Ignore' or 'Fail'")
		return fmt.Errorf("invalid admission registration failure policy type")
	}

	client := clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
	_, err = client.Get(r.Name, metav1.GetOptions{})
	if err == nil {
		if err := client.Delete(r.Name, nil); err != nil {
			mylog.Error().Err(err).Str("name", r.Name).Msg("failed to delete the webhook")
			return errors.New("failed to delete the webhook")
		}
	}

	var rules []admissionreg.RuleWithOperations
	for _, target := range r.Targets {
		rules = append(rules, admissionreg.RuleWithOperations{
			Operations: []admissionreg.OperationType{admissionreg.Create, admissionreg.Update},
			Rule: admissionreg.Rule{
				APIGroups:   target.APIGroups,
				APIVersions: target.APIVersions,
				Resources:   target.Resources,
			},
		})
	}

	path := pathFromName(r.Name)
	webhookConfig := &admissionreg.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Webhooks: []admissionreg.Webhook{
			{
				Name:              r.Name + "." + s.CompanyDomain,
				FailurePolicy:     &failurePolicy,
				NamespaceSelector: selector,
				Rules:             rules,
				ClientConfig: admissionreg.WebhookClientConfig{
					Service: &admissionreg.ServiceReference{
						Namespace: s.Namespace,
						Name:      s.Service,
						Path:      &path,
					},
					CABundle: s.CACert,
				},
			},
		},
	}
	if _, err := client.Create(webhookConfig); err != nil {
		mylog.Error().Err(err).Str("name", r.Name).Msg("webhook registration failed")
		return errors.New("webhook registration failed")
	}

	return nil
}
