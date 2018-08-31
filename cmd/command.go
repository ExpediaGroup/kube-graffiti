package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"stash.hcom/run/kube-graffiti/pkg/config"
	"stash.hcom/run/kube-graffiti/pkg/existing"
	"stash.hcom/run/kube-graffiti/pkg/graffiti"
	"stash.hcom/run/kube-graffiti/pkg/healthcheck"
	"stash.hcom/run/kube-graffiti/pkg/log"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

var (
	componentName = "cmd"
	cfgFile       string
	rootCmd       = &cobra.Command{
		Use:     "kube-grafitti",
		Short:   "Automatically add labels and/or annotations to kubernetes objects",
		Long:    `Write rules that match labels and object fields and add labels/annotations to kubernetes objects as they are created via a mutating webhook.`,
		Example: `kube-graffiti --config ./config.yaml`,
		PreRun:  initRootCmd,
		Run:     runRootCmd,
	}
)

// init defines command-line and environment arguments
func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/config", "[GRAFFITI_CONFIG] config file (default is /config.{yaml,json,toml,hcl})")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	rootCmd.PersistentFlags().String("log-level", config.DefaultLogLevel, "[GRAFFITI_LOG_LEVEL] set logging verbosity to one of panic, fatal, error, warn, info, debug")
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	// viper.BindEnv("log-level", "GRAFFITI_LOG_LEVEL")
	rootCmd.PersistentFlags().Bool("check-existing", false, "[GRAFFITTI_CHECK_EXISTING] run rules against existing objects")
	viper.BindPFlag("check-existing", rootCmd.PersistentFlags().Lookup("check-existing"))

	// set up Viper environment variable binding...
	replacer := strings.NewReplacer("-", "_", ".", "_")
	viper.SetEnvPrefix("GRAFFITI")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initRootCmd(_ *cobra.Command, _ []string) {
	log.InitLogger(viper.GetString("log-level"))
}

// runRootCmd is the main program which starts up our services and waits for them to complete
func runRootCmd(_ *cobra.Command, _ []string) {
	mylog := log.ComponentLogger(componentName, "runRootCmd")

	mylog.Info().Str("file", viper.GetString("config")).Msg("reading configuration file")
	config, err := config.LoadConfig(viper.GetString("config"))
	if err != nil {
		mylog.Fatal().Err(err).Msg("failed to load config")
	}

	mylog.Info().Str("level", viper.GetString("log-level")).Msg("Setting log-level to configured level")
	log.ChangeLogLevel(viper.GetString("log-level"))
	mylog = log.ComponentLogger(componentName, "runRootCmd")
	mylog.Info().Str("log-level", viper.GetString("log-level")).Msg("This is the log level")

	mylog.Info().Msg("configuration read ok")
	mylog.Debug().Msg("validating config")
	if err := config.ValidateConfig(); err != nil {
		mylog.Fatal().Err(err).Msg("failed to validate config")
	}

	mylog.Debug().Msg("getting kubernetes client")
	kubeClient := initKubeClient()
	// Setup and start the health-checker
	healthChecker := healthcheck.NewHealthChecker(healthcheck.NewCutDownNamespaceClient(kubeClient), viper.GetInt("health-checker.port"), viper.GetString("health-checker.path"))
	healthChecker.StartHealthChecker()

	// Setup and start the mutating webhook server
	if err := initWebhookServer(config, kubeClient); err != nil {
		mylog.Fatal().Err(err).Msg("webhook server failed to start")
	}

	/*if err := initExistingNamespacesCheck(kubeClient); err != nil {
		log.Fatal().Err(err).Msg("failed to check existing namespaces")
	}*/

	// wait for an interrupt
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	<-signalChan
	os.Exit(0)
}

// initKubeClient returns a valid kubernetes client only when running within a kubernetes pod.
func initKubeClient() *kubernetes.Clientset {
	mylog := log.ComponentLogger(componentName, "initKubeClient")
	// creates the in-cluster config
	mylog.Info().Msg("creating kubeconfig")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	mylog.Debug().Msg("creating kubernetes api clientset")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

func initWebhookServer(c *config.Configuration, k *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "initWebhookServer")
	port := viper.GetInt("server.port")

	mylog.Debug().Int("port", port).Msg("creating a new webhook server")
	caPath := viper.GetString("server.ca-cert-path")
	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		mylog.Error().Err(err).Str("path", caPath).Msg("Failed to load ca from file")
		return errors.New("failed to load ca from file")
	}
	mylog.Debug().Str("ca-cert-path", caPath).Msg("loaded ca cert ok")
	server := webhook.NewServer(
		viper.GetString("server.company-domain"),
		viper.GetString("server.namespace"),
		viper.GetString("server.service"),
		ca, k,
		viper.GetInt("server.port"),
	)

	// add each of the graffiti rules into the mux
	mylog.Info().Int("count", len(c.Rules)).Msg("loading graffiti rules")
	for _, rule := range c.Rules {
		mylog.Info().Str("rule-name", rule.Registration.Name).Msg("adding graffiti rule")
		server.AddGraffitiRule(graffiti.Rule{
			Name:      rule.Registration.Name,
			Matchers:  rule.Matchers,
			Additions: rule.Additions,
		})
	}

	mylog.Info().Int("port", port).Str("server.cert-path", viper.GetString("server.cert-path")).Str("server.key-path", viper.GetString("server.key-path")).Msg("starting webhook secure webserver")
	server.StartWebhookServer(viper.GetString("server.cert-path"), viper.GetString("server.key-path"))

	mylog.Debug().Msg("waiting 2 seconds")
	time.Sleep(2 * time.Second)

	// register all rules with the kubernetes apiserver
	for _, rule := range c.Rules {
		mylog.Info().Str("name", rule.Registration.Name).Msg("registering rule with api server")
		err = server.RegisterHook(rule.Registration, k)
		if err != nil {
			mylog.Error().Err(err).Str("name", rule.Registration.Name).Msg("failed to register rule with apiserver")
			return err
		}
	}

	return nil
}

func initExistingCheck(k *kubernetes.Clientset) error {
	mylog := log.ComponentLogger(componentName, "initExistingCheck")

	var err error
	if viper.GetBool("check-existing") {
		mylog.Info().Msg("checking of existing objects is disabled")
		return nil
	}
	if err = existing.CheckExistingObjects(k); err != nil {
		return err
	}
	mylog.Info().Msg("check of existing objects completed successfully")

	return nil
}
