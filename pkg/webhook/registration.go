package webhook

import (
	"errors"

	admissionreg "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"stash.hcom/run/kube-graffiti/pkg/log"
)

type Registration struct {
	Name              string
	APIGroups         []string
	APIVersions       []string
	Resources         []string
	NamespaceSelector *metav1.LabelSelector
	FailurePolicy     *admissionreg.FailurePolicyType
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
	webhookConfig := &admissionreg.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Webhooks: []admissionreg.Webhook{
			{
				Name:          r.Name + "." + s.CompanyDomain,
				FailurePolicy: r.FailurePolicy,
				Rules: []admissionreg.RuleWithOperations{{
					Operations: []admissionreg.OperationType{admissionreg.Create, admissionreg.Update},
					Rule: admissionreg.Rule{
						APIGroups:   r.APIGroups,
						APIVersions: r.APIVersions,
						Resources:   r.Resources,
					},
				}},
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
