package kubernetes

import (
	"strings"

	"github.com/liamawhite/homelab/pkg/remote"
)

func Init(client *remote.Client) error {
	command := []string{"curl -sfL https://get.k3s.io | sh -s - server"}
	command = append(command, "--cluster-init")
	command = append(command, "--tls-san", client.Address())
	command = append(command, "--tls-san", "kube.local")
	command = append(command, "--disable=traefik")
	command = append(command, "--disable=servicelb")
	command = append(command, "--disable=local-storage")

	return client.RunCommand(strings.Join(command, " "))
}
