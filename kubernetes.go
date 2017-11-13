package main

import (
	"context"

	"github.com/ericchiang/k8s"
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

	labels := new(k8s.LabelSelector)
	labels.Eq("cloud.google.com/gke-preemptible", "true")
	kubeNodes, err := cl.kubeClient.CoreV1().ListNodes(context.Background(), labels.Selector())

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
