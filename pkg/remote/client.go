package remote

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Client struct {
	client   *ssh.Client
	username string
	address  string
	password string
}

func NewClient(destination string) (*Client, error) {
	// Split the destination into username and address
	splits := strings.Split(destination, "@")
	if len(splits) != 2 {
		return nil, fmt.Errorf("invalid destination: %s", destination)
	}
	username, address := splits[0], splits[1]

	// Prompt the user for a password
	fmt.Printf("Enter password for %s: ", destination)
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // Print a newline after user presses Enter
	if err != nil {
		return nil, err
	}

	return dial(username, address, string(bytePassword))
}

func dial(username, address, password string) (*Client, error) {
	// Create a new SSH client
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // insecure but probably fine for a homelab?
	}

	c, err := ssh.Dial("tcp", address+":22", config)
	if err != nil {
		return nil, err
	}
	return &Client{client: c, username: username, address: address, password: password}, nil
}

func (c *Client) Address() string {
    return c.address
}

func (c *Client) Reconnect() error {
	refreshed, err := dial(c.username, c.address, c.password)
	if err != nil {
		return err
	}
	c.client = refreshed.client
	return nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) RunCommand(command string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(command)
}

func (c *Client) Reboot() error {
	if err := c.RunCommand("sudo reboot"); err != nil {
		return err
	}

	// Wait for the machine to come back up
	fmt.Print("waiting for machine to come back up")
	count := 0
	for count < 120/5 {
		time.Sleep(5 * time.Second)
		fmt.Print(".")
		err := c.Reconnect()
		if err != nil {
			continue
		}
		err = c.RunCommand("true")
		if err == nil {
			fmt.Println()
			break
		}
		count++
		if count == 10 {
			fmt.Println()
			return fmt.Errorf("machine did not come back up")
		}
	}
	return nil
}

func (c *Client) ReadRemoteFile(path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute: %s", path)
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(fmt.Sprintf("sudo cat %s", path)); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (c *Client) WriteRemoteTempFile(content []byte) (string, error) {
	path := fmt.Sprintf("/tmp/%x", sha256.Sum256(content))
	if err := c.WriteRemoteFile(content, path); err != nil {
		return "", err
	}
	return path, nil
}

func (c *Client) WriteRemoteFile(content []byte, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}

	session, err := c.client.NewSession()
	if err != nil {
		return err
	}

	var combinedOutput bytes.Buffer
	multiWriter := io.MultiWriter(&combinedOutput)
	session.Stdout = multiWriter
	session.Stderr = multiWriter

	defer session.Close()

	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	wg := sync.WaitGroup{}
	wg.Add(1)
	errCh := make(chan error, 1)

	go func() {
		defer wg.Done()
		hostIn, err := session.StdinPipe()
		if err != nil {
			errCh <- fmt.Errorf("failed to create stdin pipe: %w", err)
			return
		}
		defer hostIn.Close()
		if _, err := fmt.Fprintf(hostIn, "C0644 %d %s\n", len(content), filename); err != nil {
			errCh <- fmt.Errorf("failed to send file metadata: %w", err)
			return
		}
		if _, err := io.Copy(hostIn, bytes.NewReader(content)); err != nil {
			errCh <- fmt.Errorf("failed to copy file content: %w", err)
			return
		}
		if _, err := fmt.Fprint(hostIn, "\x00"); err != nil {
			errCh <- fmt.Errorf("failed to send null byte: %w", err)
			return
		}
	}()

	if err := session.Run(fmt.Sprintf("sudo /usr/bin/scp -t %s", dir)); err != nil {
		fmt.Println(combinedOutput.String())
		return fmt.Errorf("failed to run scp command: %w", err)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil

}
