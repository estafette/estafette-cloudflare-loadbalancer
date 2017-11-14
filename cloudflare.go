package main

import (
	"fmt"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/rs/zerolog/log"
)

// CloudflareAPIClient handles communications with the Cloudflare API
type CloudflareAPIClient interface {
	GetOrCreateLoadBalancerPool(string, []Node) (cloudflare.LoadBalancerPool, error)
	GetOrCreateLoadBalancer(string, string, cloudflare.LoadBalancerPool) (cloudflare.LoadBalancer, error)
}

type cloudflareAPIClientImpl struct {
	apiClient *cloudflare.API
}

// NewCloudflareAPIClient returns an instance of CloudflareAPIClient
func NewCloudflareAPIClient(key, email, organizationID string) (CloudflareAPIClient, error) {

	// init cloudflare api client
	apiClient, err := cloudflare.New(key, email)
	if err != nil {
		return nil, err
	}

	if organizationID != "" {
		apiClient, err = cloudflare.New(key, email, cloudflare.UsingOrganization(organizationID))
		if err != nil {
			return nil, err
		}
	}

	// return instance of CloudflareAPIClient
	return &cloudflareAPIClientImpl{
		apiClient: apiClient,
	}, nil
}

func (cl *cloudflareAPIClientImpl) GetOrCreateLoadBalancerPool(poolName string, nodes []Node) (pool cloudflare.LoadBalancerPool, err error) {

	// retrieve load balancer pools
	loadBalancerPools, err := cl.apiClient.ListLoadBalancerPools()
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving load balancer pools")
		return
	}
	log.Debug().Interface("loadBalancerPools", loadBalancerPools).Msg("Retrieved load balancer pools")

	// check if load balancer exists
	loadBalancerPoolExists := false
	if len(loadBalancerPools) > 0 {
		for _, lbp := range loadBalancerPools {
			if lbp.Name == poolName {
				loadBalancerPoolExists = true
				pool = lbp
			}
		}
	}

	// create list of origins from nodes
	origins := []cloudflare.LoadBalancerOrigin{}
	for _, node := range nodes {
		origins = append(origins, cloudflare.LoadBalancerOrigin{
			Name:    node.Name,
			Address: node.ExternalIP,
			Enabled: true,
		})
	}
	log.Debug().Interface("nodes", nodes).Interface("origins", origins).Msg("Created origins from nodes")

	if !loadBalancerPoolExists {
		// create load balancer pool
		pool, err = cl.apiClient.CreateLoadBalancerPool(cloudflare.LoadBalancerPool{
			Name:    poolName,
			Origins: origins,
			Enabled: true,
		})
		if err != nil {
			log.Error().Err(err).Msgf("Error creating load balancer pool with name %v", poolName)
			return
		}
	} else {
		// update load balancer pool
		pool.Origins = origins
		pool, err = cl.apiClient.ModifyLoadBalancerPool(pool)
		if err != nil {
			log.Error().Err(err).Msgf("Error updating load balancer pool with name %v", poolName)
			return
		}
	}
	log.Debug().Interface("loadBalancerPool", pool).Msgf("Load balancer pool object for name %v", poolName)

	return
}

func (cl *cloudflareAPIClientImpl) GetOrCreateLoadBalancer(loadbalancerName, zoneName string, pool cloudflare.LoadBalancerPool) (loadBalancer cloudflare.LoadBalancer, err error) {

	// get zone id
	zones, err := cl.apiClient.ListZones(zoneName)
	if err != nil {
		log.Error().Err(err).Msgf("Error retrieving zone %v", zoneName)
		return
	}
	if len(zones) == 0 {
		log.Error().Err(err).Msgf("Zero zones returned when retrieving zone %v", zoneName)
		return
	}
	zoneID := zones[0].ID
	log.Debug().Msgf("Zone ID for zone %v is %v", zoneName, zoneID)

	// retrieve load balancers for zone
	loadBalancers, err := cl.apiClient.ListLoadBalancers(zoneID)
	if err != nil {
		log.Error().Err(err).Msgf("Error retrieving load balancers for zone id %v", zoneID)
		return
	}
	log.Debug().Interface("loadBalancers", loadBalancers).Msgf("Retrieved load balancers for zone %v", zoneID)

	// check if load balancer exists
	lbName := fmt.Sprintf("%v.%v", loadbalancerName, zoneName)
	loadBalancerExists := false
	if len(loadBalancers) > 0 {
		for _, lb := range loadBalancers {
			if lb.Name == lbName {
				loadBalancerExists = true
				loadBalancer = lb
			}
		}
	}

	if !loadBalancerExists {
		// create loadbalancer
		loadBalancer, err = cl.apiClient.CreateLoadBalancer(zoneID, cloudflare.LoadBalancer{
			Name:         lbName,
			Description:  "Created by estafette-cloudflare-loadbalancer",
			FallbackPool: pool.ID,
			DefaultPools: []string{pool.ID},
			Proxied:      true,
		})
		if err != nil {
			log.Error().Err(err).Msgf("Error creating load balancer with name %v", lbName)
			return
		}
	} else {
		if !contains(loadBalancer.DefaultPools, pool.ID) {
			loadBalancer.DefaultPools = append(loadBalancer.DefaultPools, pool.ID)
			cl.apiClient.ModifyLoadBalancer(zoneID, loadBalancer)
			if err != nil {
				log.Error().Err(err).Msgf("Error updating load balancer with name %v", lbName)
				return
			}
		}
	}
	log.Debug().Interface("loadBalancer", loadBalancer).Msgf("Load balancer object for zone %v and name %v", zoneID, lbName)

	return
}

func contains(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}
