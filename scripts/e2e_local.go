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

func pickTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func pickUDPPort() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port, nil
}

func waitTCPPort(addr string, timeout time.Duration) error {
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

func startTCPEcho(addr string) (func(), error) {
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
	return func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	}, nil
}

func startUDPEcho(addr string) (func(), error) {
	u, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", u)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64*1024)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			_, _ = conn.WriteToUDP(buf[:n], remote)
		}
	}()
	return func() {
		cancel()
		_ = conn.Close()
		wg.Wait()
	}, nil
}

func roundtripTCP(addr string, payload []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
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

func roundtripUDP(addr string, payload []byte) error {
	r, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, r)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(payload); err != nil {
		return err
	}
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, buf[:n]) {
		return fmt.Errorf("response mismatch")
	}
	return nil
}

func runCase(enableWS bool, protocol string) error {
	serverPort, err := pickTCPPort()
	if err != nil {
		return err
	}
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	var backendAddr, clientAddr string
	if protocol == "udp" {
		bp, err := pickUDPPort()
		if err != nil {
			return err
		}
		cp, err := pickUDPPort()
		if err != nil {
			return err
		}
		backendAddr = fmt.Sprintf("127.0.0.1:%d", bp)
		clientAddr = fmt.Sprintf("127.0.0.1:%d", cp)
	} else {
		bp, err := pickTCPPort()
		if err != nil {
			return err
		}
		cp, err := pickTCPPort()
		if err != nil {
			return err
		}
		backendAddr = fmt.Sprintf("127.0.0.1:%d", bp)
		clientAddr = fmt.Sprintf("127.0.0.1:%d", cp)
	}

	var stopEcho func()
	if protocol == "udp" {
		stopEcho, err = startUDPEcho(backendAddr)
	} else {
		stopEcho, err = startTCPEcho(backendAddr)
	}
	if err != nil {
		return err
	}
	defer stopEcho()

	wsCfg := transport.DefaultWSConfig()
	wsCfg.Path = "/ws"

	srvCfg := serverpkg.Config{
		ListenAddr:   serverAddr,
		TargetAddr:   backendAddr,
		Protocol:     protocol,
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
	if err := waitTCPPort(serverAddr, 5*time.Second); err != nil {
		return err
	}

	cliCfg := clientpkg.Config{
		ListenAddr:   clientAddr,
		ServerAddr:   serverAddr,
		TargetAddr:   "",
		Protocol:     protocol,
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

	if protocol == "udp" {
		time.Sleep(300 * time.Millisecond)
	} else {
		if err := waitTCPPort(clientAddr, 5*time.Second); err != nil {
			return err
		}
	}

	payload := []byte("C2_Tunnel_E2E_Payload_0123456789")
	if protocol == "udp" {
		if err := roundtripUDP(clientAddr, payload); err != nil {
			return err
		}
	} else {
		if err := roundtripTCP(clientAddr, payload); err != nil {
			return err
		}
	}

	_ = cli.Stop()
	if !enableWS {
		_ = srv.Stop()
	}
	select {
	case <-clientErr:
	case <-time.After(2 * time.Second):
	}
	if !enableWS {
		select {
		case <-serverErr:
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

func main() {
	cases := []struct {
		name     string
		enableWS bool
		protocol string
	}{
		{"TCP over TCP", false, "tcp"},
		{"TCP over WS", true, "tcp"},
		{"UDP over TCP", false, "udp"},
		{"UDP over WS", true, "udp"},
	}

	for _, c := range cases {
		fmt.Printf("[E2E] %s start\n", c.name)
		if err := runCase(c.enableWS, c.protocol); err != nil {
			fmt.Printf("[E2E] %s failed: %v\n", c.name, err)
			os.Exit(1)
		}
		fmt.Printf("[E2E] %s passed\n", c.name)
	}
	fmt.Println("[E2E] all cases passed")
}
