package main

import (
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"

	"github.com/rs/zerolog/log"
)

// LoadBalancerController orchestrates the load balancer update process
type LoadBalancerController interface {
	Init(string, string, string, string) error
	InitDns(string, string) error
	InitMonitor(string, string, string) error
	InitPool(string) error
	InitLoadBalancer(string, string) error
	RefreshLoadBalancerOnChanges(string) error
	RefreshLoadBalancerOnInterval(string, string, string, int) error
}

type loadBalancerControllerImpl struct {
	k8sAPIClient KubernetesAPIClient
	cfAPIClient  CloudflareAPIClient
	nodes        map[string]Node
	lbType       string

	monitor      cloudflare.LoadBalancerMonitor
	pool         cloudflare.LoadBalancerPool
	loadbalancer cloudflare.LoadBalancer

	waitGroup *sync.WaitGroup
}

// NewLoadBalancerController returns an instance of LoadBalancerController
func NewLoadBalancerController(key, email, organizationID, lbType string, waitGroup *sync.WaitGroup) (LoadBalancerController, error) {

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
		lbType:       lbType,
		waitGroup:    waitGroup,
	}, nil
}

func (ctl *loadBalancerControllerImpl) Init(poolName, lbName, zoneName, monitorPath string) (err error) {

	if ctl.lbType == "dns" {

		err = ctl.InitDns(lbName, zoneName)
		if err != nil {
			return
		}

	} else if ctl.lbType == "lb" {

		err = ctl.InitMonitor(poolName, zoneName, monitorPath)
		if err != nil {
			return
		}

		err = ctl.InitPool(poolName)
		if err != nil {
			return
		}

		err = ctl.InitLoadBalancer(lbName, zoneName)
		if err != nil {
			return
		}

	}

	return
}

func (ctl *loadBalancerControllerImpl) InitDns(lbName, zoneName string) (err error) {

	// todo set dns records <lbName>.<zoneName> for each node; remove ones that no longer point to an existing node

	return
}

func (ctl *loadBalancerControllerImpl) InitMonitor(poolName, zoneName, monitorPath string) (err error) {

	ctl.monitor, err = ctl.cfAPIClient.GetOrCreateLoadBalancerMonitor(poolName, zoneName, monitorPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare load balancer monitor")
		return
	}

	return
}

func (ctl *loadBalancerControllerImpl) InitPool(poolName string) (err error) {

	nodes, err := ctl.k8sAPIClient.GetHealthyNodes()
	if err != nil {
		log.Error().Err(err).Msg("Failed retrieving Kubernetes nodes")
		return
	}

	// copy nodes into map
	for _, node := range nodes {
		ctl.nodes[node.Name] = node
	}

	ctl.pool, err = ctl.cfAPIClient.GetOrCreateLoadBalancerPool(poolName, nodes, ctl.monitor)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating Cloudflare load balancer pool")
		return
	}

	return
}

func (ctl *loadBalancerControllerImpl) InitLoadBalancer(lbName, zoneName string) (err error) {

	ctl.loadbalancer, err = ctl.cfAPIClient.GetOrCreateLoadBalancer(lbName, zoneName, ctl.pool)
	if err != nil {
		log.Error().Err(err).Msg("Failed creating load balancer")
		return
	}

	log.Debug().Interface("loadBalancer", ctl.loadbalancer).Msg("Load balancer object")

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

func (ctl *loadBalancerControllerImpl) RefreshLoadBalancerOnInterval(poolName, lbName, zoneName string, interval int) (err error) {

	go func(waitGroup *sync.WaitGroup) {
		// loop indefinitely
		for {

			if ctl.lbType == "dns" {

				err = ctl.InitDns(lbName, zoneName)
				if err != nil {
					return
				}

			} else if ctl.lbType == "lb" {

				err = ctl.InitPool(poolName)
				if err != nil {
					log.Warn().Err(err).Msgf("Updating pool with name %v failed", poolName)
				}

			}

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
