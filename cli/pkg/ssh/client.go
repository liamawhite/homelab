package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	Host       string
	Port       int
	User       string
	AuthMethod ssh.AuthMethod
	client     *ssh.Client
}

// NewClient creates a new SSH client with key authentication
func NewClient(host, user, keyPath string) (*Client, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Client{
		Host:       host,
		Port:       22,
		User:       user,
		AuthMethod: ssh.PublicKeys(signer),
	}, nil
}

// NewClientWithPassword creates a new SSH client with password authentication
func NewClientWithPassword(host, user, password string) *Client {
	slog.Debug("Creating SSH client with password authentication", "host", host, "user", user, "password_length", len(password))
	return &Client{
		Host:       host,
		Port:       22,
		User:       user,
		AuthMethod: ssh.Password(password),
	}
}

// Connect establishes an SSH connection
func (c *Client) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	// First check if we can reach the host at all
	slog.Debug("Checking network connectivity", "address", addr)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		slog.Error("Failed to reach host", "address", addr, "error", err.Error())
		return fmt.Errorf("cannot reach host %s: %w", addr, err)
	}
	conn.Close()
	slog.Info("Host is reachable", "address", addr)

	// Create a host key callback that logs but accepts all keys
	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		slog.Debug("Accepting host key", "hostname", hostname, "key_type", key.Type(), "fingerprint", ssh.FingerprintSHA256(key))
		return nil
	}

	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            []ssh.AuthMethod{c.AuthMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	slog.Info("Attempting SSH connection", "address", addr, "user", c.User, "timeout", "10s", "host_key_verification", "disabled")

	// Retry with exponential backoff
	for i := 0; i < 3; i++ {
		slog.Debug("SSH connection attempt", "attempt", i+1, "max_attempts", 3)

		c.client, err = ssh.Dial("tcp", addr, config)
		if err == nil {
			slog.Info("SSH connection successful", "address", addr, "user", c.User)
			return nil
		}

		slog.Warn("SSH connection attempt failed", "attempt", i+1, "error", err.Error())

		if i < 2 {
			delay := time.Duration(i+1) * 2 * time.Second
			slog.Debug("Retrying SSH connection", "delay", delay.String())
			time.Sleep(delay)
		}
	}

	slog.Error("SSH connection failed after all attempts", "address", addr, "user", c.User, "final_error", err.Error())

	return fmt.Errorf("failed to connect after 3 attempts: %w", err)
}

// Execute runs a command over SSH
func (c *Client) Execute(cmd string) (stdout, stderr string, err error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf []byte
	stdoutBuf, err = session.CombinedOutput(cmd)
	if err != nil {
		slog.Error("Command execution failed", "error", err.Error(), "output", string(stdoutBuf))
		return string(stdoutBuf), "", fmt.Errorf("command failed: %w", err)
	}

	return string(stdoutBuf), string(stderrBuf), nil
}

// ExecuteSudo runs a command with sudo
func (c *Client) ExecuteSudo(cmd string) (stdout, stderr string, err error) {
	return c.Execute("sudo " + cmd)
}

// WriteFile writes content to a file via SSH
func (c *Client) WriteFile(path, content string, useSudo bool) error {
	// Use heredoc to safely handle any content including quotes and special characters
	var cmd string
	if useSudo {
		cmd = fmt.Sprintf("sudo tee %s > /dev/null <<'EOFMARKER'\n%s\nEOFMARKER", path, content)
	} else {
		cmd = fmt.Sprintf("cat > %s <<'EOFMARKER'\n%s\nEOFMARKER", path, content)
	}

	stdout, _, err := c.Execute(cmd)
	if err != nil {
		slog.Error("Failed to write file", "path", path, "error", err.Error(), "output", stdout)
	}
	return err
}

// ReadFile reads a file via SSH
func (c *Client) ReadFile(path string, useSudo bool) (string, error) {
	var cmd string
	if useSudo {
		cmd = fmt.Sprintf("sudo cat %s", path)
	} else {
		cmd = fmt.Sprintf("cat %s", path)
	}

	stdout, _, err := c.Execute(cmd)
	return stdout, err
}

// Reboot reboots the remote machine
func (c *Client) Reboot() error {
	_, _, err := c.ExecuteSudo("reboot")
	// Ignore connection errors after reboot command
	if err != nil && !isConnectionError(err) {
		return err
	}
	return nil
}

// WaitForReboot waits for the machine to come back online after reboot
func (c *Client) WaitForReboot(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	time.Sleep(5 * time.Second) // Initial delay for shutdown

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			// Give SSH daemon time to fully start
			time.Sleep(2 * time.Second)

			// Try to reconnect SSH
			err = c.Connect(context.Background())
			if err == nil {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for reboot")
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common connection error strings
	errStr := err.Error()
	return contains(errStr, "connection refused") ||
		contains(errStr, "connection reset") ||
		contains(errStr, "broken pipe") ||
		contains(errStr, "EOF")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(hasPrefix(s, substr) || hasSuffix(s, substr) || hasInfix(s, substr)))
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasInfix(s, infix string) bool {
	for i := 0; i <= len(s)-len(infix); i++ {
		if s[i:i+len(infix)] == infix {
			return true
		}
	}
	return false
}
