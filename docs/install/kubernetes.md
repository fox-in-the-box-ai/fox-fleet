# Install Fox Fleet on Kubernetes

Fox Fleet runs on any Kubernetes cluster with Docker socket access on
the node. The Helm chart is the primary deployment method.

---

## Prerequisites

- Kubernetes cluster (tested with Kind, Minikube, K3s, EKS, GKE, AKS)
- `kubectl` configured for your cluster
- Helm 3+
- Docker socket (`/var/run/docker.sock`) accessible on the node where
  fox-control runs — see [Docker socket access](#docker-socket-access)

---

## Helm chart (recommended)

### Install

```bash
helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="$(openssl rand -hex 32)" \
  --set auth.instancePassword="$(openssl rand -hex 32)"
```

Or clone the repo first:

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="$(openssl rand -hex 32)" \
  --set auth.instancePassword="$(openssl rand -hex 32)"
```

### Verify

```bash
kubectl get pods -l app.kubernetes.io/name=fox-control
kubectl port-forward svc/fox-control 9090:9090
curl http://localhost:9090/healthz
# {"status":"ok"}
```

Open `http://localhost:9090` in your browser.

### Key values

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `ghcr.io/fox-in-the-box-ai/fox-control` | Container image |
| `image.tag` | `1.4.2` (chart appVersion) | Image tag |
| `service.port` | `9090` | Service port |
| `persistence.enabled` | `true` | Persistent volume for data |
| `persistence.size` | `1Gi` | Volume size |
| `config.maxInstances` | `10` | Instance cap |
| `config.dockerImage` | `ghcr.io/fox-in-the-box-ai/fox:latest` | Fox instance image |
| `config.portStart` | `8787` | First instance port |
| `auth.existingSecret` | `""` | Use an existing K8s Secret |
| `ingress.enabled` | `false` | Enable Ingress resource |

### Uninstall

```bash
helm uninstall fox-control
# Optionally delete the PVC:
kubectl delete pvc -l app.kubernetes.io/name=fox-control
```

---

## Docker socket access

fox-control manages Fox instances as Docker containers. On Kubernetes,
the pod needs a `hostPath` volume mount for `/var/run/docker.sock`.
The chart includes this by default.

**Security implications:** The pod has root access to the node's
Docker daemon. Mitigations:

- Use `nodeSelector` or `affinity` to pin fox-control to a dedicated
  node
- Enforce Pod Security Standards on other workloads
- Consider a dedicated node pool in managed K8s (EKS, GKE, AKS)

---

## External Qdrant

The Helm chart does **not** deploy Qdrant. If you need the data plane
(knowledge ingestion + vector search), deploy Qdrant separately:

```bash
# Example: Qdrant Helm chart
helm repo add qdrant https://qdrant.github.io/qdrant-helm
helm install qdrant qdrant/qdrant

# Then configure fox-control to use it
helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="..." \
  --set auth.instancePassword="..." \
  --set dataPlane.enabled=true \
  --set qdrant.enabled=true \
  --set embedding.baseURL="http://ollama.default.svc:11434"
```

---

## Ingress

```yaml
# values.yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: fox.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: fox-tls
      hosts:
        - fox.example.com
```

---

## Container image only (no Helm)

If you prefer plain manifests over Helm:

```bash
docker pull ghcr.io/fox-in-the-box-ai/fox-control:1.4.2
```

Verify the image signature:

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  ghcr.io/fox-in-the-box-ai/fox-control:1.4.2
```

Write your own Deployment, Service, ConfigMap, and Secret manifests
referencing the image. See the Helm chart templates in
`deploy/helm/fox-control/templates/` as a starting point.

---

## Next steps

- [Kubernetes Quickstart](../quickstart/kubernetes.md) — from zero to
  a running Fleet with one provisioned Fox assistant
- [Configuration Reference](../configuration.md) — full config file
  documentation
- [Deployment Guide](../DEPLOYMENT.md#helm-kubernetes) — full Helm
  deployment guide
