# homelab

## Troubleshooting

### `no IP addresses available in range set`

`ssh` into the node and run `sudo rm -rf /var/lib/cni/networks/cbr0 && sudo reboot`. See [issue](https://github.com/k3s-io/k3s/issues/4682).

### k3s node failes

Check `systemctl status k3s.service` and `journalctl -xeu k3s.service` for details.
