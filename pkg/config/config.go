package config

import (
	"errors"

	"github.com/rs/zerolog"
)

// All of our configuration modelled with mapstructure tags so that we can use viper to properly parse and load it for us.

type Configuration struct {
	Server Server `mapstructure:"server"`
	Rules  []Rule `mapstructure:"rules"`
}

type Server struct {
	LogLevel      string `mapstructure:"log-level"`
	WebhookPort   int    `mapstructure:"port"`
	MetricsPort   int    `mapstructure:"metrics-port"`
	CompanyDomain string `mapstructure:"company-domain"`
	Namespace     string `mapstructure:"namespace"`
	Service       string `mapstructure:"service"`
	CACert        []byte `mapstructure:"ca-cert"`
	CheckExisting bool   `mapstructure:"check-existing"`
}

type Rule struct {
	Registration Registration `mapstructure:"registration"`
	Matcher      Matcher      `mapstructure:"matcher"`
	Additions    Additions    `mapstructure:"additions"`
}

type Registration struct {
	Name              string                `mapstructure:"name"`
	Targets           []RegistrationTargets `mapstructure:"targets"`
	NamespaceSelector string                `mapstructure:"namespace-selector"`
	FailurePolicy     string                `mapstructure:"failure-policy"`
}

type RegistrationTargets struct {
	APIGroups   []string `mapstructure:"api-groups"`
	APIVersions []string `mapstructure:"api-versions"`
	Resources   []string `mapstructure:"resources"`
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

var (
	// logLevels defines a map of valid log level strings to their corresponding zerolog types.
	logLevels = map[string]zerolog.Level{
		"panic": zerolog.DebugLevel,
		"fatal": zerolog.FatalLevel,
		"error": zerolog.ErrorLevel,
		"warn":  zerolog.WarnLevel,
		"info":  zerolog.InfoLevel,
		"debug": zerolog.DebugLevel,
	}
)

func LoadConfigFromViper() *Configuration {
	return &configuration{
		config:          cfgFile,
		logLevel:        getParam("loglevel", "info").(string),
		metricsPort:     getParam("metrics-port", defaultMetricsPort).(int),
		webhookPort:     getParam("webhook-port", defaultWebhookPort).(int),
		webhookKeyfile:  getParam("webhook-keyfile", "ERROR").(string),
		webhookCertfile: getParam("webhook-certfile", "ERROR").(string),
		webhookCAfile:   getParam("webhook-cafile", "ERROR").(string),
		checkExisting:   getParam("check-existing", false).(bool),
	}
}

// validateRootCmdArgs manages the validation of all arguments passed to the program
func (c *Configuration) ValidateConfig() error {
	if err := c.validateWebhookArgs(); err != nil {
		return err
	}
	return c.validateLogArgs()
}

// validateLogArgs check that a requested log-level is defined/allowed.
func (c *Configuration) validateLogArgs() error {
	// check the configured log level is valid.
	if _, ok := logLevels[c.logLevel]; !ok {
		return errors.New(c.logLevel + "is not a valid log-level")
	}
	return nil
}

func (c *Configuration) validateWebhookArgs() error {
	if c.webhookCAfile == "ERROR" || c.webhookCertfile == "ERROR" || c.webhookKeyfile == "ERROR" {
		return errors.New("you must provide values for webhook-cafile, webhook-certfile and webhook-keyfile")
	}
	return nil
}
