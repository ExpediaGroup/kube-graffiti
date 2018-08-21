package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"stash.hcom/run/kube-graffiti/pkg/existing"
	"stash.hcom/run/kube-graffiti/pkg/healthcheck"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "kube-grafitti",
		Short: "Automatically add labels and/or annotations to kubernetes objects",
		Long:  "Write rules that match labels and object fields and add labels/annotations to kubernetes objects as they are created via a mutating webhook.",
		Run:   runRootCmd,
	}
)

// init defines command-line and environment arguments
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /config.yaml)")
	rootCmd.PersistentFlags().String("log-level", defaultLogLevel, "[LOG_LEVEL] set logging verbosity to one of panic, fatal, error, warn, info, debug")
	bindCmdEnvFlag(rootCmd, "server.log-level", "log-level")
	rootCmd.PersistentFlags().Bool("check-existing", false, "[CHECK_EXISTING] check and update existing namespaces")
	bindCmdEnvFlag(rootCmd, "server.check-existing", "check-existing")
}

func bindCmdEnvFlag(command *cobra.Command, nested, short string) {
	viper.BindPFlag(nested, command.PersistentFlags().Lookup(short))
	viper.BindEnv(nested, paramToEnv(short))
}

func paramToEnv(p string) string {
	var re = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	e := strings.ToUpper(re.ReplaceAllString(p, "_"))
	return e
}

// initConfig
func initConfig() {
	// Don't forget to read config either from cfgFile or from home directory!
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("/config")
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
	initLogger()
	config := loadConfigFromViper()
	if err := config.validateConfig(); err != nil {
		log.Fatal().Err(err).Msg("failed to validate config")
	}
	kubeClient := initKubeClient()

	if err := initWebhookServer(kubeClient); err != nil {
		log.Fatal().Err(err).Msg("webhook server failed to start")
	}

	healthcheck.AddHealthCheckHandler(defaultHealthPath, kubeClient)
	if err := initMetricsWebServer(); err != nil {
		log.Fatal().Err(err).Msg("metrics/health server failed to start")
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

// initLogger sets up our logger such as logging level and to use the consolewriter.
func initLogger() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	// set level width if PR https://github.com/rs/zerolog/pull/87 is accepted
	// zerolog.LevelWidth = 5
	level := getParam("loglevel", defaultLogLevel).(string)
	zerolog.SetGlobalLevel(logLevels[level])
}

// initKubeClient returns a valid kubernetes client only when running within a kubernetes pod.
func initKubeClient() *kubernetes.Clientset {
	// creates the in-cluster config
	log.Info().Msg("creating kubeconfig")
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	log.Debug().Msg("creating kubernetes api clientset")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

func initMetricsWebServer() error {
	port := getParam("metrics-port", defaultMetricsPort).(int)
	log.Info().Int("metrics-port", port).Msg("starting metrics/health web server")
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Error().Err(err).Msg("health check webserver failed")
			panic("healthz webserver failed")
		}
	}()
	return nil
}

func initWebhookServer(k *kubernetes.Clientset) error {
	port := getParam("webhook-port", defaultWebhookPort).(int)
	paths := make(map[string]string, 3)
	for _, p := range []string{"webhook-certfile", "webhook-keyfile", "webhook-cafile"} {
		paths[p] = getParam(p, "ERROR").(string)
		if paths[p] == "ERROR" {
			log.Fatal().Str("parameter", p).Msg("missing required parameter value")
		}
	}

	log.Info().Int("port", port).Msg("starting webhook secure webserver")
	webhook.StartWebhookServer(port, k, paths["webhook-certfile"], paths["webhook-keyfile"])

	log.Debug().Msg("waiting 2 seconds")
	time.Sleep(2 * time.Second)

	name := getParam("webhook-name", defaultWebHookName).(string)
	ns := getParam("namespace", defaultWebHookNamespace).(string)

	ca, err := ioutil.ReadFile(paths["webhook-cafile"])
	if err != nil {
		log.Error().Err(err).Str("path", paths["webhook-cafile"]).Msg("Failed to load ca")
		return errors.New("failed to read ca")
	}

	log.Info().Msg("registering webhook with apiserver")
	if err := webhook.RegisterWebhook(k, name, ns, ca); err != nil {
		return err
	}
	return nil
}

func initExistingCheck(k *kubernetes.Clientset) error {
	var err error
	doCheck := getParam("check-existing", false).(bool)
	if !doCheck {
		log.Info().Msg("checking of existing objects is disabled")
		return nil
	}
	if err = existing.CheckExistingObjects(k); err != nil {
		return err
	}
	log.Info().Msg("check of existing objects completed successfully")

	return nil
}
