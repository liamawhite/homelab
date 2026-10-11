# Bootstrap with Ansible and hand off shared definitions to Flux

Ansible phase 2 installs kube-vip, Cilium, and Flux from checked-in definitions that Flux also consumes, then hands normal reconciliation to Flux. This resolves the dependency on networking during GitOps bootstrap without duplicating versions or configuration. Use stable resource/release identities, verify adoption, and retain an explicit external recovery path through direct node APIs; ordinary Ansible runs must not compete with Flux after handoff.
