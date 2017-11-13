package main

import (
	stdlog "log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/alecthomas/kingpin"
	cloudflare "github.com/cloudflare/cloudflare-go"

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
	cloudflareAPIEmail         = kingpin.Flag("cloudflare-api-email", "The email address used to authenticate to the Cloudflare API.").Envar("CF_API_EMAIL").Required().String()
	cloudflareAPIKey           = kingpin.Flag("cloudflare-api-key", "The api key used to authenticate to the Cloudflare API.").Envar("CF_API_KEY").Required().String()
	cloudflareLoadbalancerName = kingpin.Flag("cloudflare-lb-name", "The name of the Cloudflare load balancer.").Envar("CF_LB_NAME").Required().String()
	cloudflareLoadbalancerZone = kingpin.Flag("cloudflare-lb-zone", "The zone for the Cloudflare load balancer.").Envar("CF_LB_ZONE").Required().String()
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
		Msg("Starting estafette-cloudflare-laodbalancer...")

	// define channel and wait group to gracefully shutdown the application
	gracefulShutdown := make(chan os.Signal)
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	waitGroup := &sync.WaitGroup{}

	// init cloudflare api client
	cfClient, err := cloudflare.New(*cloudflareAPIKey, *cloudflareAPIEmail)

	// get zone id
	zones, err := cfClient.ListZones(*cloudflareLoadbalancerZone)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error retrieving zone %v", *cloudflareLoadbalancerZone)
	}
	if len(zones) == 0 {
		log.Fatal().Err(err).Msgf("Zero zones returned when retrieving zone %v", *cloudflareLoadbalancerZone)
	}
	zoneID := zones[0].ID
	log.Debug().Msgf("Zone ID for zone %v is %v", *cloudflareLoadbalancerZone, zoneID)

	// retrieve load balancers for zone
	loadBalancers, err := cfClient.ListLoadBalancers(zoneID)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error retrieving load balancers for zone id %v", zoneID)
	}

	// check if load balancer exists
	loadBalancerExists := false
	var loadBalancer cloudflare.LoadBalancer
	if len(loadBalancers) > 0 {
		for _, lb := range loadBalancers {
			if loadBalancer.Name == *cloudflareLoadbalancerName {
				loadBalancerExists = true
				loadBalancer = lb
			}
		}
	}

	if !loadBalancerExists {
		// create loadbalancer
		loadBalancer, err = cfClient.CreateLoadBalancer(zoneID, cloudflare.LoadBalancer{
			Name:        *cloudflareLoadbalancerName,
			Description: "Created by estafette-cloudflare-loadbalancer",
		})
		if err != nil {
			log.Fatal().Err(err).Msgf("Error creating load balancer with name %v", cloudflareLoadbalancerName)
		}
	}

	log.Debug().Interface("loadBalancer", loadBalancer).Msgf("Load balancer object for zone %v and name %v", zoneID, *cloudflareLoadbalancerName)

	// wait for sigterm
	signalReceived := <-gracefulShutdown
	log.Info().
		Msgf("Received signal %v. Waiting on running tasks to finish...", signalReceived)

	waitGroup.Wait()

	log.Info().Msg("Shutting down...")
}
