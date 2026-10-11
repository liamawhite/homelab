# Network security architecture

## Deployment status

These diagrams describe the security model agreed in [ADR 0006](../adr/0006-explicit-l4-network-access.md) and the configuration being prepared in this checkout. **It is not yet enforced on the live cluster.** The live inspection during this change found the Ansible/Flux foundation running Cilium and Flux, with Flux's stock broad network grants and no Istio installation or persistent host deny firewall. API-server dry runs validate definitions; they do not establish enforcement.

Do not treat this milestone as complete until the allow/deny and recovery checks pass after deployment. See [the verification matrix](#verification).

## Ownership

Git contains the authoritative configuration for both owners. Ansible handles the node layer because it must work before Kubernetes starts and when the cluster is unavailable. Flux owns the Kubernetes layer after bootstrap; Ansible uses the same definitions for initial installation and deliberate recovery.

```mermaid
flowchart TB
    Git["Reviewed Git configuration"]
    Git --> Ansible["Ansible: host and bootstrap owner"]
    Git --> Flux["Flux: Kubernetes configuration owner"]

    subgraph Hosts["Node layer — independent of Kubernetes"]
        NFT["Persistent nftables INPUT / OUTPUT policy"]
        Boot["systemd ordering and approved resolver"]
        Recovery["Explicit SSH and direct API recovery access"]
    end
    Ansible --> NFT
    Ansible --> Boot
    Ansible --> Recovery

    subgraph Cluster["Kubernetes layer"]
        Cilium["Cilium: pod L4 default deny and explicit transport grants"]
        Admission["Admission: protect policy, identities, and workload privileges"]
        Istio["Istio ambient: strict mTLS and identity default deny"]
        Waypoint["Waypoints: service and external destination authorization"]
    end
    Flux --> Cilium
    Flux --> Admission
    Flux --> Istio
    Flux --> Waypoint
    Ansible -. "same manifests during bootstrap / deliberate repair" .-> Cluster
```

| Configuration | Normal owner | Enforcement component | Shared source |
| --- | --- | --- | --- |
| Node ingress/egress and bootstrap exceptions | Ansible | nftables | `ansible/roles/host_firewall/`, `ansible/inventory/group_vars/all.yml` |
| Administrator and resolver inputs | Git; Ansible and Flux consume them | Host firewall and Cilium | `clusters/homelab/flux-system/network-inputs.yaml` |
| Cilium installation and pod transport grants | Flux | Cilium | `clusters/homelab/cilium/`, `clusters/homelab/security/network.yaml` |
| Policy/identity ownership and application restrictions | Flux | Kubernetes admission and RBAC | `clusters/homelab/security/admission.yaml` |
| Ambient installation and identity authorization | Flux | Istio CNI, ztunnel, and waypoint proxies | `clusters/homelab/istio/`, `clusters/homelab/mesh-security/` |

Cilium owns pod forwarding; the host firewall's INPUT/OUTPUT chains cover host processes and host-network workloads. Host-network applications are infrastructure exceptions because the host firewall cannot distinguish individual applications sharing the host network identity.

## Application traffic

Both layers require explicit permission. Cilium grants a transport path between selected endpoints, usually HBONE on TCP 15008 for mesh connections. Istio checks the authenticated caller and the actual service port inside the tunnel. Permitting the tunnel does not authorize every service connection it could carry.

```mermaid
flowchart LR
    App["Application A"] --> L4["Cilium: explicit source → destination transport grant"]
    L4 --> Z["ztunnel: authenticated identity and service-port permission"]
    Z --> B["Application B"]
    Z --> W["Service waypoint, when required"]
    W --> Auth["Waypoint application authorization"]
    Auth --> Backend["Backend accepts only required waypoint identity"]

    App -. "No transport grant" .-> L4Drop["DENY at Cilium"]
    L4 -. "No identity / service permission" .-> IdentityDrop["DENY at Istio"]
    App -. "Direct backend bypass" .-> BypassDrop["DENY when waypoint path is required"]
```

A service without a waypoint still needs ztunnel authorization. A service whose security depends on a waypoint must restrict backend authorization to that waypoint's identity. New application namespaces require ambient enrollment and restricted Pod Security; applications cannot disable capture, relabel identities, or grant themselves access.

## Internet, LAN, and DNS

Application internet egress defaults deny at L4. The shared egress waypoint starts with no permitted external destinations. Adding an internet dependency requires an explicit workload-to-waypoint transport grant, an external service definition, per-workload authorization for that destination, and a waypoint-to-destination network grant. A waypoint alone is not an allowlist.

```mermaid
flowchart LR
    App["Application"] --> EPath["Explicit Cilium path to shared egress waypoint"]
    EPath --> EAuth["Waypoint: caller identity + approved external service"]
    EAuth --> Out["Explicit waypoint egress grant"]
    Out --> Internet["Approved internet destination and port"]
    App -. "Direct internet / waypoint bypass" .-> Deny["DENY at L4"]

    App --> LANRule["Explicit workload + device address + protocol + port"]
    LANRule --> LAN["Approved LAN device"]

    App --> DNSGrant["Explicit DNS permission"]
    DNSGrant --> CoreDNS["Designated cluster resolver"]
    CoreDNS --> Upstream["Approved upstream resolver: TCP / UDP 53"]
    App -. "Direct external DNS" .-> Deny
```

DNS-path restrictions do not prevent DNS exfiltration through arbitrary names queried via an approved resolver. Name filtering is a separate requirement. Protocols that cannot use the mesh remain denied for internet access unless a controlled proxy or specific infrastructure exception is approved.

Bootstrap infrastructure has separate, named destination exceptions: nodes need image/package downloads, and Flux's source-controller needs Git and the pinned chart repositories. Those exceptions do not grant application internet access. Host HTTPS destinations are resolved to IPv4 address sets; this enforces addresses and ports, not TLS hostnames on shared IP addresses.

## Bootstrap and recovery

```mermaid
flowchart TD
    Inputs["Validate administrator, resolver, and infrastructure allowlists"]
    Inputs --> Host["Ansible: persistent host deny firewall before K3s"]
    Host --> K3s["K3s: direct-node API and etcd"]
    K3s --> VIP["Ansible: shared kube-vip manifests"]
    VIP --> CNI["Ansible: shared Cilium release"]
    CNI --> PodDeny["Shared pod default deny, foundation grants, admission restrictions"]
    PodDeny --> Ambient["Shared Istio installation and identity deny baseline"]
    Ambient --> Handoff["Flux adopts the same definitions"]
    Handoff --> Graph["Flux dependency order: Cilium → network-security → Istio → mesh-security"]
    Graph --> Checks["Allowed AND denied traffic checks"]

    Operator["Approved administrator"] --> SSH["SSH survives cluster / mesh failure"]
    Operator --> API["Direct-node API bypasses VIP and pod routing"]
    SSH --> Repair["Ansible: deliberate repair from reviewed checkout"]
    API --> Repair
    Repair --> Resume["Resume Flux after successful repair"]
    Resume --> Checks
```

Recovery restores the reviewed policies; it does not temporarily open all application traffic. Admission-policy or mesh failures should stop affected application connections. The host firewall remains loaded even if its systemd service stops; it does not depend on a Kubernetes controller for persistence.

## Verification

| Check | Expected result |
| --- | --- |
| Approved client on each other node → test service TCP 8080 | Allowed through ambient |
| Same client → same listening service on unauthorized TCP 8081 | Denied |
| Caller with a transport grant but an unauthorized service identity | Denied by mesh authorization |
| Caller without the transport grant | Denied at L4 |
| Application → public IP directly | Denied; operator checks the target is reachable first |
| Application → direct node API without permission | Denied |
| Privileged application, host port, capture opt-out, or unauthorized policy/identity change | Rejected by admission |
| Approved administrator → node SSH and direct API | Allowed |
| Unapproved source / destination → host listening port or host egress target | Denied by host firewall |
| Mesh unavailable | Application access fails closed; administrative recovery remains usable |

`security` is the deliberate migration command; `verify` performs the cross-node traffic checks. Repository checks render manifests and Helm charts and validate security contracts. The complete live matrix, including disruption/recovery, remains a deployment gate; a passing static check is not a substitute.
