package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tunnel/pkg/crypto"
	"tunnel/pkg/transport"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

type Config struct {
	ListenAddr   string
	ServerAddr   string
	TargetAddr   string
	Protocol     string
	Password     string
	EnableHTTPS  bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	EnableWS bool
	WSConfig transport.WSConfig
}

type Client struct {
	config   Config
	cipher   *crypto.AESCipher
	ln       net.Listener
	udpConn  *net.UDPConn
	wsClient *transport.WSClient
}

func New(config Config) (*Client, error) {
	cipher, err := crypto.NewAESCipher(config.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	client := &Client{
		config: config,
		cipher: cipher,
	}

	if config.EnableWS {
		client.wsClient = transport.NewWSClient(config.WSConfig, cipher)
	}

	return client, nil
}

func (c *Client) Start() error {
	switch normalizeProtocol(c.config.Protocol) {
	case "tcp":
		return c.startTCP()
	case "udp":
		if c.config.EnableHTTPS {
			return fmt.Errorf("https mode is not supported with udp protocol")
		}
		return c.startUDP()
	default:
		return fmt.Errorf("unsupported protocol: %s", c.config.Protocol)
	}
}

func (c *Client) startTCP() error {
	ln, err := net.Listen("tcp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	c.ln = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			continue
		}

		go c.handleConnection(conn)
	}
}

func (c *Client) Stop() error {
	if c.udpConn != nil {
		return c.udpConn.Close()
	}
	if c.ln != nil {
		return c.ln.Close()
	}
	return nil
}

func (c *Client) handleConnection(ownerConn net.Conn) {
	defer ownerConn.Close()

	var targetAddr string
	var initialData []byte

	if c.config.EnableHTTPS {
		target, data, err := c.handleHTTPSConnect(ownerConn)
		if err != nil {
			return
		}
		targetAddr = target
		initialData = data
	} else {
		if c.config.TargetAddr == "" {
			targetAddr = "USE_DEFAULT"
		} else {
			targetAddr = c.config.TargetAddr
		}
	}

	if c.config.EnableWS {
		c.handleWSConnection(ownerConn, targetAddr, initialData)
	} else {
		c.handleTCPConnection(ownerConn, targetAddr, initialData)
	}
}

func (c *Client) handleWSConnection(ownerConn net.Conn, targetAddr string, initialData []byte) {
	wsConn, err := c.wsClient.Connect(c.config.ServerAddr)
	if err != nil {
		return
	}
	defer wsConn.Close()

	tunnelTarget := targetAddr
	if normalizeProtocol(c.config.Protocol) == "udp" {
		tunnelTarget = "UDP:" + targetAddr
	}

	if err := wsConn.WriteEncrypted([]byte(tunnelTarget)); err != nil {
		return
	}

	response, err := wsConn.ReadEncrypted()
	if err != nil {
		return
	}

	if !strings.HasPrefix(string(response), "OK") {
		return
	}

	if len(initialData) > 0 {
		if err := wsConn.WriteEncrypted(initialData); err != nil {
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = ownerConn.Close()
			_ = wsConn.Close()
		})
	}

	go func() {
		defer wg.Done()
		defer closeBoth()
		bufPtr := bufferPool.Get().(*[]byte)
		buf := *bufPtr
		defer bufferPool.Put(bufPtr)
		for {
			n, err := ownerConn.Read(buf)
			if err != nil {
				return
			}
			if err := wsConn.WriteEncrypted(buf[:n]); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer closeBoth()
		for {
			data, err := wsConn.ReadEncrypted()
			if err != nil {
				return
			}
			if _, err := ownerConn.Write(data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func (c *Client) handleTCPConnection(ownerConn net.Conn, targetAddr string, initialData []byte) {
	dialer := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	serverConn, err := dialer.Dial("tcp", c.config.ServerAddr)
	if err != nil {
		return
	}
	defer serverConn.Close()

	if tcpConn, ok := serverConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	cryptoConn := crypto.NewCryptoConn(serverConn, c.cipher)

	tunnelTarget := targetAddr
	if normalizeProtocol(c.config.Protocol) == "udp" {
		tunnelTarget = "UDP:" + targetAddr
	}

	if err := cryptoConn.WriteEncrypted([]byte(tunnelTarget)); err != nil {
		return
	}

	response, err := cryptoConn.ReadEncrypted()
	if err != nil {
		return
	}

	if !strings.HasPrefix(string(response), "OK") {
		return
	}

	if len(initialData) > 0 {
		if err := cryptoConn.WriteEncrypted(initialData); err != nil {
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = ownerConn.Close()
			_ = serverConn.Close()
		})
	}

	go func() {
		defer wg.Done()
		defer closeBoth()
		c.forwardToServer(ownerConn, cryptoConn)
	}()

	go func() {
		defer wg.Done()
		defer closeBoth()
		c.forwardFromServer(cryptoConn, ownerConn)
	}()

	wg.Wait()
}

func (c *Client) handleHTTPSConnect(conn net.Conn) (string, []byte, error) {
	reader := bufio.NewReader(conn)

	req, err := http.ReadRequest(reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read HTTP request: %w", err)
	}

	var targetAddr string
	var initialData []byte

	if req.Method == "CONNECT" {
		targetAddr = req.Host
		if !strings.Contains(targetAddr, ":") {
			targetAddr += ":443"
		}

		response := "HTTP/1.1 200 Connection Established\r\n\r\n"
		if _, err := conn.Write([]byte(response)); err != nil {
			return "", nil, fmt.Errorf("failed to send CONNECT response: %w", err)
		}
	} else {
		targetAddr = req.Host
		if !strings.Contains(targetAddr, ":") {
			targetAddr += ":80"
		}

		var buf bytes.Buffer
		req.Write(&buf)
		initialData = buf.Bytes()
	}

	return targetAddr, initialData, nil
}

func (c *Client) forwardToServer(src net.Conn, dst *crypto.CryptoConn) {
	defer func() {
		if tcpConn, ok := dst.Conn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer bufferPool.Put(bufPtr)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}

		if err := dst.WriteEncrypted(buf[:n]); err != nil {
			return
		}
	}
}

func (c *Client) forwardFromServer(src *crypto.CryptoConn, dst net.Conn) {
	defer func() {
		if tcpConn, ok := dst.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	for {
		data, err := src.ReadEncrypted()
		if err != nil {
			return
		}

		if _, err := dst.Write(data); err != nil {
			return
		}
	}
}

type udpSession struct {
	remote    *net.UDPAddr
	sendCh    chan []byte
	done      chan struct{}
	onClose   func()
	closeOnce sync.Once
	lastSeen  int64
}

func (s *udpSession) touch() {
	atomic.StoreInt64(&s.lastSeen, time.Now().UnixNano())
}

func (s *udpSession) expired(d time.Duration) bool {
	last := atomic.LoadInt64(&s.lastSeen)
	return time.Since(time.Unix(0, last)) > d
}

func (s *udpSession) send(payload []byte) bool {
	select {
	case <-s.done:
		return false
	default:
	}
	select {
	case s.sendCh <- payload:
		return true
	case <-s.done:
		return false
	default:
		return false
	}
}

func (s *udpSession) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		close(s.sendCh)
		if s.onClose != nil {
			s.onClose()
		}
	})
}

func normalizeProtocol(v string) string {
	p := strings.ToLower(strings.TrimSpace(v))
	if p == "" {
		return "tcp"
	}
	return p
}
