package main

import (
	"flag"
	stdlog "log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	cloudflareAPIEmail                = kingpin.Flag("cloudflare-api-email", "The email address used to authenticate to the Cloudflare API.").Envar("CF_API_EMAIL").Required().String()
	cloudflareAPIKey                  = kingpin.Flag("cloudflare-api-key", "The api key used to authenticate to the Cloudflare API.").Envar("CF_API_KEY").Required().String()
	cloudflareOrganizationID          = kingpin.Flag("cloudflare-organization-id", "The organization id used to get organization level items from the Cloudflare API.").Envar("CF_ORG_ID").Required().String()
	cloudflareLoadbalancerName        = kingpin.Flag("cloudflare-lb-name", "The name of the Cloudflare load balancer.").Envar("CF_LB_NAME").Required().String()
	cloudflareLoadbalancerPoolName    = kingpin.Flag("cloudflare-lb-pool-name", "The name of the Cloudflare load balancer pool.").Envar("CF_LB_POOL_NAME").Required().String()
	cloudflareLoadbalancerZone        = kingpin.Flag("cloudflare-lb-zone", "The zone for the Cloudflare load balancer.").Envar("CF_LB_ZONE").Required().String()
	cloudflareLoadbalancerMonitorPath = kingpin.Flag("cloudflare-lb-monitor-path", "The path for the monitor the check the health of the Cloudflare load balancer pool.").Envar("CF_LB_MONITOR_PATH").Required().String()

	// prometheus metrics listener
	addr = flag.String("listen-address", ":9101", "The address to listen on for HTTP requests.")

	// define prometheus counter
	loadBalancerTotals = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "estafette_cloudflare_loadbalancer_pools_totals",
			Help: "Number of created/updated Cloudflare load balancer pools.",
		},
		[]string{"status"},
	)

	// seed random number
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(loadBalancerTotals)
}

func main() {

	// parse command line parameters
	kingpin.Parse()

	// log as severity for stackdriver logging to recognize the level
	zerolog.LevelFieldName = "severity"

	// set some default fields added to all logs
	log.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "estafette-cloudflare-loadbalancer").
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

	// start prometheus
	go func() {
		log.Debug().
			Str("port", *addr).
			Msg("Serving Prometheus metrics...")

		http.Handle("/metrics", promhttp.Handler())

		if err := http.ListenAndServe(*addr, nil); err != nil {
			log.Fatal().Err(err).Msg("Starting Prometheus listener failed")
		}
	}()

	lbController, err := NewLoadBalancerController(*cloudflareAPIKey, *cloudflareAPIEmail, *cloudflareOrganizationID, waitGroup)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed creating load balancer controller")
	}

	err = lbController.InitLoadBalancer(*cloudflareLoadbalancerPoolName, *cloudflareLoadbalancerName, *cloudflareLoadbalancerZone, *cloudflareLoadbalancerMonitorPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed initializing load balancer")
	}

	err = lbController.RefreshLoadBalancerOnChanges(*cloudflareLoadbalancerPoolName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed setting up refresh on changes")
	}

	err = lbController.RefreshLoadBalancerOnInterval(*cloudflareLoadbalancerPoolName, 900)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed setting up refresh on interval")
	}

	// wait for sigterm
	signalReceived := <-gracefulShutdown
	log.Info().
		Msgf("Received signal %v. Waiting on running tasks to finish...", signalReceived)

	waitGroup.Wait()

	log.Info().Msg("Shutting down...")
}
