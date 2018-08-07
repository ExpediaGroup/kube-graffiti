package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// Configuration defaults: -
	defaultMetricsPort      = 8080
	defaultWebhookPort      = 8443
	defaultHealthPath       = "/healthz"
	defaultWebHookName      = "namespace-webhook"
	defaultWebHookNamespace = "namespace-webhook"
	defaultLogLevel         = "info"
)

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
	cfgFile            string
	defaultBlacklist   = []string{"kube-system", "default", "kube-public"}
	defaultLabels      = []string{}
	defaultAnnotations = []string{}
)

type configuration struct {
	config          string
	logLevel        string
	metricsPort     int
	name            string
	namespace       string
	service         string
	webhookPort     int
	webhookKeyfile  string
	webhookCertfile string
	webhookCAfile   string
	blacklist       []string
	checkExisting   bool
	labels          map[string]string
	annotations     map[string]string
}

// init defines all of the arguments passed to the program
func init() {
	// general logging, health and metrics args
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.istio-namespace-webhook.yaml)")
	newParam("loglevel", defaultLogLevel, "[LOGLEVEL] set logging verbosity to one of panic, fatal, error, warn, info, debug")
	newParam("metrics-port", defaultMetricsPort, "[METRICS_PORT] metrics/health port")

	// webhook args
	newParam("name", defaultWebHookName, "[NAME] the name of this webhook (used in registration)")
	newParam("namespace", defaultWebHookNamespace, "[NAMESPACE] namespace containing the webhook (used in registration)")
	newParam("service", defaultWebHookName, "[SERVICE] service pointing to this webhook (used in registration)")
	newParam("webhook-port", defaultWebhookPort, "[WEBHOOK_PORT] secure webhook port")
	newParam("webhook-keyfile", "ERROR", "[WEBHOOK_KEYFILE] path to the webhook private key file")
	newParam("webhook-certfile", "ERROR", "[WEBHOOK_CERTFILE] path to the webhook x509 pem file")
	newParam("webhook-cafile", "ERROR", "[WEBHOOK_CAFILE] path to ca that signed webhook cert")

	newParam("blacklist", defaultBlacklist, "[BLACKLIST] list of namespaces to ignore, --blacklist=a,b,c or --blacklist a --blacklist b --blacklist c")
	newParam("check-existing", false, "[CHECK_EXISTING] check and update existing namespaces")
	newParam("labels", defaultLabels, "[LABELS] labels to add: --labels a=123,b=xyz or --labels a=123 -labels b=xyz")
	newParam("annotations", defaultAnnotations, "[ANNOTATIONS] annotations to add: --annotations a=123,b=xyz or --annotations a=123 --annotations b=xyz)")
}

// newParam creates different types of cobra and viper bound configuration parameters
// it deterimine the type from the default value.
func newParam(p string, def interface{}, use string) {
	switch def.(type) {
	case int:
		rootCmd.PersistentFlags().Int(p, def.(int), use)
	case string:
		rootCmd.PersistentFlags().String(p, def.(string), use)
	case []string:
		rootCmd.PersistentFlags().StringSlice(p, def.([]string), use)
	case bool:
		rootCmd.PersistentFlags().Bool(p, def.(bool), use)
	default:
		log.Fatal().Str("param", p).Interface("type", def).Msg("did not recognise parameter type")
	}
	viper.BindPFlag(p, rootCmd.PersistentFlags().Lookup(p))
	viper.BindEnv(p, paramToEnv(p))
}

func loadConfigFromViper() *configuration {
	return &configuration{
		config:          cfgFile,
		logLevel:        getParam("loglevel", "info").(string),
		name:            getParam("name", defaultWebHookName).(string),
		namespace:       getParam("namespace", defaultWebHookNamespace).(string),
		service:         getParam("namespace", defaultWebHookNamespace).(string),
		metricsPort:     getParam("metrics-port", defaultMetricsPort).(int),
		webhookPort:     getParam("webhook-port", defaultWebhookPort).(int),
		webhookKeyfile:  getParam("webhook-keyfile", "ERROR").(string),
		webhookCertfile: getParam("webhook-certfile", "ERROR").(string),
		webhookCAfile:   getParam("webhook-cafile", "ERROR").(string),
		blacklist:       getParam("blacklist", defaultBlacklist).([]string),
		checkExisting:   getParam("check-existing", false).(bool),
		labels:          getParam("labels", map[string]string{}).(map[string]string),
		annotations:     getParam("annotations", map[string]string{}).(map[string]string),
	}
}

// getParam gets cobra/viper parameters of multiple types, it also uses
// a default value to determine the type
func getParam(p string, def interface{}) interface{} {
	var val interface{}
	if !viper.IsSet(p) {
		return def
	}
	switch def.(type) {
	case int:
		val = viper.GetInt(p)
	case string:
		val = viper.GetString(p)
	case []string:
		val = viper.GetStringSlice(p)
	case bool:
		val = viper.GetBool(p)
	default:
		log.Fatal().Str("param", p).Interface("type", def).Msg("did not recognise parameter type")
	}
	log.Info().Str("parameter", p).Str("env", paramToEnv(p)).Interface("value", val).Msg("loaded")
	return val
}

func paramToEnv(p string) string {
	var re = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	e := strings.ToUpper(re.ReplaceAllString(p, "_"))
	return e
}

// makeStringMapParam turns a string slice of key=value pairs validating both the key and values provided
func makeStringMapParam(p string, def map[string]string, validateKey func(string) bool, validateVal func(string) bool) (map[string]string, error) {
	if !viper.IsSet(p) {
		return def, nil
	}
	supplied := viper.GetStringSlice(p)
	values := make(map[string]string, len(supplied))
	for _, val := range supplied {
		parts := strings.Split(val, "=")
		if len(parts) != 2 {
			return values, fmt.Errorf("can't parse map parts (i.e. a=b) from %s", val)
		}
		if err := validateKey(parts[0]); err != nil {
			return values, fmt.Errorf("invalid key %s: %v", parts[0], err)
		}
		if err := validateVal(parts[1]); err != nil {
			return values, fmt.Errorf("invalid value %s: %v", parts[1], err)
		}
		values[parts[0]] = parts[1]
	}
	return values, nil
}

func validateLabelKey(key string) error {

}

func validateLab

// validateRootCmdArgs manages the validation of all arguments passed to the program
func (c *configuration) validateConfig() error {
	if err := c.validateBlacklistArg(); err != nil {
		return err
	}
	if err := c.validateWebhookArgs(); err != nil {
		return err
	}
	return c.validateLogArgs()
}

func (c *configuration) validateBlacklistArg() error {
	for _, name := range c.blacklist {
		if valid, err := isValidNamespaceName(name); !valid {
			return fmt.Errorf("invalid namespace name '%s': %v", name, err)
		}
	}
	return nil
}

func isValidNamespaceName(name string) (bool, error) {
	if len(name) == 0 {
		return false, errors.New("name is empty")
	}
	if len(name) > 253 {
		return false, errors.New("name is too long, must be <= 253 chars")
	}
	valid := regexp.MustCompile(`[a-z0-9.-]+`)
	if !valid.MatchString(name) {
		return false, errors.New("name contains illegal characters, can only contain chars [a-z0-9.-]")
	}
	return true, nil
}

// validateLogArgs check that a requested log-level is defined/allowed.
func (c *configuration) validateLogArgs() error {
	// check the configured log level is valid.
	if _, ok := logLevels[c.logLevel]; !ok {
		return errors.New(c.logLevel + "is not a valid log-level")
	}
	return nil
}

func (c *configuration) validateWebhookArgs() error {
	if c.webhookCAfile == "ERROR" || c.webhookCertfile == "ERROR" || c.webhookKeyfile == "ERROR" {
		return errors.New("you must provide values for webhook-cafile, webhook-certfile and webhook-keyfile")
	}
	return nil
}

func initConfig() {
	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".istio-namespace-webhook")
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
}
