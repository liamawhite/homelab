// Package probe checks host reachability and MAC address by forcing ARP
// resolution and reading the OS neighbor table — no external commands.
package probe

import (
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	timeout   = 2 * time.Second
	pollEvery = 100 * time.Millisecond
)

// Result holds the reachability and MAC address for a single host.
type Result struct {
	Reachable bool
	MAC       string
}

// Host forces ARP resolution for address and polls the OS neighbor table
// until it resolves to a MAC address or timeout elapses.
func Host(address string) Result {
	triggerARP(address)

	deadline := time.Now().Add(timeout)
	for {
		mac, complete := lookupNeighbor(address)
		if complete {
			return Result{Reachable: true, MAC: mac}
		}
		if time.Now().After(deadline) {
			return Result{Reachable: false, MAC: mac}
		}
		time.Sleep(pollEvery)
	}
}

// Table returns the OS neighbor table as a map of IP address to MAC
// address, covering every host currently resolved in the local ARP/neighbor
// cache — not just ones queried via Host.
func Table() map[string]string {
	return dumpTable()
}

// SSHResult holds the outcome of an authenticated SSH check.
type SSHResult struct {
	Authenticated bool
	K3sInstalled  bool
}

// SSH attempts a single SSH handshake and authentication against address,
// and if it succeeds, also checks whether k3s is installed there.
func SSH(address, user, password string) SSHResult {
	client, err := dialSSH(address, user, password)
	if err != nil {
		return SSHResult{}
	}
	defer client.Close()

	result := SSHResult{Authenticated: true}

	session, err := client.NewSession()
	if err == nil {
		defer session.Close()
		result.K3sInstalled = session.Run("command -v k3s >/dev/null 2>&1") == nil
	}

	return result
}

func dialSSH(address, user, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	return ssh.Dial("tcp", net.JoinHostPort(address, "22"), config)
}

// triggerARP sends a harmless UDP datagram so the kernel resolves the
// neighbor's link-layer address and populates the local ARP table.
func triggerARP(address string) {
	conn, err := net.DialTimeout("udp4", net.JoinHostPort(address, "9"), timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte{0})
}
