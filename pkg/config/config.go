package config

import (
	"errors"
	"fmt"

	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/healthcheck"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

const (
	componentName = "config"
)

// All of our configuration modelled with mapstructure tags so that we can use viper to properly parse and load it for us.

// Configuration models the structre of our configuration values loaded through viper.
type Configuration struct {
	_             string                    `mapstructure:"config" yaml:"config"`
	LogLevel      string                    `mapstructure:"log-level" yaml:"log-level"`
	CheckExisting bool                      `mapstructure:"check-existing" yaml:"check-existing,omitempty"`
	HealthChecker healthcheck.HealthChecker `mapstructure:"health-checker" yaml:"health-checker,omitempty"`
	Server        Server                    `mapstructure:"server" yaml:"server"`
	Rules         []Rule                    `mapstructure:"rules" yaml:"rules"`
}

// Server contains all the settings for the webhook https server and access from the kubernetes api.
type Server struct {
	WebhookPort    int    `mapstructure:"port" yaml:"port"`
	CompanyDomain  string `mapstructure:"company-domain" yaml:"company-domain"`
	Namespace      string `mapstructure:"namespace" yaml:"namespace"`
	Service        string `mapstructure:"service" yaml:"service"`
	CACertPath     string `mapstructure:"ca-cert-path" yaml:"ca-cert-path"`
	ServerCertPath string `mapstructure:"cert-path" yaml:"cert-path"`
	ServerKeyPath  string `mapstructure:"key-path" yaml:"key-path"`
}

// Rule models a single graffiti rule with three sections for managing registration, matching and the payload to graffiti on the object.
type Rule struct {
	Registration webhook.Registration `mapstructure:"registration" yaml:"registration"`
	Matchers     graffiti.Matchers    `mapstructure:"matchers" yaml:"matchers,omitempty"`
	Payload      graffiti.Payload     `mapstructure:"payload" yaml:"payload"`
}

// ValidateConfig is responsible for throwing errors when the configuration is bad.
func (c Configuration) ValidateConfig() error {
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
func (c Configuration) validateLogArgs() error {
	mylog := log.ComponentLogger(componentName, "validateLogArgs")
	mylog.Debug().Msg("validating logging configuration")
	// check the configured log level is valid.
	if _, ok := log.LogLevels[c.LogLevel]; !ok {
		return errors.New(c.LogLevel + " is not a valid log-level")
	}
	return nil
}

func (c Configuration) validateWebhookArgs() error {
	mylog := log.ComponentLogger(componentName, "validateWebhookArgs")
	mylog.Debug().Msg("validating webhook configuration")
	if c.Server.Namespace == "" {
		mylog.Error().Str("parameter", "server.namespace").Msg("missing required parameter server.namespace")
		return fmt.Errorf("missing required parameter server.namespace")
	}
	if c.Server.Service == "" {
		mylog.Error().Str("parameter", "server.service").Msg("missing required parameter server.service")
		return fmt.Errorf("missing required parameter server.service")
	}
	return nil
}

func (c Configuration) validateRules() error {
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

		gr := graffiti.Rule{
			Name:     rule.Registration.Name,
			Matchers: rule.Matchers,
			Payload:  rule.Payload,
		}
		if err := gr.Validate(mylog); err != nil {
			return err
		}
	}
	return nil
}
