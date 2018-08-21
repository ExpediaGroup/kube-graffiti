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
	Name              string
	Targets           []Target
	NamespaceSelector *metav1.LabelSelector
	FailurePolicy     *admissionreg.FailurePolicyType
}

type Target struct {
	APIGroups   []string
	APIVersions []string
	Resources   []string
}

func NewRegistration(name string, targets []Target, nselect, policy string) (Registration, error) {
	mylog := log.ComponentLogger(componentName, "NewRegistration")

	selector, err := metav1.ParseToLabelSelector(nselect)
	if err != nil {
		mylog.Error().Err(err).Str("namespace-selector", nselect).Msg("could not parse the namespace selector")
		return Registration{}, fmt.Errorf("could not parse the namespace selector: %v", err)
	}

	var failurePolicy admissionreg.FailurePolicyType
	failurePolicy = admissionreg.FailurePolicyType(strings.Title(policy))
	if failurePolicy != admissionreg.Ignore && failurePolicy != admissionreg.Fail {
		mylog.Error().Err(err).Str("policy", policy).Msg("invalid admission registration failure policy type, must be 'Ignore' or 'Fail'")
		return Registration{}, fmt.Errorf("invalid admission registration failure policy type")
	}

	return Registration{
		Name:              name,
		Targets:           targets,
		NamespaceSelector: selector,
		FailurePolicy:     &failurePolicy,
	}, nil
}

// RegisterWebhook registers our webhook as MutatingWebhook with the kubernetes api.
func (s Server) RegisterHook(path string, r Registration, clientset *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "RegisterHook")

	client := clientset.AdmissionregistrationV1beta1().MutatingWebhookConfigurations()
	_, err := client.Get(r.Name, metav1.GetOptions{})
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
				FailurePolicy:     r.FailurePolicy,
				NamespaceSelector: r.NamespaceSelector,
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
