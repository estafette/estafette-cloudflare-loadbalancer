package main

import "github.com/rs/zerolog/log"

// LoadBalancerController orchestrates the load balancer update process
type LoadBalancerController interface {
	InitLoadBalancer(string, string, string, string) error
}

type loadBalancerControllerImpl struct {
	k8sAPIClient KubernetesAPIClient
	cfAPIClient  CloudflareAPIClient
}

// NewLoadBalancerController returns an instance of LoadBalancerController
func NewLoadBalancerController(key, email, organizationID string) (LoadBalancerController, error) {

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
	}, nil
}

func (ctl *loadBalancerControllerImpl) InitLoadBalancer(poolName, lbName, zoneName, monitorPath string) (err error) {

	nodes, err := ctl.k8sAPIClient.GetHealthyNodes()
	if err != nil {
		log.Error().Err(err).Msg("Failed retrieving Kubernetes nodes")
		return
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
