# Retain K3s for the deployment rewrite

Keep K3s on the Raspberry Pis and Nix server while rewriting deployment. K3s already runs successfully on the active hardware; switching to k0s or kubeadm would add migration work without resolving the Pi kernel and container-image compatibility constraints. OS changes remain a later milestone.
