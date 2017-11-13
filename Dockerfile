FROM scratch

LABEL maintainer="estafette.io" \
      description="The estafette-cloudflare-dns component is a Kubernetes controller that create a Cloudflare load balancer with all GKE nodes as a backend pool"

COPY ca-certificates.crt /etc/ssl/certs/
COPY estafette-cloudflare-loadbalancer /

ENTRYPOINT ["/estafette-cloudflare-loadbalancer"]