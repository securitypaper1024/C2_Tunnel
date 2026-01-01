package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"tunnel/pkg/crypto"
	"tunnel/pkg/transport"
)

type Config struct {
	ListenAddr   string
	ServerAddr   string
	TargetAddr   string
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
	ln, err := net.Listen("tcp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	c.ln = ln

	if c.config.EnableWS {
		log.Printf("[Client] 🌐 WebSocket 模式启动成功，监听地址: %s", c.config.ListenAddr)
	} else {
		log.Printf("[Client] 🚀 TCP 模式启动成功，监听地址: %s", c.config.ListenAddr)
	}
	log.Printf("[Client] 🔗 Server 地址: %s", c.config.ServerAddr)
	if c.config.TargetAddr != "" {
		log.Printf("[Client] 🎯 默认目标: %s", c.config.TargetAddr)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Printf("[Client] ⚠️ Accept 错误: %v", err)
			continue
		}

		go c.handleConnection(conn)
	}
}

func (c *Client) Stop() error {
	if c.ln != nil {
		return c.ln.Close()
	}
	return nil
}

func (c *Client) handleConnection(ownerConn net.Conn) {
	defer ownerConn.Close()
	ownerAddr := ownerConn.RemoteAddr().String()
	log.Printf("[Client] 📥 新连接来自: %s", ownerAddr)

	var targetAddr string
	var initialData []byte

	if c.config.EnableHTTPS {
		target, data, err := c.handleHTTPSConnect(ownerConn)
		if err != nil {
			log.Printf("[Client] ❌ HTTPS CONNECT 处理失败: %v", err)
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
		c.handleWSConnection(ownerConn, ownerAddr, targetAddr, initialData)
	} else {
		c.handleTCPConnection(ownerConn, ownerAddr, targetAddr, initialData)
	}
}

func (c *Client) handleWSConnection(ownerConn net.Conn, ownerAddr, targetAddr string, initialData []byte) {
	wsConn, err := c.wsClient.Connect(c.config.ServerAddr)
	if err != nil {
		log.Printf("[Client] ❌ 连接 WebSocket Server 失败: %v", err)
		return
	}
	defer wsConn.Close()

	if err := wsConn.WriteEncrypted([]byte(targetAddr)); err != nil {
		log.Printf("[Client] ❌ 发送目标地址失败: %v", err)
		return
	}

	response, err := wsConn.ReadEncrypted()
	if err != nil {
		log.Printf("[Client] ❌ 读取 Server 响应失败: %v", err)
		return
	}

	if !strings.HasPrefix(string(response), "OK") {
		log.Printf("[Client] ❌ Server 返回错误: %s", string(response))
		return
	}

	log.Printf("[Client] ✅ WebSocket 隧道建立成功: %s -> %s", ownerAddr, targetAddr)

	if len(initialData) > 0 {
		if err := wsConn.WriteEncrypted(initialData); err != nil {
			log.Printf("[Client] ❌ 发送初始数据失败: %v", err)
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := ownerConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Client] 读取 Owner 数据错误: %v", err)
				}
				return
			}
			if err := wsConn.WriteEncrypted(buf[:n]); err != nil {
				log.Printf("[Client] 写入 WebSocket 数据错误: %v", err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			data, err := wsConn.ReadEncrypted()
			if err != nil {
				if err != io.EOF {
					log.Printf("[Client] 读取 WebSocket 数据错误: %v", err)
				}
				return
			}
			if _, err := ownerConn.Write(data); err != nil {
				log.Printf("[Client] 写入 Owner 数据错误: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	log.Printf("[Client] 🔌 WebSocket 连接关闭: %s", ownerAddr)
}

func (c *Client) handleTCPConnection(ownerConn net.Conn, ownerAddr, targetAddr string, initialData []byte) {
	serverConn, err := net.DialTimeout("tcp", c.config.ServerAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Client] ❌ 连接 Server 失败: %v", err)
		return
	}
	defer serverConn.Close()

	cryptoConn := crypto.NewCryptoConn(serverConn, c.cipher)

	if err := cryptoConn.WriteEncrypted([]byte(targetAddr)); err != nil {
		log.Printf("[Client] ❌ 发送目标地址失败: %v", err)
		return
	}

	response, err := cryptoConn.ReadEncrypted()
	if err != nil {
		log.Printf("[Client] ❌ 读取 Server 响应失败: %v", err)
		return
	}

	if !strings.HasPrefix(string(response), "OK") {
		log.Printf("[Client] ❌ Server 返回错误: %s", string(response))
		return
	}

	log.Printf("[Client] ✅ TCP 隧道建立成功: %s -> %s", ownerAddr, targetAddr)

	if len(initialData) > 0 {
		if err := cryptoConn.WriteEncrypted(initialData); err != nil {
			log.Printf("[Client] ❌ 发送初始数据失败: %v", err)
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.forwardToServer(ownerConn, cryptoConn)
	}()

	go func() {
		defer wg.Done()
		c.forwardFromServer(cryptoConn, ownerConn)
	}()

	wg.Wait()
	log.Printf("[Client] 🔌 TCP 连接关闭: %s", ownerAddr)
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

		log.Printf("[Client] 🔒 HTTPS CONNECT: %s", targetAddr)
	} else {
		targetAddr = req.Host
		if !strings.Contains(targetAddr, ":") {
			targetAddr += ":80"
		}

		var buf bytes.Buffer
		req.Write(&buf)
		initialData = buf.Bytes()

		log.Printf("[Client] 🌐 HTTP Request: %s %s", req.Method, targetAddr)
	}

	return targetAddr, initialData, nil
}

func (c *Client) forwardToServer(src net.Conn, dst *crypto.CryptoConn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Client] 读取 Owner 数据错误: %v", err)
			}
			return
		}

		if err := dst.WriteEncrypted(buf[:n]); err != nil {
			log.Printf("[Client] 写入 Server 数据错误: %v", err)
			return
		}
	}
}

func (c *Client) forwardFromServer(src *crypto.CryptoConn, dst net.Conn) {
	for {
		data, err := src.ReadEncrypted()
		if err != nil {
			if err != io.EOF {
				log.Printf("[Client] 读取 Server 数据错误: %v", err)
			}
			return
		}

		if _, err := dst.Write(data); err != nil {
			log.Printf("[Client] 写入 Owner 数据错误: %v", err)
			return
		}
	}
}
