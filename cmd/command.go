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
		Use:   "kube-grafitti",
		Short: "Automatically add labels and/or annotations to kubernetes objects",
		Long:  "Write rules that match labels and object fields and add labels/annotations to kubernetes objects as they are created via a mutating webhook.",
		Run:   runRootCmd,
	}
	defaultConfigPath = "/config"
)

// init defines command-line and environment arguments
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /config.yaml)")
	rootCmd.PersistentFlags().String("log-level", config.DefaultLogLevel, "[GRAFFITI_LOG_LEVEL] set logging verbosity to one of panic, fatal, error, warn, info, debug")
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	rootCmd.PersistentFlags().Bool("check-existing", false, "[GRAFFITTI_CHECK_EXISTING] run rules against existing objects")
	viper.BindPFlag("check-existing", rootCmd.PersistentFlags().Lookup("check-existing"))

	// set up Viper environment variable binding...
	replacer := strings.NewReplacer("-", "_", ".", "_")
	viper.SetEnvPrefix("GRAFFITI_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()
	config.SetDefaults()
}

// initConfig is reponsible for loading the viper configuration file.
func initConfig() {
	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(defaultConfigPath)
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// runRootCmd is the main program which starts up our services and waits for them to complete
func runRootCmd(_ *cobra.Command, _ []string) {
	log.InitLogger(viper.GetString("log-level"))
	mylog := log.ComponentLogger(componentName, "runRootCmd")

	config, err := config.ReadConfiguration()
	if err != nil {
		mylog.Fatal().Err(err).Msg("failed to load config")
	}
	if err := config.ValidateConfig(); err != nil {
		mylog.Fatal().Err(err).Msg("failed to validate config")
	}

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
	caPath := viper.GetString("server.ca-path")
	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		mylog.Error().Err(err).Str("path", caPath).Msg("Failed to load ca from file")
		return errors.New("failed to load ca from file")
	}
	server := webhook.NewServer(
		viper.GetString("server.company-domain"),
		viper.GetString("server.namespace"),
		viper.GetString("server.service"),
		ca, k,
		viper.GetInt("server.port"),
	)

	// add each of the graffiti rules into the mux
	for _, rule := range c.Rules {
		server.AddGraffitiRule(rule.Registration.Name, graffiti.Rule{
			Matcher:   rule.Matcher,
			Additions: rule.Additions,
		})
		return nil
	}

	mylog.Info().Int("port", port).Msg("starting webhook secure webserver")
	server.StartWebhookServer(viper.GetString("server.cert-path"), viper.GetString("server.key-path"))

	mylog.Debug().Msg("waiting 2 seconds")
	time.Sleep(2 * time.Second)

	// register all rules with the kubernetes apiserver
	for _, rule := range c.Rules {
		mylog.Info().Str("name", rule.Registration.Name).Msg("registering rule with api server")
		err = server.RegisterHook(rule.Registration.Name, rule.Registration, k)
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
