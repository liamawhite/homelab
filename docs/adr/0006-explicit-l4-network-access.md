# Require explicit L4 network access across the homelab

Default deny applies to ingress and egress for every pod, including system workloads, and for the nodes themselves. Every allowed connection must identify its source, destination, protocol, destination port, and purpose; shared infrastructure exceptions must be explicit. This wider boundary adds bootstrap and recovery complexity but avoids leaving nodes or system workloads implicitly trusted.

Only trusted administration and deployment identities may modify network policies or security-relevant workload identities. Application identities must not be able to grant themselves network access. Enforcement mechanisms, bootstrap allowances, and rollout are still being designed; this decision records the agreed boundaries, not an implemented guarantee.

Nodes must have a minimal persistent allowlist for administration and cluster bootstrap before Cilium starts, including recovery access independent of Kubernetes and Cilium. DNS, Kubernetes API access, monitoring, and health probes require explicit grants for each workload that needs them.

Host networking is an explicit exception restricted to trusted infrastructure deployments. Each exception must document its required traffic and how node policy enforces it.

Cilium and node firewalls enforce permitted network paths; Istio ambient enforces authenticated service identities and application permissions. Mesh network grants identify permitted endpoints and tunnel ports rather than allowing the tunnel port cluster-wide. Both layers default deny.

Outbound internet access is denied at L4 by default. Workloads needing internet access must receive explicit access through an egress waypoint, with L4 enforcement blocking direct bypass. Start with a shared egress waypoint, with separate permissions for each workload identity and approved destination; split waypoints when a concrete isolation requirement warrants it.

Application access to LAN devices also defaults deny. Direct LAN exceptions identify the workload, device addresses, protocol, and destination ports.

Application connections fail closed if mesh enrollment, identity issuance, or authorization fails. Authenticated mesh traffic is required, with narrowly defined exceptions for bootstrap, probes, and non-mesh infrastructure. These decisions describe the target model; they do not assert that the current deployment enforces it.

Every ambient workload receives default-deny identity authorization at ztunnel, including workloads without waypoints. Where waypoint policies form part of a service's security boundary, backend authorization must require that waypoint path and prevent direct caller bypass.

Enforcement is staged to preserve recovery access. Before further homelab work, verification must demonstrate both allowed and denied connections, including direct internet bypass, unauthorized mesh callers, node access, and recovery with the mesh unavailable. Successful connectivity alone is insufficient evidence of default deny.

Admission restrictions prohibit ordinary applications from using privileged containers, host namespaces, network administration capabilities, or host mounts that could undermine enforcement. Explicitly trusted infrastructure may receive documented exceptions.

Git is authoritative for network grants, mesh policies, and security-relevant identity assignments, following the existing Ansible/Flux ownership boundaries. Emergency changes must be recorded and reconciled afterward; recovery may restore administrative access but must not silently open application traffic.

DNS grants permit queries only to designated cluster resolvers, which use explicitly approved upstreams. DNS-name filtering is a separate requirement: allowing unrestricted queries through an approved resolver does not prevent DNS exfiltration.

Internet traffic that cannot use an egress waypoint remains denied unless explicitly approved as an infrastructure exception or routed through a suitable controlled proxy. There is no general direct-internet fallback; exceptions specify source, destination, protocol, destination ports, and purpose.
