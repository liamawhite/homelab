# homelab

## Troubleshooting

### `no IP addresses available in range set`

`ssh` into the node and run `sudo rm -rf /var/lib/cni/networks/cbr0 && sudo reboot`. See [issue](https://github.com/k3s-io/k3s/issues/4682).
