# Tailscale migration

## Settled scope

- Migrate Tailscale from the legacy deployment into the new homelab deployment.
- Cloudflare is excluded from this migration.
- Keep the existing tailnet and preserve previous Tailscale service hostnames as applications return.
- Install the operator first and verify HTTPS access through a temporary service; production endpoints follow their application migrations.
- Leave existing tailnet access rules unchanged; manage only the operator's required tag ownership.
- Reuse the existing operator OAuth credential. Ansible delivers it from the git-crypt-protected configuration into a Kubernetes Secret referenced by Flux; Flux does not receive the git-crypt key.
- Keep ingress proxies enrolled in Istio ambient. Private requests traverse application waypoints with explicit service-account authorization; tailnet access controls entry and mesh policies control backend access.
- Verify successful tailnet HTTPS access, denied unauthorized backend access, and failure when the mesh is unavailable. Remove the temporary endpoint and its tailnet device afterward, retaining a repeatable verification procedure.
- Start with no Tailscale-specific outbound grants. Determine required connections from observed denied traffic before adding grants; do not copy the legacy internet transport allowance.
- Ansible's Tailscale responsibilities are limited to secret delivery. It does not change tailnet policy or deploy Tailscale resources.
- Support only HTTPS Ingress through userspace proxies in this milestone, retaining restricted Pod Security and narrowly scoped operator permissions for proxy resources.
- Check existing tag ownership; the owner applies any missing required ownership in the Tailscale admin console. Preserve existing owners and access rules; do not automate policy writes.
- After design confirmation, implement and deploy the initially blocked operator. Inspect operator logs and Cilium denied flows, then propose grants identifying source, destination, protocol, port, purpose, and evidence. Obtain a decision on each proposed grant before enabling it.
- Retain failed temporary verification resources for diagnosis without relaxing restrictions. Clean them up after successful verification.

## Deployment decisions deferred until evidence exists

- Required outbound grants, based on operator logs and denied flows.

## Outbound policy direction

The owner settled the boundary: all internet-bound traffic from ordinary pods,
including Flux and Tailscale, must traverse the egress gateway. The mesh must
originate outbound TLS; client TLS passthrough does not satisfy this requirement.
Only explicitly registered external destinations may be allowed, with gateway
authorization and network enforcement as well as routing configuration. External
TCP 80 remains blocked. Explicit cluster and LAN permissions remain separate;
node processes and host-network traffic retain the existing externally managed
enforcement boundary. The previous direct chart-download grant is superseded by
this design and must not be enabled.

The earlier port-80 restriction must distinguish external TCP 80 from internal client
HTTP that is carried through mesh mTLS to the egress waypoint. Native client
HTTPS cannot be converted into mesh TLS origination by adding a ServiceEntry
and DestinationRule alone. Flux repository redirects and absolute HTTPS chart
URLs, as well as Tailscale's native TLS and VPN transports, require an explicit
compatibility decision before implementing this policy.

Pinned Tailscale compatibility research found that changing the operator's
loginServer can affect its API/control URL, but DERP relay connections still
derive native HTTPS URLs from the DERP map. Standard ServiceEntries and TLS
origination rules do not turn those existing TLS streams into HTTP inputs.
Direct WireGuard UDP cannot satisfy the current gateway/TLS model either.
Tailscale internet access therefore remains blocked while a compatible path is
designed; no TLS passthrough or direct UDP exception is authorized.

The owner confirmed shared understanding and authorized implementation and blocked deployment.

## Initial blocked deployment

Flux definitions live in `clusters/homelab/tailscale` and `tailscale-proxies`.
The second reconciliation waits for the operator chart to install its CRDs before
applying ProxyClass resources; this ordering is required on a fresh cluster.
The operator chart is pinned to the legacy version, 1.102.4; both the operator
and userspace proxies use restricted security contexts. No Tailscale-specific
network grants or DNS-access labels are initially present.

Create the namespace from the shared definition, then deliver the existing OAuth
Secret with `scripts/homelab tailscale-secrets`. This command only delivers the
Secret; it does not deploy the operator or modify tailnet policy. It reads secrets
from the unlocked checkout without exposing them in Ansible output or Helm values.

During the local diagnostic rollout, install the exact public Helm values directly
with Helm, using the release name and namespace in the Flux definition. This
stages the blocked operator without requiring an outbound chart-download grant
for Flux. Stage the HelmRepository and a suspended HelmRelease separately; resume
the HelmRelease for adoption once chart downloads are approved and working.
The existing foundation continues to reconcile its current Git revision.

Initial live inspection found the cluster-wide `homelab-default-deny` Cilium
policy invalid: its empty rules are rejected with “rule must have at least one
of Ingress, IngressDeny, Egress, EgressDeny.” Tailscale therefore has its own
standard Kubernetes deny-all NetworkPolicy, applied before operator startup.
This isolates the migration without changing the foundation's active Git revision;
repair and full verification of the cluster-wide baseline remain outstanding.
The repository baseline now uses Cilium's `none` entity in nonempty rule lists,
which enables deny-by-default without matching any real peer; this correction
still needs rollout and full-cluster verification.
The corrected rule form passed live Cilium validation in an additional policy
scoped only to the already-denied Tailscale namespace. That validation policy
was removed afterward; the namespace deny-all NetworkPolicy remains in place.
The required operator/proxy tag ownership already exists; no console repair was
needed during initial inspection.

Inspect operator logs and Cilium denied flows before proposing any grants. Do not
resume automatic reconciliation of unreviewed local changes or relax the existing
security baseline to work around blocked connections.

## Initial evidence and proposed grants

The blocked operator was installed with the shared Helm values on 2026-10-11.
Ansible delivered only `tailscale/operator-oauth`; OAuth credentials were not
passed to Helm. The pod runs as UID 65532 with ambient capture enabled, and
Cilium endpoint inspection confirms ingress and egress enforcement enabled.
The staged Flux HelmRelease is suspended pending chart-download access and adoption.

Captured Cilium drops and operator logs establish the first required connection:
the `tailscale/operator` identity attempts Kubernetes API calls, translated from
`kubernetes.default.svc:443` to control-plane TCP 6443. The capture shows drops to
`192.168.1.52:6443` with the kube-apiserver identity and `192.168.1.60:6443` with
the host identity on the operator's own node. Propose only those API entity types
and TCP 6443 for this service account; do not yet grant proxies, DNS, or internet access.

The Flux source-controller also times out fetching
`https://pkgs.tailscale.com/helmcharts/index.yaml`. Captured drops from that
controller reach the site's CDN addresses on TCP 443. Propose FQDN-scoped TCP 443
to `pkgs.tailscale.com` for `flux-system/source-controller`, using its existing
DNS-inspection path rather than allowing all internet destinations.

The concrete proposals live in `diagnostics/tailscale/proposed-grants.yaml`.
They are not referenced by Flux and have not been applied. Each awaits the owner's
decision before being moved into active definitions and enabled.
The later gateway decision supersedes the direct chart-download proposal, which
has been removed. Kubernetes API access remains a pending internal-path decision.

Local shared-definition checks, pinned chart rendering, Ansible syntax checks,
and server dry runs passed. Initial inspection found the invalid Cilium baseline;
the gateway-first rollout below repaired and verified it. Global admission
enforcement remains deliberately deferred to issue #26 on main.

## Gateway-first rollout and verification

The gateway-first policy is implemented and staged live. Flux controllers are
ambient-enrolled; the source-controller's direct-download grant has been removed.
ServiceEntries register GitHub, the Cilium chart origin, and the Istio chart origin.
Authorization names the source-controller identity and specific Git/chart paths.
The egress waypoint originates TLS on 443 with explicit SNI, subject-alternative
name checks, and system trust roots. Both IPv4 and IPv6 service VIPs route to it.
No external port-80 or TLS-passthrough listener is registered.

Flux uses HTTP origin URLs inside mesh mTLS. An internal static Istio index
retains original chart metadata and digests while rewriting absolute download
URLs to HTTP origins. Update its pinned entries after changing chart versions:

```sh
uv run --project ansible --locked python ansible/tools/update_helm_index.py
scripts/homelab check
```

Review the gateway's chart path permissions at the same time. Cold bootstrap
installs ambient, the shared gateway resources, and the internal index before
Flux starts; operator-side chart downloads still use HTTPS independently of the mesh.

The corrected cluster-wide deny baseline is now active. Live rollout exposed
ambient local delivery from `169.254.7.127`; that exact source and required
receiver ports are explicitly permitted. Merge review revealed that main had
deliberately deferred global admission enforcement to issue #26. The older
branch's temporarily staged admission policies were retired, and the final
change preserves that deferral. Tailscale's explicitly requested restricted
namespace and proxy security contexts remain scoped to its migration.

Verified live:

- Fresh Flux Git clone, including upload-pack POST, and all five pinned Cilium/Istio chart downloads through upstream TLS 443.
- Envoy's actual configuration contains expected SNI, exact certificate SAN checks, and system trust roots for all three origins.
- Unauthorized identity and unapproved Git path return 403; unregistered hosts, direct public-IP ports 80/443, and client HTTPS bypass fail.
- Waypoint outage fails closed, and recovery restores the allowed path.
- Cross-node allowed mesh traffic and denied identities/ports/node access/internet bypass.
- Istio access logging emits JSON to stdout, confirmed on the live egress waypoint.

`scripts/verify-egress` repeats the gateway checks and cleans successful temporary
resources. Use `--disruptive` to additionally stop and restore the waypoint;
failed verification retains resources for diagnosis. The cross-node matrix
passed. Global admission verification remains a separate deferred milestone.

All active Flux controllers and foundation Helm releases are healthy. The old
Flux containers were confirmed exited before their stuck terminating Pod objects
were removed, allowing source-controller recreation and fresh artifact downloads.
Covered stale node-runtime netns mounts remain a separate maintenance finding;
no node reboot or broad namespace removal was performed.

## Review and reconciliation handoff

The live GitRepository still follows `main`. To prevent the previous revision
from restoring direct internet grants, these Kustomizations are temporarily
suspended: `flux-system`, `cilium`, `network-security`, `istio`, `mesh-security`.
Helm controllers continue reconciling the verified live release definitions;
`kube-vip` reconciliation is active. Merge the reviewed definitions before resuming
the held foundation Kustomizations, then require Ready on the updated revision.
Do not resume them against the older Git configuration.

Tailscale's HelmRepository and HelmRelease are suspended, and its checked-in
Kustomizations are held. The local operator remains network-blocked; its API
grant is not applied and its native DERP/TLS compatibility is still unresolved.
Ansible's Tailscale role remains secret delivery only; Cloudflare is excluded.
