package client

import (
	"fmt"
	"net"
	"sync"
	"time"

	"tunnel/pkg/crypto"
)

func (c *Client) startUDP() error {
	addr, err := net.ResolveUDPAddr("udp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve udp listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen udp: %w", err)
	}
	c.udpConn = conn

	targetAddr := c.config.TargetAddr
	if targetAddr == "" {
		targetAddr = "USE_DEFAULT"
	}

	sessions := make(map[string]*udpSession)
	var mu sync.Mutex

	removeSession := func(key string, s *udpSession) {
		mu.Lock()
		defer mu.Unlock()
		if cur, ok := sessions[key]; ok && cur == s {
			delete(sessions, key)
		}
	}

	createSession := func(remote *net.UDPAddr) (*udpSession, error) {
		key := remote.String()
		if c.config.EnableWS {
			return c.createUDPSessionWS(conn, remote, key, targetAddr, removeSession)
		}
		return c.createUDPSessionTCP(conn, remote, key, targetAddr, removeSession)
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for key, s := range sessions {
				if s.expired(2 * time.Minute) {
					delete(sessions, key)
					s.close()
				}
			}
			mu.Unlock()
		}
	}()

	buf := make([]byte, 64*1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(*net.OpError); ok && !ne.Timeout() {
				return nil
			}
			return nil
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])

		key := remote.String()
		mu.Lock()
		sess := sessions[key]
		mu.Unlock()

		if sess == nil {
			s, err := createSession(remote)
			if err != nil {
				continue
			}
			mu.Lock()
			sessions[key] = s
			mu.Unlock()
			sess = s
		}

		sess.touch()
		if !sess.send(payload) {
			sess.close()
		}
	}
}

func (c *Client) createUDPSessionTCP(
	udpConn *net.UDPConn,
	remote *net.UDPAddr,
	key string,
	targetAddr string,
	onGone func(string, *udpSession),
) (*udpSession, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	serverConn, err := dialer.Dial("tcp", c.config.ServerAddr)
	if err != nil {
		return nil, err
	}

	cryptoConn := crypto.NewCryptoConn(serverConn, c.cipher)
	if err := cryptoConn.WriteEncrypted([]byte("UDP:" + targetAddr)); err != nil {
		_ = serverConn.Close()
		return nil, err
	}
	resp, err := cryptoConn.ReadEncrypted()
	if err != nil || string(resp) != "OK" {
		_ = serverConn.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("udp tunnel rejected: %s", string(resp))
	}

	s := &udpSession{
		remote: remote,
		sendCh: make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	s.touch()
	s.onClose = func() {
		onGone(key, s)
		_ = serverConn.Close()
	}

	go func() {
		defer s.close()
		for {
			select {
			case <-s.done:
				return
			case data, ok := <-s.sendCh:
				if !ok {
					return
				}
				if err := cryptoConn.WriteEncrypted(data); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer s.close()
		for {
			data, err := cryptoConn.ReadEncrypted()
			if err != nil {
				return
			}
			s.touch()
			if _, err := udpConn.WriteToUDP(data, remote); err != nil {
				return
			}
		}
	}()

	return s, nil
}

func (c *Client) createUDPSessionWS(
	udpConn *net.UDPConn,
	remote *net.UDPAddr,
	key string,
	targetAddr string,
	onGone func(string, *udpSession),
) (*udpSession, error) {
	wsConn, err := c.wsClient.Connect(c.config.ServerAddr)
	if err != nil {
		return nil, err
	}

	if err := wsConn.WriteEncrypted([]byte("UDP:" + targetAddr)); err != nil {
		_ = wsConn.Close()
		return nil, err
	}
	resp, err := wsConn.ReadEncrypted()
	if err != nil || string(resp) != "OK" {
		_ = wsConn.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("udp ws tunnel rejected: %s", string(resp))
	}

	s := &udpSession{
		remote: remote,
		sendCh: make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	s.touch()
	s.onClose = func() {
		onGone(key, s)
		_ = wsConn.Close()
	}

	go func() {
		defer s.close()
		for {
			select {
			case <-s.done:
				return
			case data, ok := <-s.sendCh:
				if !ok {
					return
				}
				if err := wsConn.WriteEncrypted(data); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer s.close()
		for {
			data, err := wsConn.ReadEncrypted()
			if err != nil {
				return
			}
			s.touch()
			if _, err := udpConn.WriteToUDP(data, remote); err != nil {
				return
			}
		}
	}()

	return s, nil
}
