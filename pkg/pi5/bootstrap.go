package pi5

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/liamawhite/homelab/pkg/remote"
)

//go:embed config.txt
var config []byte

//go:embed eeprom.conf
var eeprom []byte

func Bootstrap(client *remote.Client) error {
	fmt.Println("bootstrapping Raspberry Pi 5")

	fmt.Println("copying config.txt")
	if err := client.WriteRemoteFile(config, "/boot/firmware/config.txt"); err != nil {
		return err
	}

	fmt.Println("configuring boot order")
	tmp, err := client.WriteRemoteTempFile(eeprom)
	if err != nil {
		return err
	}
	if err := client.RunCommand(fmt.Sprintf("sudo rpi-eeprom-config --apply %s", tmp)); err != nil {
		return err
	}

	fmt.Println("ensuring cgroups are enabled")
	curr, err := client.ReadRemoteFile("/boot/firmware/cmdline.txt")
	if err != nil {
		return err
	}
	if !bytes.Contains(curr, []byte("cgroup_memory=1")) {
		curr = append(curr, []byte(" cgroup_memory=1")...)
	}
	if !bytes.Contains(curr, []byte("cgroup_enable=memory")) {
		curr = append(curr, []byte(" cgroup_enable=memory")...)
	}
	if err := client.WriteRemoteFile(curr, "/boot/firmware/cmdline.txt"); err != nil {
		return err
	}

	fmt.Println("rebooting")
	if err := client.Reboot(); err != nil {
		return err
	}

	fmt.Println("raspberry pi 5 bootstrapped successfully")

	return nil
}
