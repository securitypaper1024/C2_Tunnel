package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	clientpkg "tunnel/pkg/client"
	serverpkg "tunnel/pkg/server"
	"tunnel/pkg/transport"
)

func pickPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return fmt.Errorf("port not ready: %s", addr)
}

func startEchoServer(addr string) (func(), error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	stop := func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	}
	return stop, nil
}

func roundtrip(addr string, payload []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	if _, err := conn.Write(payload); err != nil {
		return err
	}

	resp := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}

	if !bytes.Equal(payload, resp) {
		return fmt.Errorf("response mismatch")
	}
	return nil
}

func runCase(enableWS bool) error {
	backendPort, err := pickPort()
	if err != nil {
		return err
	}
	serverPort, err := pickPort()
	if err != nil {
		return err
	}
	clientPort, err := pickPort()
	if err != nil {
		return err
	}

	backendAddr := fmt.Sprintf("127.0.0.1:%d", backendPort)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)
	clientAddr := fmt.Sprintf("127.0.0.1:%d", clientPort)

	stopEcho, err := startEchoServer(backendAddr)
	if err != nil {
		return err
	}
	defer stopEcho()

	wsCfg := transport.DefaultWSConfig()
	wsCfg.Path = "/ws"

	srvCfg := serverpkg.Config{
		ListenAddr:   serverAddr,
		TargetAddr:   backendAddr,
		Password:     "E2EPass@2026",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableWS:     enableWS,
		WSConfig:     wsCfg,
	}

	srv, err := serverpkg.New(srvCfg)
	if err != nil {
		return err
	}

	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.Start() }()

	if err := waitPort(serverAddr, 5*time.Second); err != nil {
		return fmt.Errorf("server startup failed: %w", err)
	}

	cliCfg := clientpkg.Config{
		ListenAddr:   clientAddr,
		ServerAddr:   serverAddr,
		TargetAddr:   "",
		Password:     "E2EPass@2026",
		EnableHTTPS:  false,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableWS:     enableWS,
		WSConfig:     wsCfg,
	}

	cli, err := clientpkg.New(cliCfg)
	if err != nil {
		return err
	}

	clientErr := make(chan error, 1)
	go func() { clientErr <- cli.Start() }()

	if err := waitPort(clientAddr, 5*time.Second); err != nil {
		return fmt.Errorf("client startup failed: %w", err)
	}

	payload := []byte("C2_Tunnel_E2E_Payload_0123456789")
	if err := roundtrip(clientAddr, payload); err != nil {
		return err
	}

	_ = cli.Stop()
	if !enableWS {
		_ = srv.Stop()
	}

	select {
	case err := <-clientErr:
		if err != nil {
			return fmt.Errorf("client exit error: %w", err)
		}
	case <-time.After(2 * time.Second):
		if !enableWS {
			return fmt.Errorf("client did not stop in time")
		}
	}

	if !enableWS {
		select {
		case err := <-serverErr:
			if err != nil {
				return fmt.Errorf("server exit error: %w", err)
			}
		case <-time.After(2 * time.Second):
			return fmt.Errorf("server did not stop in time")
		}
	}

	return nil
}

func main() {
	fmt.Println("[E2E] TCP case start")
	if err := runCase(false); err != nil {
		fmt.Printf("[E2E] TCP case failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[E2E] TCP case passed")

	fmt.Println("[E2E] WS case start")
	if err := runCase(true); err != nil {
		fmt.Printf("[E2E] WS case failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[E2E] WS case passed")

	fmt.Println("[E2E] all cases passed")
}
