package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"stash.hcom/run/kube-graffiti/pkg/blacklist"
	"stash.hcom/run/kube-graffiti/pkg/existing"
	"stash.hcom/run/kube-graffiti/pkg/healthcheck"
	"stash.hcom/run/kube-graffiti/pkg/webhook"
)

var rootCmd = &cobra.Command{
	Use:   "namespace-webhook",
	Short: "Automatically add labels and/or annotations to your namespaces",
	Long:  "Operates as a kubernetes mutating webhook that compares each namespace to a blacklist before applying the alterations",
	Run:   runRootCmd,
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
	bl := initBlackList()
	blacklist.SharedBlacklist = bl
	kubeClient := initKubeClient()

	if err := initWebhookServer(kubeClient); err != nil {
		log.Fatal().Err(err).Msg("webhook server failed to start")
	}

	healthcheck.AddHealthCheckHandler(defaultHealthPath, kubeClient)
	if err := initMetricsWebServer(); err != nil {
		log.Fatal().Err(err).Msg("metrics/health server failed to start")
	}

	if err := initExistingNamespacesCheck(kubeClient); err != nil {
		log.Fatal().Err(err).Msg("failed to check existing namespaces")
	}

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

// initBlackList loads the blacklist from values configured in the environment 'BLACKLIST', e.g. BLACKLIST="a b c d"
// or command-line --blacklist=a,b,c --blacklist=d results in blacklist [ "a", "b", "c", "d" ]
// Entry can not be mixed, environment will be ignored if command-line --blacklist is supplied.
func initBlackList() blacklist.Blacklist {
	bl := blacklist.New()
	if viper.IsSet("blacklist") {
		bl.Set(getParam("blacklist", []string{}).([]string)...)
	}
	return bl
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

func initExistingNamespacesCheck(k *kubernetes.Clientset) error {
	var err error
	doCheck := getParam("check-existing", false).(bool)
	if !doCheck {
		log.Info().Msg("checking of existing namespaces is disabled")
		return nil
	}
	if err = existing.CheckExistingNamespaces(k); err != nil {
		return err
	}
	log.Info().Msg("check of existing namespaces completed successfully")

	return nil
}
