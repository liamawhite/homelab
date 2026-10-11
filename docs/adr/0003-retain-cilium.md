# Retain Cilium for cluster networking

Keep Cilium while rewriting K3s deployment. It already runs on all three active nodes, and the Nix server's existing configuration disables Flannel and kube-proxy for this arrangement. Replacing it would add a networking migration and require revisiting current policies; initialization must instead explicitly configure compatible K3s flags and bootstrap ordering.
