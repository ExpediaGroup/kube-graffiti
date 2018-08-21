package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

const (
	componentName = "config"
	// DefaultLogLevel - the zero logging level set for whole program
	DefaultLogLevel = "info"
)

// All of our configuration modelled with mapstructure tags so that we can use viper to properly parse and load it for us.

type Configuration struct {
	Server Server `mapstructure:"server"`
	Rules  []Rule `mapstructure:"rules"`
}

type Server struct {
	LogLevel       string `mapstructure:"log-level"`
	WebhookPort    int    `mapstructure:"port"`
	MetricsPort    int    `mapstructure:"metrics-port"`
	HealthPath     string `mapstructure:"health-path"`
	CompanyDomain  string `mapstructure:"company-domain"`
	Namespace      string `mapstructure:"namespace"`
	Service        string `mapstructure:"service"`
	CACertPath     string `mapstructure:"ca-cert-path"`
	ServerCertPath string `mapstructure:"cert-path"`
	ServerKeyPath  string `mapstructure:"key-path"`
	CheckExisting  bool   `mapstructure:"check-existing"`
}

type Rule struct {
	Registration Registration `mapstructure:"registration"`
	Matcher      Matcher      `mapstructure:"matcher"`
	Additions    Additions    `mapstructure:"additions"`
}

type Registration struct {
	Name              string           `mapstructure:"name"`
	Targets           []webhook.Target `mapstructure:"targets"`
	NamespaceSelector string           `mapstructure:"namespace-selector"`
	FailurePolicy     string           `mapstructure:"failure-policy"`
}

type Matcher struct {
	LabelSelectors  []string `mapstructure:"label-selectors"`
	FieldSelectors  []string `mapstructure:"field-selectors"`
	BooleanOperator string   `mapstructure:"boolean-operator"`
}

type Additions struct {
	Annotations map[string]string `mapstructure:"annotations"`
	Labels      map[string]string `mapstructure:"labels"`
}

func SetDefaults() {
	viper.SetDefault("server.metrics-port", 8080)
	viper.SetDefault("server.port", 8443)
	viper.SetDefault("server.log-level", DefaultLogLevel)
	viper.SetDefault("server.health-path", "/healthz")
	viper.SetDefault("server.company-domain", "acme.com")
	viper.SetDefault("server.check-existing", false)
	viper.SetDefault("server.ca-cert-path", "/ca.pem")
}

func ReadConfiguration() (*Configuration, error) {
	var c Configuration

	if err := viper.Unmarshal(&c); err != nil {
		return &c, fmt.Errorf("Failed to marshal configuration: %v", err)
	}
	return &c, nil
}

// ValidateConfig is responsible for throwing errors when the configuration is bad.
func (c *Configuration) ValidateConfig() error {
	if err := c.validateWebhookArgs(); err != nil {
		return err
	}
	return c.validateLogArgs()
}

// validateLogArgs check that a requested log-level is defined/allowed.
func (c *Configuration) validateLogArgs() error {
	// check the configured log level is valid.
	if _, ok := log.LogLevels[c.Server.LogLevel]; !ok {
		return errors.New(c.Server.LogLevel + "is not a valid log-level")
	}
	return nil
}

func (c *Configuration) validateWebhookArgs() error {
	mylog := log.ComponentLogger(componentName, "validateWebhookArgs")
	for _, p := range []string{"server.ca-cert-path", "server.cert-path", "server.key-path"} {
		if !viper.IsSet(p) {
			mylog.Error().Str("parameter", p).Msg("missing required parameter value")
			return fmt.Errorf("missing required parameter")
		}
	}
	return nil
}
