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
	Name              string   `mapstructure:"name"`
	Targets           []Target `mapstructure:"targets"`
	NamespaceSelector string   `mapstructure:"namespace-selector"`
	FailurePolicy     string   `mapstructure:"failure-policy"`
}

// Target defines a kubernetes compatible admissionreg.Rule but with mapstructure tags so that we can
// unmarshal it as part of a Viper structured configuration.
type Target struct {
	APIGroups   []string `mapstructure:"api-groups"`
	APIVersions []string `mapstructure:"api-versions"`
	Resources   []string `mapstructure:"resources"`
}

// RegisterHook registers our webhook as MutatingWebhook with the kubernetes api.
func (s Server) RegisterHook(path string, r Registration, clientset *kubernetes.Clientset) error {
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
