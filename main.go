package main

import (
	stdlog "log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/alecthomas/kingpin"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()

	// flags
	cloudflareAPIEmail             = kingpin.Flag("cloudflare-api-email", "The email address used to authenticate to the Cloudflare API.").Envar("CF_API_EMAIL").Required().String()
	cloudflareAPIKey               = kingpin.Flag("cloudflare-api-key", "The api key used to authenticate to the Cloudflare API.").Envar("CF_API_KEY").Required().String()
	cloudflareLoadbalancerName     = kingpin.Flag("cloudflare-lb-name", "The name of the Cloudflare load balancer.").Envar("CF_LB_NAME").Required().String()
	cloudflareLoadbalancerPoolName = kingpin.Flag("cloudflare-lb-pool-name", "The name of the Cloudflare load balancer pool.").Envar("CF_LB_POOL_NAME").Required().String()
	cloudflareLoadbalancerZone     = kingpin.Flag("cloudflare-lb-zone", "The zone for the Cloudflare load balancer.").Envar("CF_LB_ZONE").Required().String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// log as severity for stackdriver logging to recognize the level
	zerolog.LevelFieldName = "severity"

	// set some default fields added to all logs
	log.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "estafette-cloudflare-dns").
		Str("version", version).
		Logger()

	// use zerolog for any logs sent via standard log library
	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)

	// log startup message
	log.Info().
		Str("branch", branch).
		Str("revision", revision).
		Str("buildDate", buildDate).
		Str("goVersion", goVersion).
		Msg("Starting estafette-cloudflare-loadbalancer...")

	// define channel and wait group to gracefully shutdown the application
	gracefulShutdown := make(chan os.Signal)
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	waitGroup := &sync.WaitGroup{}

	k8sAPIClient, err := NewKubernetesAPIClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Kubernetes api client")
	}

	nodes, err := k8sAPIClient.GetHealthyNodes()
	if err != nil {
		log.Error().Err(err).Msg("Failed retrieving Kubernetes nodes")
	}

	cfAPIClient, err := NewCloudflareAPIClient(*cloudflareAPIKey, *cloudflareAPIEmail)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare api client")
	}

	pool, err := cfAPIClient.GetOrCreateLoadBalancerPool(*cloudflareLoadbalancerPoolName, nodes)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare load balancer pool")
	}

	loadBalancer, err := cfAPIClient.GetOrCreateLoadBalancer(*cloudflareLoadbalancerName, *cloudflareLoadbalancerZone, pool)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating load balancer")
	}
	log.Debug().Interface("loadBalancer", loadBalancer).Msg("Load balancer object")

	// wait for sigterm
	signalReceived := <-gracefulShutdown
	log.Info().
		Msgf("Received signal %v. Waiting on running tasks to finish...", signalReceived)

	waitGroup.Wait()

	log.Info().Msg("Shutting down...")
}
