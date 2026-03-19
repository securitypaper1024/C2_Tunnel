package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"tunnel/pkg/acl"
	"tunnel/pkg/crypto"
	"tunnel/pkg/logger"
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
	TargetAddr   string
	Protocol     string
	Password     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	EnableWS bool
	WSConfig transport.WSConfig

	ACLConfig acl.Config
}

type Server struct {
	config Config
	cipher *crypto.AESCipher
	ln     net.Listener
	acl    *acl.ACL
}

func New(config Config) (*Server, error) {
	cipher, err := crypto.NewAESCipher(config.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	accessControl, err := acl.New(config.ACLConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACL: %w", err)
	}

	return &Server{
		config: config,
		cipher: cipher,
		acl:    accessControl,
	}, nil
}

func (s *Server) Start() error {
	if s.config.EnableWS {
		return s.startWebSocket()
	}
	return s.startTCP()
}

func (s *Server) startWebSocket() error {
	logger.Printf("[Server] WebSocket mode start")
	logger.Printf("[Server] target addr: %s", s.config.TargetAddr)

	wsServer := transport.NewWSServer(s.config.WSConfig, s.cipher, s.handleWSConnection)

	originalHandler := wsServer
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		if !s.acl.IsAllowed(clientIP) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		originalHandler.ServeHTTP(w, r)
	})

	httpServer := &http.Server{Addr: s.config.ListenAddr, Handler: wrappedHandler}
	if s.config.WSConfig.EnableTLS {
		logger.Printf("[Server] listen with TLS: %s%s", s.config.ListenAddr, s.config.WSConfig.Path)
		return httpServer.ListenAndServeTLS(s.config.WSConfig.TLSCert, s.config.WSConfig.TLSKey)
	}

	logger.Printf("[Server] listen: ws://%s%s", s.config.ListenAddr, s.config.WSConfig.Path)
	return httpServer.ListenAndServe()
}

func (s *Server) startTCP() error {
	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.ln = ln

	logger.Printf("[Server] TCP mode start, listen: %s", s.config.ListenAddr)
	logger.Printf("[Server] target addr: %s", s.config.TargetAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			logger.Printf("[Server] accept error: %v", err)
			continue
		}

		if !s.acl.IsAllowed(conn.RemoteAddr().String()) {
			_ = conn.Close()
			continue
		}

		go s.handleTCPConnection(conn)
	}
}

func (s *Server) Stop() error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

func (s *Server) handleWSConnection(wsConn *transport.WSConn) {
	defer wsConn.Close()
	clientAddr := wsConn.RemoteAddr().String()
	logger.Printf("[Server] new WS connection: %s", clientAddr)

	targetAddr, targetProtocol, err := s.readTargetFromWS(wsConn)
	if err != nil {
		logger.Printf("[Server] read target failed: %v", err)
		return
	}

	targetConn, err := s.dialTarget(targetProtocol, targetAddr)
	if err != nil {
		logger.Printf("[Server] dial target failed: %v", err)
		_ = wsConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	if err := wsConn.WriteEncrypted([]byte("OK")); err != nil {
		logger.Printf("[Server] send response failed: %v", err)
		return
	}

	logger.Printf("[Server] WS tunnel established: %s <-> %s (%s)", clientAddr, targetAddr, targetProtocol)
	if targetProtocol == "udp" {
		s.bridgeWSUDP(wsConn, targetConn)
	} else {
		transport.BridgeWSToTCP(wsConn, targetConn)
	}
	logger.Printf("[Server] WS connection closed: %s", clientAddr)
}

func (s *Server) handleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientAddr := clientConn.RemoteAddr().String()
	logger.Printf("[Server] new TCP connection: %s", clientAddr)

	cryptoConn := crypto.NewCryptoConn(clientConn, s.cipher)
	targetAddr, targetProtocol, err := s.readTargetFromCrypto(cryptoConn)
	if err != nil {
		logger.Printf("[Server] read target failed: %v", err)
		return
	}

	targetConn, err := s.dialTarget(targetProtocol, targetAddr)
	if err != nil {
		logger.Printf("[Server] dial target failed: %v", err)
		_ = cryptoConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	if err := cryptoConn.WriteEncrypted([]byte("OK")); err != nil {
		logger.Printf("[Server] send response failed: %v", err)
		return
	}

	logger.Printf("[Server] TCP tunnel established: %s <-> %s (%s)", clientAddr, targetAddr, targetProtocol)
	if targetProtocol == "udp" {
		s.bridgeCryptoUDP(cryptoConn, targetConn)
	} else {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			s.forwardFromClient(cryptoConn, targetConn)
		}()

		go func() {
			defer wg.Done()
			s.forwardToClient(targetConn, cryptoConn)
		}()

		wg.Wait()
	}
	logger.Printf("[Server] TCP connection closed: %s", clientAddr)
}

func (s *Server) readTargetFromWS(wsConn *transport.WSConn) (string, string, error) {
	data, err := wsConn.ReadEncrypted()
	if err != nil {
		return "", "", err
	}
	targetAddr := string(data)
	protocol := normalizeProtocol(s.config.Protocol)
	if strings.HasPrefix(targetAddr, "UDP:") {
		protocol = "udp"
		targetAddr = strings.TrimPrefix(targetAddr, "UDP:")
	}
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}
	if targetAddr == "" {
		return "", "", fmt.Errorf("empty target addr")
	}
	return targetAddr, protocol, nil
}

func (s *Server) readTargetFromCrypto(cryptoConn *crypto.CryptoConn) (string, string, error) {
	data, err := cryptoConn.ReadEncrypted()
	if err != nil {
		return "", "", err
	}
	targetAddr := string(data)
	protocol := normalizeProtocol(s.config.Protocol)
	if strings.HasPrefix(targetAddr, "UDP:") {
		protocol = "udp"
		targetAddr = strings.TrimPrefix(targetAddr, "UDP:")
	}
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}
	if targetAddr == "" {
		return "", "", fmt.Errorf("empty target addr")
	}
	return targetAddr, protocol, nil
}

func (s *Server) dialTarget(protocol, targetAddr string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := dialer.Dial(protocol, targetAddr)
	if err != nil {
		return nil, err
	}
	if protocol == "tcp" {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetNoDelay(true)
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(30 * time.Second)
		}
	}
	return conn, nil
}

func (s *Server) bridgeCryptoUDP(src *crypto.CryptoConn, udpConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = src.Close()
			_ = udpConn.Close()
		})
	}

	go func() {
		defer wg.Done()
		defer closeBoth()
		for {
			data, err := src.ReadEncrypted()
			if err != nil {
				return
			}
			if _, err := udpConn.Write(data); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer closeBoth()
		buf := make([]byte, 64*1024)
		for {
			n, err := udpConn.Read(buf)
			if err != nil {
				return
			}
			if err := src.WriteEncrypted(buf[:n]); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func (s *Server) bridgeWSUDP(wsConn *transport.WSConn, udpConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = wsConn.Close()
			_ = udpConn.Close()
		})
	}

	go func() {
		defer wg.Done()
		defer closeBoth()
		for {
			data, err := wsConn.ReadEncrypted()
			if err != nil {
				return
			}
			if _, err := udpConn.Write(data); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer closeBoth()
		buf := make([]byte, 64*1024)
		for {
			n, err := udpConn.Read(buf)
			if err != nil {
				return
			}
			if err := wsConn.WriteEncrypted(buf[:n]); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func (s *Server) forwardFromClient(src *crypto.CryptoConn, dst net.Conn) {
	defer func() {
		if tcpConn, ok := dst.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()
	for {
		data, err := src.ReadEncrypted()
		if err != nil {
			if err != io.EOF {
				logger.Printf("[Server] read client data error: %v", err)
			}
			return
		}

		if _, err := dst.Write(data); err != nil {
			logger.Printf("[Server] write target data error: %v", err)
			return
		}
	}
}

func (s *Server) forwardToClient(src net.Conn, dst *crypto.CryptoConn) {
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
			if err != io.EOF {
				logger.Printf("[Server] read target data error: %v", err)
			}
			return
		}

		if err := dst.WriteEncrypted(buf[:n]); err != nil {
			logger.Printf("[Server] write client data error: %v", err)
			return
		}
	}
}

func (s *Server) GetACL() *acl.ACL {
	return s.acl
}

func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func normalizeProtocol(v string) string {
	p := strings.ToLower(strings.TrimSpace(v))
	if p == "" {
		return "tcp"
	}
	return p
}
