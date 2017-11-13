package main

import (
	"context"

	"github.com/ericchiang/k8s"
	"github.com/rs/zerolog/log"
)

type Node struct {
	Name       string
	ExternalIP string
}

// KubernetesAPIClient handles communications with the Kubernetes API
type KubernetesAPIClient interface {
	GetHealthyNodes() ([]Node, error)
}

type kubernetesAPIClientImpl struct {
	kubeClient *k8s.Client
}

// NewKubernetesAPIClient returns an instance of KubernetesAPIClient
func NewKubernetesAPIClient() (KubernetesAPIClient, error) {

	// init cloudflare api client
	kubeClient, err := k8s.NewInClusterClient()
	if err != nil {
		return nil, err
	}

	// return instance of KubernetesAPIClient
	return &kubernetesAPIClientImpl{
		kubeClient: kubeClient,
	}, nil
}

func (cl *kubernetesAPIClientImpl) GetHealthyNodes() (nodes []Node, err error) {

	nodes = []Node{}

	kubeNodes, err := cl.kubeClient.CoreV1().ListNodes(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("Retrieving Kubernetes nodes failed")
		return
	}

	if kubeNodes.Items == nil || len(kubeNodes.Items) == 0 {
		return
	}

	for _, node := range kubeNodes.Items {

		externalIP := ""
		for _, address := range node.Status.Addresses {
			if *address.Type == "ExternalIP" {
				externalIP = *address.Address
			}
		}

		nodes = append(nodes, Node{Name: *node.Metadata.Name, ExternalIP: externalIP})
	}

	return
}
