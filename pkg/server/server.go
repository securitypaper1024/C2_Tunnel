package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"tunnel/pkg/crypto"
	"tunnel/pkg/transport"
)

// Config Server 配置
type Config struct {
	ListenAddr   string // 监听地址 (接收 Client 连接)
	TargetAddr   string // 目标地址 (CobaltStrike TeamServer)
	Password     string // 加密密码
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	
	// WebSocket 配置
	EnableWS     bool               // 是否启用 WebSocket
	WSConfig     transport.WSConfig // WebSocket 配置
}

// Server 隧道服务端
type Server struct {
	config Config
	cipher *crypto.AESCipher
	ln     net.Listener
}

// New 创建新的 Server
func New(config Config) (*Server, error) {
	cipher, err := crypto.NewAESCipher(config.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &Server{
		config: config,
		cipher: cipher,
	}, nil
}

// Start 启动服务
func (s *Server) Start() error {
	if s.config.EnableWS {
		return s.startWebSocket()
	}
	return s.startTCP()
}

// startWebSocket 启动 WebSocket 模式
func (s *Server) startWebSocket() error {
	log.Printf("[Server] 🌐 WebSocket 模式启动中...")
	log.Printf("[Server] 🎯 目标地址: %s", s.config.TargetAddr)

	wsServer := transport.NewWSServer(s.config.WSConfig, s.cipher, s.handleWSConnection)
	return wsServer.Start(s.config.ListenAddr)
}

// handleWSConnection 处理 WebSocket 连接
func (s *Server) handleWSConnection(wsConn *transport.WSConn) {
	defer wsConn.Close()
	clientAddr := wsConn.RemoteAddr().String()
	log.Printf("[Server] 📥 新 WebSocket 连接: %s", clientAddr)

	// 读取目标地址
	targetData, err := wsConn.ReadEncrypted()
	if err != nil {
		log.Printf("[Server] ❌ 读取目标地址失败: %v", err)
		return
	}

	targetAddr := string(targetData)
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}

	log.Printf("[Server] 🔗 连接目标: %s", targetAddr)

	// 连接目标服务器
	targetConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Server] ❌ 连接目标失败: %v", err)
		wsConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	// 发送成功响应
	if err := wsConn.WriteEncrypted([]byte("OK")); err != nil {
		log.Printf("[Server] ❌ 发送响应失败: %v", err)
		return
	}

	log.Printf("[Server] ✅ WebSocket 隧道建立成功: %s <-> %s", clientAddr, targetAddr)

	// 桥接 WebSocket 和 TCP
	transport.BridgeWSToTCP(wsConn, targetConn)

	log.Printf("[Server] 🔌 WebSocket 连接关闭: %s", clientAddr)
}

// startTCP 启动 TCP 模式
func (s *Server) startTCP() error {
	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.ln = ln

	log.Printf("[Server] 🚀 TCP 模式启动成功，监听地址: %s", s.config.ListenAddr)
	log.Printf("[Server] 🎯 目标地址: %s", s.config.TargetAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Printf("[Server] ⚠️ Accept 错误: %v", err)
			continue
		}

		go s.handleTCPConnection(conn)
	}
}

// Stop 停止服务
func (s *Server) Stop() error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

// handleTCPConnection 处理 TCP 客户端连接
func (s *Server) handleTCPConnection(clientConn net.Conn) {
	defer clientConn.Close()
	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("[Server] 📥 新 TCP 连接来自: %s", clientAddr)

	// 创建加密连接包装器
	cryptoConn := crypto.NewCryptoConn(clientConn, s.cipher)

	// 读取目标地址 (由 Client 发送)
	targetData, err := cryptoConn.ReadEncrypted()
	if err != nil {
		log.Printf("[Server] ❌ 读取目标地址失败: %v", err)
		return
	}

	targetAddr := string(targetData)
	// 如果 Client 发送的是特殊标记，使用配置的目标地址
	if targetAddr == "USE_DEFAULT" {
		targetAddr = s.config.TargetAddr
	}

	log.Printf("[Server] 🔗 连接目标: %s", targetAddr)

	// 连接目标服务器 (Owner Server / CobaltStrike TeamServer)
	targetConn, err := net.DialTimeout("tcp", targetAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Server] ❌ 连接目标失败: %v", err)
		// 发送错误响应给 Client
		cryptoConn.WriteEncrypted([]byte("ERROR:" + err.Error()))
		return
	}
	defer targetConn.Close()

	// 发送成功响应
	if err := cryptoConn.WriteEncrypted([]byte("OK")); err != nil {
		log.Printf("[Server] ❌ 发送响应失败: %v", err)
		return
	}

	log.Printf("[Server] ✅ TCP 隧道建立成功: %s <-> %s", clientAddr, targetAddr)

	// 双向数据转发
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Target (解密后转发)
	go func() {
		defer wg.Done()
		s.forwardFromClient(cryptoConn, targetConn)
	}()

	// Target -> Client (加密后转发)
	go func() {
		defer wg.Done()
		s.forwardToClient(targetConn, cryptoConn)
	}()

	wg.Wait()
	log.Printf("[Server] 🔌 TCP 连接关闭: %s", clientAddr)
}

// forwardFromClient 从 Client 读取加密数据，解密后发送到目标
func (s *Server) forwardFromClient(src *crypto.CryptoConn, dst net.Conn) {
	for {
		data, err := src.ReadEncrypted()
		if err != nil {
			if err != io.EOF {
				log.Printf("[Server] 读取客户端数据错误: %v", err)
			}
			return
		}

		if _, err := dst.Write(data); err != nil {
			log.Printf("[Server] 写入目标数据错误: %v", err)
			return
		}
	}
}

// forwardToClient 从目标读取数据，加密后发送到 Client
func (s *Server) forwardToClient(src net.Conn, dst *crypto.CryptoConn) {
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Server] 读取目标数据错误: %v", err)
			}
			return
		}

		if err := dst.WriteEncrypted(buf[:n]); err != nil {
			log.Printf("[Server] 写入客户端数据错误: %v", err)
			return
		}
	}
}
