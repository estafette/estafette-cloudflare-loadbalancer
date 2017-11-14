package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ericchiang/k8s"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v2"
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

	kubeClient, err := getKubeClient()
	if err != nil {
		return nil, err
	}

	// return instance of KubernetesAPIClient
	return &kubernetesAPIClientImpl{
		kubeClient: kubeClient,
	}, nil
}

func getKubeClient() (*k8s.Client, error) {

	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath != "" {

		data, err := ioutil.ReadFile(kubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("Read kubeconfig error:\n%v", err)
		}

		// Unmarshal YAML into a Kubernetes config object.
		var config k8s.Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("Unmarshal kubeconfig error:\n%v", err)
		}

		// fmt.Printf("%#v", config)
		return k8s.NewClient(&config)
	}

	// init cloudflare api client
	return k8s.NewInClusterClient()
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
