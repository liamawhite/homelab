// The dev runner owns both servers and rebuilds the Go backend without CGO.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"time"
)

type process struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
}

func start(ctx context.Context, name string, args []string, env []string) (*process, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = env
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = 5 * time.Second
	if err := command.Start(); err != nil {
		return nil, err
	}
	p := &process{command: command, done: make(chan struct{})}
	go func() { p.err = command.Wait(); close(p.done) }()
	return p, nil
}
func (p *process) stop() {
	if p == nil {
		return
	}
	select {
	case <-p.done:
		return
	default:
	}
	_ = p.command.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		_ = p.command.Process.Kill()
		<-p.done
	}
}
func build(ctx context.Context) error {
	command := exec.CommandContext(ctx, "go", "build", "-o", "bin/melodical-dev", "./cmd/melodical")
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
func sources() (map[string]string, error) {
	out := map[string]string{}
	for _, root := range []string{"internal", "cmd/melodical", "web/embed.go", "go.mod", "go.sum"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			out[path] = fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func setting(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	address := setting("MELODICAL_ADDR", "127.0.0.1:8080")
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("MELODICAL_ADDR: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	frontendPort, err := strconv.Atoi(setting("MELODICAL_DEV_PORT", "5173"))
	if err != nil || frontendPort < 1 || frontendPort > 65535 {
		return fmt.Errorf("MELODICAL_DEV_PORT must be a valid port")
	}
	for {
		probe, listenErr := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(frontendPort)))
		if listenErr == nil {
			probe.Close()
			break
		}
		if os.Getenv("MELODICAL_DEV_PORT") != "" || frontendPort >= 5273 {
			return fmt.Errorf("frontend port unavailable: %w", listenErr)
		}
		frontendPort++
	}
	proxy := "http://" + net.JoinHostPort(host, port)
	env := append(os.Environ(), "CGO_ENABLED=0", "MELODICAL_ADDR="+address, "MELODICAL_API_URL="+proxy)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("backend address %s is unavailable: %w", address, err)
	}
	listener.Close()
	baseline, err := sources()
	if err != nil {
		return err
	}
	if err = build(ctx); err != nil {
		return err
	}
	backend, err := start(ctx, "./bin/melodical-dev", nil, env)
	if err != nil {
		return err
	}
	defer func() { backend.stop() }()
	frontend, err := start(ctx, "node", []string{"web/node_modules/vite/bin/vite.js", "--config", "web/vite.config.ts", "--host", "127.0.0.1", "--port", strconv.Itoa(frontendPort), "--strictPort", "web"}, env)
	if err != nil {
		return err
	}
	defer frontend.stop()
	fmt.Printf("Melodical dev: http://localhost:%d (React hot reload, Go rebuilds; Ctrl-C stops both)\n", frontendPort)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-frontend.done:
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("frontend stopped: %v", frontend.err)
		case <-backend.done:
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("backend stopped: %v", backend.err)
		case <-ticker.C:
			current, err := sources()
			if err != nil {
				fmt.Fprintln(os.Stderr, "watch:", err)
				continue
			}
			if reflect.DeepEqual(current, baseline) {
				continue
			}
			baseline = current
			fmt.Println("Backend sources changed; rebuilding…")
			if err = build(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				fmt.Fprintln(os.Stderr, "Build failed; keeping the previous server. Fix the error and save to retry.")
				continue
			}
			backend.stop()
			backend, err = start(ctx, "./bin/melodical-dev", nil, env)
			if err != nil {
				return err
			}
			fmt.Println("Backend restarted.")
		}
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
