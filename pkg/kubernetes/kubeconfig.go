package kubernetes

import (
	"fmt"
	"os"

	"github.com/liamawhite/homelab/pkg/remote"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var kubeconfigPath = func() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "~/.kube/config"
	}
	return fmt.Sprintf("%s/.kube/config", dir)
}()

func RetrieveKubeConfig(client *remote.Client) error {
	data, err := client.ReadRemoteFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return err
	}
	config, err := clientcmd.Load(data)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Everything is named default... So fix that (and the server address) here.
	authInfo := config.AuthInfos["default"]
	context := config.Contexts["default"]
	context.AuthInfo = "homelab"
	context.Cluster = "homelab"
	cluster := config.Clusters["default"]
	cluster.Server = fmt.Sprintf("https://%s:6443", client.Address())

	// Now merge the kubeconfig with the local one
	var localConfig *api.Config
	curr, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read local kubeconfig: %w", err)
		}
		localConfig = api.NewConfig()
	} else {
		localConfig, err = clientcmd.Load(curr)
		if err != nil {
			return fmt.Errorf("failed to load local kubeconfig: %w", err)
		}
	}

	localConfig.AuthInfos["homelab"] = authInfo
	localConfig.Contexts["homelab"] = context
	localConfig.Clusters["homelab"] = cluster
	localConfig.CurrentContext = "homelab"

	fmt.Println("Adding kubeconfig to ", kubeconfigPath)

	content, err := clientcmd.Write(*localConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize kubeconfig: %w", err)
	}

	if err := os.WriteFile(kubeconfigPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %w", err)
	}
	return nil
}
