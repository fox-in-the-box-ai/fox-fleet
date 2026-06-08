# Quickstart: Kubernetes

From zero to a running Fox Fleet with one provisioned assistant on
Kubernetes — under 15 minutes.

This example uses Kind (Kubernetes in Docker) for a local cluster.
The same Helm chart works on any cluster with Docker socket access.

---

## What you'll have at the end

- Fox Fleet running on a local Kind cluster via Helm
- One Fox AI assistant provisioned and accessible via port-forward
- The management panel showing instance health

---

## Prerequisites

- **Docker** — Engine or Desktop, running
- **kubectl** — [install](https://kubernetes.io/docs/tasks/tools/)
- **Helm 3+** — [install](https://helm.sh/docs/intro/install/)
- **Kind** — [install](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
  (or substitute with Minikube, K3s, or any cluster you have)

---

## Step 1: Create a cluster

```bash
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /var/run/docker.sock
        containerPath: /var/run/docker.sock
EOF
```

The `extraMounts` entry gives the cluster node access to the host's
Docker socket, which fox-control needs to manage containers.

Verify:

```bash
kubectl cluster-info
```

---

## Step 2: Install Fox Fleet

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet

helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="$(openssl rand -hex 32)" \
  --set auth.instancePassword="$(openssl rand -hex 32)"
```

Wait for the pod to be ready:

```bash
kubectl get pods -l app.kubernetes.io/name=fox-control -w
```

---

## Step 3: Access the panel

```bash
kubectl port-forward svc/fox-control 9090:9090
```

Open `http://localhost:9090` in your browser. Log in with your
admin secret.

To retrieve the admin secret:

```bash
kubectl get secret fox-control -o jsonpath='{.data.admin-secret}' | base64 -d
```

---

## Step 4: Provision your first Fox

```bash
SECRET=$(kubectl get secret fox-control -o jsonpath='{.data.admin-secret}' | base64 -d)

curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $SECRET" \
  -H "Content-Type: application/json" \
  -d '{"id": "my-fox"}'
```

Or click **"Create Instance"** in the panel.

Wait for the Fox container to start and pass health checks.

---

## Step 5: Access your Fox assistant

The Fox instance runs as a Docker container on the Kind node. To
access it, port-forward to the instance port (default 8787):

```bash
# From the Kind node (the instance runs on the host Docker, not in K8s)
# Open a new terminal and access the Fox instance directly
curl http://localhost:8787/health
```

> **Note:** In a Kind cluster, instances run on the host Docker daemon
> (not inside Kubernetes pods). They're accessible at `localhost:8787`
> on the machine running Kind.

---

## Clean up

```bash
# Destroy the instance
curl -X DELETE http://localhost:9090/api/instances/my-fox \
  -H "Authorization: Bearer $SECRET"

# Uninstall Fleet
helm uninstall fox-control

# Delete the cluster
kind delete cluster
```

---

## Production Kubernetes

For production clusters (EKS, GKE, AKS, K3s):

1. **Docker socket access** — the fox-control pod needs
   `/var/run/docker.sock` on its node. Use `nodeSelector` to pin it
   to a node where this is acceptable.
2. **External Qdrant** — the chart doesn't deploy Qdrant. Deploy it
   separately if you need the data plane.
3. **Ingress** — enable in `values.yaml` for external access with TLS.
4. **Secrets management** — use `auth.existingSecret` to reference a
   Secret managed by your cluster's secret store (Vault, SOPS, etc.).

See the [Helm deployment guide](../DEPLOYMENT.md#helm-kubernetes)
for full production configuration.

---

## What's next

- **Enable the data plane** — deploy Qdrant and configure embedding
  for knowledge-augmented responses
- **Add Ingress** — expose the panel and instances with TLS
- **Production hardening** — dedicated node, RBAC, NetworkPolicy

You now have Fleet running and one Fox provisioned. Continue to the
[Operator Handbook](../operator/handbook.md) for day-2 operations.
