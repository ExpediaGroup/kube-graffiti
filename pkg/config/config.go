package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
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

type Configuration struct {
	LogLevel      string                    `mapstructure:"log-level"`
	CheckExisting bool                      `mapstructure:"check-existing"`
	HealthChecker healthcheck.HealthChecker `mapstructure:"health-checker"`
	Server        Server                    `mapstructure:"server"`
	Rules         []Rule                    `mapstructure:"rules"`
}

type Server struct {
	WebhookPort    int    `mapstructure:"port"`
	CompanyDomain  string `mapstructure:"company-domain"`
	Namespace      string `mapstructure:"namespace"`
	Service        string `mapstructure:"service"`
	CACertPath     string `mapstructure:"ca-cert-path"`
	ServerCertPath string `mapstructure:"cert-path"`
	ServerKeyPath  string `mapstructure:"key-path"`
}

type Rule struct {
	Registration webhook.Registration `mapstructure:"registration"`
	Matcher      graffiti.Matcher     `mapstructure:"matcher"`
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
	viper.SetDefault("server.ca-cert-path", "/ca.pem")
	viper.SetDefault("server.cert-path", "/server.pem")
	viper.SetDefault("server.cert-path", "/key.pem")
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
		return &c, fmt.Errorf("Failed to unmarshal configuration: %v", err)
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
	if err := c.validateWebhookArgs(); err != nil {
		return err
	}
	return c.validateLogArgs()
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
