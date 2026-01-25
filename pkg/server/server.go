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
	logger.Printf("[Server] WebSocket 模式启动中...")
	logger.Printf("[Server] 目标地址: %s", s.config.TargetAddr)

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

	server := &http.Server{
		Addr:    s.config.ListenAddr,
		Handler: wrappedHandler,
	}

	if s.config.WSConfig.EnableTLS {
		logger.Printf("[Server] 启用 TLS，监听地址: %s%s", s.config.ListenAddr, s.config.WSConfig.Path)
		return server.ListenAndServeTLS(s.config.WSConfig.TLSCert, s.config.WSConfig.TLSKey)
	}

	logger.Printf("[Server] 启动，监听地址: ws://%s%s", s.config.ListenAddr, s.config.WSConfig.Path)
	return server.ListenAndServe()
}

func (s *Server) handleWSConnection(wsConn *transport.WSConn) {
	defer wsConn.Close()
	clientAddr := wsConn.RemoteAddr().String()
	logger.Printf("[Server] 新 WebSocket 连接: %s", clientAddr)

	targetData, err := wsConn.ReadEncrypted()
	if err != nil {
		logger.Printf("[Server] 读取目标地址失败: %v", err)
		return
	}

	targetAddr := string(targetData)
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}

	logger.Printf("[Server] 连接目标: %s", targetAddr)

	dialer := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	targetConn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		logger.Printf("[Server] 连接目标失败: %v", err)
		wsConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	if tcpConn, ok := targetConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	if err := wsConn.WriteEncrypted([]byte("OK")); err != nil {
		logger.Printf("[Server] 发送响应失败: %v", err)
		return
	}

	logger.Printf("[Server] WebSocket 隧道建立: %s <-> %s", clientAddr, targetAddr)

	transport.BridgeWSToTCP(wsConn, targetConn)

	logger.Printf("[Server] WebSocket 连接关闭: %s", clientAddr)
}

func (s *Server) startTCP() error {
	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.ln = ln

	logger.Printf("[Server] TCP 模式启动，监听地址: %s", s.config.ListenAddr)
	logger.Printf("[Server] 目标地址: %s", s.config.TargetAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			logger.Printf("[Server] Accept 错误: %v", err)
			continue
		}

		if !s.acl.IsAllowed(conn.RemoteAddr().String()) {
			conn.Close()
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

func (s *Server) handleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientAddr := clientConn.RemoteAddr().String()
	logger.Printf("[Server] 新 TCP 连接来自: %s", clientAddr)

	cryptoConn := crypto.NewCryptoConn(clientConn, s.cipher)

	targetData, err := cryptoConn.ReadEncrypted()
	if err != nil {
		logger.Printf("[Server] 读取目标地址失败: %v", err)
		return
	}

	targetAddr := string(targetData)
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}

	logger.Printf("[Server] 连接目标: %s", targetAddr)

	dialer := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	targetConn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		logger.Printf("[Server] 连接目标失败: %v", err)
		cryptoConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	if tcpConn, ok := targetConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	if err := cryptoConn.WriteEncrypted([]byte("OK")); err != nil {
		logger.Printf("[Server] 发送响应失败: %v", err)
		return
	}

	logger.Printf("[Server] TCP 隧道建立: %s <-> %s", clientAddr, targetAddr)

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
	logger.Printf("[Server] TCP 连接关闭: %s", clientAddr)
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
				logger.Printf("[Server] 读取客户端数据错误: %v", err)
			}
			return
		}

		if _, err := dst.Write(data); err != nil {
			logger.Printf("[Server] 写入目标数据错误: %v", err)
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
				logger.Printf("[Server] 读取目标数据错误: %v", err)
			}
			return
		}

		if err := dst.WriteEncrypted(buf[:n]); err != nil {
			logger.Printf("[Server] 写入客户端数据错误: %v", err)
			return
		}
	}
}

func (s *Server) GetACL() *acl.ACL {
	return s.acl
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
