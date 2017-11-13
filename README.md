# estafette-cloudflare-loadbalancer

Kubernetes controller to create a Cloudflare load balancer with all GKE nodes as a backend pool

[![License](https://img.shields.io/github/license/estafette/estafette-cloudflare-loadbalancer.svg)](https://github.com/estafette/estafette-cloudflare-loadbalancer/blob/master/LICENSE)

## Why?

The Google load balancer is pretty expensive due to the costs of forwarding rules. Instead of using the Google load balancer a free Cloudflare load balancer can send traffic to each node in a GKE cluster. 

This application ensures once set up the nodes in the load balancer pool are kept up to date while autoscaling and preemptibles have nodes coming and going. It also ensures firewall rules are set to open up the nodes to traffic coming from the Cloudflare load balancer.