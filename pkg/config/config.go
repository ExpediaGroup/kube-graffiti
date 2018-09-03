package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/healthcheck"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

const (
	componentName = "config"
	// DefaultLogLevel - the zero logging level set for whole program
	DefaultLogLevel   = "info"
	defaultConfigPath = "/config"
)

// All of our configuration modelled with mapstructure tags so that we can use viper to properly parse and load it for us.

// Configuration models the structre of our configuration values loaded through viper.
type Configuration struct {
	_             string                    `mapstructure:"config"`
	LogLevel      string                    `mapstructure:"log-level"`
	CheckExisting bool                      `mapstructure:"check-existing"`
	HealthChecker healthcheck.HealthChecker `mapstructure:"health-checker"`
	Server        Server                    `mapstructure:"server"`
	Rules         []Rule                    `mapstructure:"rules"`
}

// Server contains all the settings for the webhook https server and access from the kubernetes api.
type Server struct {
	WebhookPort    int    `mapstructure:"port"`
	CompanyDomain  string `mapstructure:"company-domain"`
	Namespace      string `mapstructure:"namespace"`
	Service        string `mapstructure:"service"`
	CACertPath     string `mapstructure:"ca-cert-path"`
	ServerCertPath string `mapstructure:"cert-path"`
	ServerKeyPath  string `mapstructure:"key-path"`
}

// Rule models a single graffiti rule with three sections for managing registration, matching and the payload to graffiti on the object.
type Rule struct {
	Registration webhook.Registration `mapstructure:"registration"`
	Matchers     graffiti.Matchers    `mapstructure:"matchers"`
	Additions    graffiti.Additions   `mapstructure:"additions"`
}

// LoadConfig is reponsible for loading the viper configuration file.
func LoadConfig(file string) (*Configuration, error) {
	setDefaults()

	// Don't forget to read config either from cfgFile or from home directory!
	if file != "" {
		// Use config file from the flag.
		viper.SetConfigFile(file)
	} else {
		viper.SetConfigName(defaultConfigPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}

	return unmarshalFromViperStrict()
}

func setDefaults() {
	viper.SetDefault("log-level", DefaultLogLevel)
	viper.SetDefault("check-existing", false)
	viper.SetDefault("server.port", 8443)
	viper.SetDefault("health-checker.port", 8080)
	viper.SetDefault("health-checker.path", "/healthz")
	viper.SetDefault("server.company-domain", "acme.com")
	viper.SetDefault("server.ca-cert-path", "/ca-cert")
	viper.SetDefault("server.cert-path", "/server-cert")
	viper.SetDefault("server.cert-path", "/server-key")
}

func unmarshalFromViperStrict() (*Configuration, error) {
	var c Configuration
	// add in a special decoder so that viper can unmarshal boolean operator values such as AND, OR and XOR
	// and enable mapstructure's ErrorUnused checking so we can catch bad configuration keys in the source.
	decoderHookFunc := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		graffiti.StringToBooleanOperatorFunc(),
	)
	opts := decodeHookWithErrorUnused(decoderHookFunc)

	if err := viper.Unmarshal(&c, opts); err != nil {
		return &c, fmt.Errorf("failed to unmarshal configuration: %v", err)
	}
	return &c, nil
}

// Our own implementation of Viper's DecodeHook so that we can set ErrorUnused to true
func decodeHookWithErrorUnused(hook mapstructure.DecodeHookFunc) viper.DecoderConfigOption {
	return func(c *mapstructure.DecoderConfig) {
		c.DecodeHook = hook
		c.ErrorUnused = true
	}
}

// ValidateConfig is responsible for throwing errors when the configuration is bad.
func (c *Configuration) ValidateConfig() error {
	mylog := log.ComponentLogger(componentName, "ValidateConfig")
	mylog.Debug().Msg("validating configuration")

	if err := c.validateLogArgs(); err != nil {
		return err
	}
	if err := c.validateWebhookArgs(); err != nil {
		return err
	}
	if err := c.validateRules(); err != nil {
		return err
	}

	return nil
}

// validateLogArgs check that a requested log-level is defined/allowed.
func (c *Configuration) validateLogArgs() error {
	mylog := log.ComponentLogger(componentName, "validateLogArgs")
	mylog.Debug().Msg("validating logging configuration")
	// check the configured log level is valid.
	if _, ok := log.LogLevels[c.LogLevel]; !ok {
		return errors.New(c.LogLevel + " is not a valid log-level")
	}
	return nil
}

func (c *Configuration) validateWebhookArgs() error {
	mylog := log.ComponentLogger(componentName, "validateWebhookArgs")
	mylog.Debug().Msg("validating webhook configuration")
	for _, p := range []string{"server.namespace", "server.service"} {
		if !viper.IsSet(p) {
			mylog.Error().Str("parameter", p).Msg("missing required parameter value")
			return fmt.Errorf("missing required parameter")
		}
	}
	return nil
}

func (c *Configuration) validateRules() error {
	mylog := log.ComponentLogger(componentName, "validateRules")
	mylog.Debug().Msg("validating graffiti rules")

	if len(c.Rules) == 0 {
		mylog.Error().Msg("configuration does not contain any rules")
		return errors.New("no rules found")
	}

	existingRuleNames := make(map[string]bool)
	for _, rule := range c.Rules {
		// rules can't have duplicate names...
		if _, set := existingRuleNames[rule.Registration.Name]; set == true {
			mylog.Error().Str("rule", rule.Registration.Name).Msg("found duplicate rules with the same name, they must be unique")
			return fmt.Errorf("rule %s is invalid - found duplicate rules with the same name, they must be unique", rule.Registration.Name)
		}
		existingRuleNames[rule.Registration.Name] = true

		if err := validateRule(rule); err != nil {
			return err
		}
	}
	return nil
}

func validateRule(rule Rule) error {
	mylog := log.ComponentLogger(componentName, "validateRule")
	rulelog := mylog.With().Str("rule", rule.Registration.Name).Logger()

	if err := rule.validateRuleSelectors(rulelog); err != nil {
		return err
	}
	if err := rule.validateRuleAdditions(rulelog); err != nil {
		return err
	}
	return nil
}

func (rule Rule) validateRuleSelectors(rulelog zerolog.Logger) error {
	// all label selectors must be valid...
	if len(rule.Matchers.LabelSelectors) > 0 {
		for _, selector := range rule.Matchers.LabelSelectors {
			if err := graffiti.ValidateLabelSelector(selector); err != nil {
				rulelog.Error().Str("label-selector", selector).Msg("rule contains an invalid label selector")
				return fmt.Errorf("rule %s is invalid - contains invalid label selector '%s': %v", rule.Registration.Name, selector, err)
			}
		}
	}

	// all field selectors must also be valid...
	if len(rule.Matchers.FieldSelectors) > 0 {
		for _, selector := range rule.Matchers.FieldSelectors {
			if err := graffiti.ValidateFieldSelector(selector); err != nil {
				rulelog.Error().Str("field-selector", selector).Msg("rule contains an invalid field selector")
				return fmt.Errorf("rule %s is invalid - contains invalid field selector '%s': %v", rule.Registration.Name, selector, err)
			}
		}
	}
	return nil
}

func (rule Rule) validateRuleAdditions(rulelog zerolog.Logger) error {
	// rules need to contain one or more label or annotation additions...
	if len(rule.Additions.Labels) == 0 && len(rule.Additions.Annotations) == 0 {
		rulelog.Error().Msg("invalid rule - it does not contain any additional labels or annotations")
		return fmt.Errorf("rule %s is invalid - it does not contain any additional labels or annotations", rule.Registration.Name)
	}

	// validate all additions labels using kubernetes validation methods
	templateRegex := regexp.MustCompile(`\{\{.*\}\}`)
	if len(rule.Additions.Labels) > 0 {
		for k, v := range rule.Additions.Labels {
			if errorList := utilvalidation.IsQualifiedName(k); len(errorList) != 0 {
				rulelog.Error().Str("label-key", k).Str("errors", strings.Join(errorList, "; ")).Msg("rule contains invalid additions label key")
				return fmt.Errorf("rule %s contains invalid label key: %s", rule.Registration.Name, strings.Join(errorList, "; "))
			}
			if templateRegex.MatchString(v) {
				rulelog.Info().Str("label-value", v).Msg("value contains a template - skipping validation")
			} else {
				if errorList := utilvalidation.IsValidLabelValue(v); len(errorList) != 0 {
					rulelog.Error().Str("label-value", v).Msg("rule contains invalid additions label value")
					return fmt.Errorf("rule %s contains invalid label value: %s", rule.Registration.Name, strings.Join(errorList, "; "))
				}
			}
		}
	}

	// validate all additions annotations by using kubernetes validation methods
	path := field.NewPath("metadata.annotations")
	if len(rule.Additions.Annotations) > 0 {
		if errorList := apivalidation.ValidateAnnotations(rule.Additions.Annotations, path); len(errorList) != 0 {
			var info []string
			for _, errorPart := range errorList.ToAggregate().Errors() {
				info = append(info, errorPart.Error())
			}
			rulelog.Error().Str("errors", strings.Join(info, "; ")).Msg("rule contains invalid annotations")
			return fmt.Errorf("rule %s contains invalid annotations: %s", rule.Registration.Name, strings.Join(info, "; "))
		}
	}
	return nil
}
