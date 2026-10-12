# Network security architecture

## Deployment status

These diagrams describe [ADR 0006](../adr/0006-explicit-l4-network-access.md) and the stricter internet boundary in [ADR 0008](../adr/0008-require-gateway-for-pod-internet-egress.md). The live cluster now has valid Cilium default deny, Istio ambient, and admission enforcement. Flux Git and pinned chart downloads have been verified through the TLS-originating egress waypoint, including denied direct bypass and waypoint outage/recovery. Tailscale remains blocked pending compatibility with mandatory TLS origination. Root and foundation reconciliation are temporarily suspended until these locally staged definitions are reviewed and merged.

Node and host-network isolation is managed by the homelab owner through external local-network controls. This repository does not configure node firewalls or administrator CIDR allowlists. Cilium still restricts pod access to node services.

## Current observed deployment

```mermaid
flowchart LR
    Ansible["Ansible"] --> Nodes["K3s on three nodes"]
    Nodes --> Cilium["Cilium pod networking"]
    Flux["Flux"] --> Cilium
    Flux --> VIP["kube-vip"]
    Flux --> Controllers["Ambient Flux controllers"]
    Controllers --> Egress["Shared egress waypoint: registered destinations and TLS origination"]
    Egress --> Origins["Git and pinned chart origins: TLS 443"]
    Pending["Tailscale staged with traffic blocked"]
```

## Ownership after migration

Git is authoritative for the Kubernetes security model. Flux owns ongoing reconciliation; Ansible consumes the same definitions for bootstrap and deliberate recovery. The external node-isolation boundary is separate from those owners.

```mermaid
flowchart TB
    Owner["Homelab owner"] --> External["External local-network isolation"]
    External --> Nodes["Nodes and host-network infrastructure"]

    Git["Reviewed Git configuration"] --> Ansible["Ansible: K3s, bootstrap, and recovery"]
    Git --> Flux["Flux: Kubernetes configuration owner"]
    Ansible --> Nodes

    subgraph Cluster["Kubernetes security layer"]
        Cilium["Cilium: pod L4 default deny and explicit transport grants"]
        Admission["Admission: policy ownership, workload identity, privileges"]
        Istio["Istio ambient: strict mTLS and identity default deny"]
        Waypoint["Waypoints: service and external destination authorization"]
    end
    Flux --> Cilium
    Flux --> Admission
    Flux --> Istio
    Flux --> Waypoint
    Ansible -. "same definitions during bootstrap / deliberate repair" .-> Cluster
```

| Configuration | Normal owner | Enforcement component | Shared source |
| --- | --- | --- | --- |
| Node and host-network isolation | Homelab owner, outside this repo | External network controls | Outside this repo |
| K3s installation and recovery access | Ansible | Host services and direct-node API | `ansible/`, `os/` |
| Cilium installation and pod transport grants | Flux | Cilium | `clusters/homelab/cilium/`, `clusters/homelab/security/network.yaml` |
| Approved upstream DNS destination for CoreDNS | Flux | Cilium pod egress policy | `clusters/homelab/flux-system/network-inputs.yaml` |
| Policy/identity ownership and application restrictions | Flux | Kubernetes admission and RBAC | `clusters/homelab/security/admission.yaml` |
| Ambient installation and identity authorization | Flux | Istio CNI, ztunnel, and waypoints | `clusters/homelab/istio/`, `clusters/homelab/mesh-security/` |

Host-network workloads share node networking and are outside the pod deny baseline. Ordinary applications cannot use host networking or equivalent privileges; trusted infrastructure is an explicit exception. There is no repository-managed node INPUT/OUTPUT firewall.

## Application traffic

Both layers require permission. Cilium permits a path between selected endpoints, usually HBONE on TCP 15008. Istio checks the authenticated caller and actual service port inside that tunnel. A tunnel grant does not authorize every service connection it can carry.

```mermaid
flowchart LR
    App["Application A"] --> L4["Cilium: explicit source → destination transport grant"]
    L4 --> Z["ztunnel: authenticated identity and service-port permission"]
    Z --> B["Application B"]
    Z --> W["Service waypoint, when required"]
    W --> Auth["Waypoint application authorization"]
    Auth --> Backend["Backend accepts only the required waypoint identity"]

    App -. "No transport grant" .-> L4Drop["DENY at Cilium"]
    L4 -. "No identity / service permission" .-> IdentityDrop["DENY at Istio"]
    App -. "Direct backend bypass" .-> BypassDrop["DENY when waypoint path is required"]
    App -. "No grant to node API / LAN" .-> L4Drop
```

A service without a waypoint still needs ztunnel authorization. A service whose security depends on a waypoint must authorize that waypoint's identity at the backend. Application namespaces require ambient enrollment and restricted Pod Security. Application identities cannot disable capture, change workload identities, or grant network access.

## Internet, LAN, and DNS

All ordinary-pod internet egress defaults deny at L4 and must traverse the shared egress waypoint. Adding an internet dependency requires a workload-to-waypoint path, an external service definition, per-workload destination authorization, a waypoint-to-destination network grant, and external TLS origination with certificate validation. Client HTTP is carried through mesh mTLS to the waypoint; external TCP 80 and client TLS passthrough are not allowed. Registration alone and Istio's `REGISTRY_ONLY` setting are not an ambient security boundary; Cilium and gateway authorization enforce it.

```mermaid
flowchart LR
    App["Application"] --> EPath["Explicit Cilium path to shared egress waypoint"]
    EPath --> EAuth["Waypoint: caller identity + approved external service"]
    EAuth --> Out["Explicit waypoint egress grant"]
    Out --> Internet["Approved internet destination and port"]
    App -. "Direct internet / waypoint bypass" .-> Deny["DENY at Cilium"]

    App --> LANRule["Explicit workload + device address + protocol + port"]
    LANRule --> LAN["Approved LAN device"]

    App --> DNSGrant["Explicit DNS permission"]
    DNSGrant --> CoreDNS["Designated cluster resolver"]
    CoreDNS --> Upstream["Approved upstream: TCP / UDP 53"]
    App -. "Direct external DNS" .-> Deny
```

Resolver-path restrictions do not prevent DNS exfiltration through arbitrary names queried via the approved resolver. Name filtering is separate. Internet protocols unsupported by the mesh remain denied unless a controlled proxy or specific infrastructure exception is approved.

Flux controllers are ambient-enrolled. The source-controller can request only the homelab Git discovery/upload-pack paths and approved pinned chart paths through ServiceEntry authorization. Its former direct-download exception is removed. A small internal index retains upstream Istio chart digests but supplies HTTP download URLs so absolute upstream HTTPS URLs cannot bypass mesh TLS origination. Istio Envoy access logs are JSON on `/dev/stdout`; ztunnel retains its own connection logs.

Ambient local delivery and probes use `169.254.7.127`; Cilium grants that exact source only on the required receiver ports. Ztunnel authorization still controls authenticated application requests. Admission trusts exact built-in K3s workload-controller identities, and permits namespace-controller security-resource deletion only while the owning namespace is already deleting.

Node image/package downloads, host-network egress, and operator-side bootstrap chart downloads remain outside these pod policies and depend on externally managed isolation. Bootstrap establishes the gateway, registered origins, and internal index before Flux starts downloading.

## Bootstrap and recovery

```mermaid
flowchart TD
    External["Externally managed node isolation and operator access"]
    External --> K3s["Ansible: K3s and direct-node API"]
    K3s --> VIP["Shared kube-vip manifests"]
    VIP --> CNI["Shared Cilium release"]
    CNI --> PodDeny["Shared pod deny baseline, foundation grants, admission restrictions"]
    PodDeny --> Ambient["Shared Istio installation and identity deny baseline"]
    Ambient --> Handoff["Flux adopts the same definitions"]
    Handoff --> Graph["Flux dependency order: Cilium → network-security → Istio → mesh-security"]
    Graph --> Checks["Allowed AND denied traffic checks"]

    Operator["Administrator via external access controls"] --> SSH["SSH independent of mesh availability"]
    Operator --> API["Direct-node API bypasses VIP and pod routing"]
    SSH --> Repair["Ansible: deliberate repair from reviewed checkout"]
    API --> Repair
    Repair --> Resume["Resume Flux after successful repair"]
    Resume --> Checks
```

Recovery restores the reviewed policies rather than opening application traffic. API and SSH access must remain usable through the externally managed node controls. Mesh enrollment, identity, or authorization failures should stop affected application connections.

## Verification

| Check | Expected result |
| --- | --- |
| Approved client on each other node → test service TCP 8080 | Allowed through ambient |
| Same client → same listening service on unauthorized TCP 8081 | Denied |
| Caller with a transport grant but unauthorized service identity | Denied by mesh authorization |
| Caller without a transport grant | Denied at L4 |
| Application → public IP directly | Denied; operator first checks the target is reachable |
| Application → direct node API without permission | Denied by pod policy |
| Privileged application, capture opt-out, or unauthorized policy/identity change | Rejected by admission |
| Mesh unavailable | Application access fails closed; external administrative recovery remains usable |

`security` performs the deliberate cluster-policy migration; `verify` runs the cross-node and admission checks. Repository checks render manifests and exact Helm values and validate security contracts. The full live matrix, including a disruptive mesh outage/recovery exercise, remains a deployment gate. Node firewall tests and administrator CIDRs are outside this model.
