package main

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// LoadBalancerController orchestrates the load balancer update process
type LoadBalancerController interface {
	InitLoadBalancer(string, string, string, string) error
	RefreshLoadBalancerOnChanges(string) error
	RefreshLoadBalancerOnInterval(string, int) error
}

type loadBalancerControllerImpl struct {
	k8sAPIClient KubernetesAPIClient
	cfAPIClient  CloudflareAPIClient
	nodes        map[string]Node
	waitGroup    *sync.WaitGroup
}

// NewLoadBalancerController returns an instance of LoadBalancerController
func NewLoadBalancerController(key, email, organizationID string, waitGroup *sync.WaitGroup) (LoadBalancerController, error) {

	k8sAPIClient, err := NewKubernetesAPIClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Kubernetes api client")
		return nil, err
	}

	cfAPIClient, err := NewCloudflareAPIClient(key, email, organizationID)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare api client")
		return nil, err
	}

	// return instance of CloudflareAPIClient
	return &loadBalancerControllerImpl{
		k8sAPIClient: k8sAPIClient,
		cfAPIClient:  cfAPIClient,
		nodes:        make(map[string]Node),
		waitGroup:    waitGroup,
	}, nil
}

func (ctl *loadBalancerControllerImpl) InitLoadBalancer(poolName, lbName, zoneName, monitorPath string) (err error) {

	nodes, err := ctl.k8sAPIClient.GetHealthyNodes()
	if err != nil {
		log.Error().Err(err).Msg("Failed retrieving Kubernetes nodes")
		return
	}

	// copy nodes into map
	for _, node := range nodes {
		ctl.nodes[node.Name] = node
	}

	monitor, err := ctl.cfAPIClient.GetOrCreateLoadBalancerMonitor(poolName, zoneName, monitorPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare load balancer monitor")
		return
	}

	pool, err := ctl.cfAPIClient.GetOrCreateLoadBalancerPool(poolName, nodes, monitor)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare load balancer pool")
		return
	}

	loadBalancer, err := ctl.cfAPIClient.GetOrCreateLoadBalancer(lbName, zoneName, pool)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating load balancer")
		return
	}

	log.Debug().Interface("loadBalancer", loadBalancer).Msg("Load balancer object")

	return nil
}

func (ctl *loadBalancerControllerImpl) RefreshLoadBalancerOnChanges(poolName string) (err error) {

	// watch services for all namespaces
	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {
			// sleep random time between 22 and 37 seconds
			sleepTime := applyJitter(30)
			log.Info().Msgf("Sleeping for %v seconds...", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}(ctl.waitGroup)

	return nil
}

func (ctl *loadBalancerControllerImpl) RefreshLoadBalancerOnInterval(poolName string, interval int) (err error) {

	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {
			// sleep random time around 900 seconds
			sleepTime := applyJitter(interval)
			log.Info().Msgf("Sleeping for %v seconds...", sleepTime)
			time.Sleep(time.Duration(sleepTime) * time.Second)
		}
	}(ctl.waitGroup)

	return nil
}

func applyJitter(input int) (output int) {

	deviation := int(0.25 * float64(input))

	return input - deviation + r.Intn(2*deviation)
}
