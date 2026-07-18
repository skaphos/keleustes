<!--
SPDX-FileCopyrightText: 2026 Rillan AI LLC
SPDX-License-Identifier: MIT
-->

# Keleustes Gateway API skeleton

This directory holds the **internal-tier** `Gateway` and `HTTPRoute` for the
Keleustes API listener (per
[runtime plan §7.5](../../docs/plans/2026-05-distributed-runtime-architecture.md#75-external-exposure-surface-gateway-api--tiered-exposure)).
It is the only tier MVP 0 ships. The public-webhooks tier (SKA-357, MVP 2)
and the agent-transport `TLSRoute` (SKA-363, MVP 2) land alongside the
engines that need them.

**Hard rule:** Keleustes uses Gateway API v1 (`gateway.networking.k8s.io/v1`).
No `Ingress`, ever — not even as a compatibility shim. If your cluster doesn't
have a Gateway API controller installed, install one before deploying
Keleustes' API listener.

## Files

| File | Purpose |
|------|---------|
| `gateway.yaml` | The `Gateway` itself: one HTTPS listener, `Same`-namespace route attachment, TLS via a customer-provided Secret. |
| `httproute-api.yaml` | `HTTPRoute` mapping `/api/*` to the future `keleustes-api` Service. The Service does not exist in MVP 0; the route will show `ResolvedRefs=False` until the API server lands in MVP 1+. |
| `kustomization.yaml` | Kustomize entry-point for `kubectl apply -k config/gateway/`. |

Not included in `config/default/`: Gateway API CRDs are customer-installed and
customer-versioned. Apply this overlay separately so it can fail fast when the
controller is missing, rather than poisoning the main install.

## What you need to install first

1. **Gateway API CRDs.** The standard channel is sufficient:
   ```bash
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml
   ```
   Pin the version to one your controller supports.

2. **A Gateway controller of your choice.** Keleustes is intentionally
   controller-agnostic — pick one your platform team already operates. See
   the [Compatible controllers](#compatible-controllers) table below.

3. **A TLS Secret named `keleustes-internal-tls`** in the
   `skaphos-keleustes-system` namespace, or a [cert-manager](https://cert-manager.io/)
   `Certificate` that issues into that Secret. Without it the `Gateway` will
   report `Programmed=False` on its listener.

## Customer overrides

The shipped YAML uses placeholders that **must** be replaced. The two minimal
overrides are:

```yaml
# kustomization.yaml in your downstream overlay
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../keleustes/config/gateway

patches:
  - target:
      kind: Gateway
      name: keleustes-internal
    patch: |-
      - op: replace
        path: /spec/gatewayClassName
        value: eg                    # your controller's GatewayClass
      - op: replace
        path: /spec/listeners/0/hostname
        value: keleustes.example.com # your DNS name
  - target:
      kind: HTTPRoute
      name: keleustes-api
    patch: |-
      - op: replace
        path: /spec/hostnames/0
        value: keleustes.example.com
```

DNS, certificate issuance, and `LoadBalancer` IP allocation are the platform
team's responsibility and out of scope for this overlay.

## Compatible controllers

Keleustes targets Gateway API v1 conformance only. Any controller listed on
the
[Gateway API implementations page](https://gateway-api.sigs.k8s.io/implementations/)
at the `Standard` channel works. Pick one your platform team already runs.

| Controller | `GatewayClass` value | Notes |
|------------|---------------------|-------|
| [Envoy Gateway](https://gateway.envoyproxy.io/) | `eg` | First-class recommended (per runtime plan §13). Easiest path on `kind` for smoke testing. |
| [GKE Gateway](https://cloud.google.com/kubernetes-engine/docs/concepts/gateway-api) | `gke-l7-rilb` (regional internal) / `gke-l7-global-external-managed` (external) | Native on GKE. Pairs with IAP for the future UI tier (MVP 3). |
| [AKS Application Gateway for Containers](https://learn.microsoft.com/en-us/azure/application-gateway/for-containers/overview) | `azure-application-lb` | Native on AKS. Pairs with AAD App Proxy for the future UI tier. |
| [Contour](https://projectcontour.io/) | `contour` | Mature; pairs with Pomerium/oauth2-proxy ExtAuth for the future UI tier. |
| [Cilium Gateway](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/) | `cilium` | If your CNI is already Cilium, this is the lowest-overhead choice. |
| [Istio](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) | `istio` | If you already run Istio for mesh. |

The runtime plan calls out Envoy Gateway + one cloud-managed controller (GKE
Gateway or AKS Application Gateway for Containers) as the eventual first-class
test matrix.

## Smoke test with Envoy Gateway on kind

End-to-end on a throwaway `kind` cluster, no DNS needed:

```bash
# 1. kind cluster
kind create cluster --name keleustes-gateway-smoke

# 2. Gateway API CRDs
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml

# 3. Envoy Gateway (controller + GatewayClass `eg`)
helm install eg oci://docker.io/envoyproxy/gateway-helm \
  --version v1.2.0 \
  --namespace envoy-gateway-system \
  --create-namespace
kubectl wait --namespace envoy-gateway-system \
  --for=condition=Available deployment/envoy-gateway --timeout=120s

# 4. Skaphos namespace + a self-signed TLS Secret for the listener
kubectl create namespace skaphos-keleustes-system
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout /tmp/tls.key -out /tmp/tls.crt \
  -subj "/CN=keleustes.example.com" -days 1
kubectl -n skaphos-keleustes-system create secret tls keleustes-internal-tls \
  --cert=/tmp/tls.crt --key=/tmp/tls.key

# 5. Apply this overlay with the GatewayClass override
kustomize build config/gateway/ \
  | sed 's/gatewayClassName: REPLACE_ME/gatewayClassName: eg/' \
  | kubectl apply -f -

# 6. Verify the Gateway accepted and Programmed
kubectl -n skaphos-keleustes-system get gateway keleustes-internal -o yaml \
  | grep -A 3 'conditions:'
# Expected: Accepted=True; Programmed=True (status may take ~30s).

# 7. Verify the HTTPRoute is attached (ResolvedRefs will be False until
#    the keleustes-api Service exists — that's MVP 1+; here we just
#    confirm the route is parented to the Gateway).
kubectl -n skaphos-keleustes-system get httproute keleustes-api -o yaml \
  | grep -A 5 'parents:'

# 8. Teardown
kind delete cluster --name keleustes-gateway-smoke
```

For end-to-end traffic verification you need a backend; stub it with a
namespaced echo server like `hashicorp/http-echo` pointed at by a `Service`
named `keleustes-api` on port 8443. That stub is out of scope for this
skeleton — the real `keleustes-api` Service lands when the API server does.

## What this overlay deliberately does not do

- **No webhook receivers.** Public-internet webhooks (`/webhooks/github`,
  `/webhooks/gitlab`, ...) get their own public `Gateway` per runtime plan
  §7.5 — different identity model, different scaling profile, must not share
  a listener with the API. SKA-357 lands those in MVP 2.
- **No UI tier.** UI access goes through an IAP-fronted `Gateway`
  (Google IAP, AAD App Proxy, Cloudflare Access, Pomerium, oauth2-proxy).
  SKA-375 lands the integration recipes in MVP 3.
- **No agent transport.** NATS leaf endpoint runs on its own `Gateway` with
  `TLSRoute` passthrough (or a standalone `LoadBalancer` Service). SKA-363
  lands it in MVP 2.
- **No metrics or profiling Routes.** `/metrics` and profiling endpoints stay
  cluster-internal — `ClusterIP` Service plus NetworkPolicy, no Gateway. The
  observability bundle (SKA-403) ships those.
- **No `ReferenceGrant`.** The HTTPRoute and its backend live in the same
  namespace; cross-namespace backends will need a `ReferenceGrant` when they
  appear.

These are intentional gaps, each tracked by its own ticket. Don't paper over
them by adding routes here — the tiered separation is load-bearing.
