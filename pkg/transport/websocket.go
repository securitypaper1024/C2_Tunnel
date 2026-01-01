package transport

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"tunnel/pkg/crypto"
)

type WSConfig struct {
	Path            string
	Origin          string
	EnableTLS       bool
	TLSCert         string
	TLSKey          string
	SkipVerify      bool
	PingInterval    time.Duration
	ReadBufferSize  int
	WriteBufferSize int
}

func DefaultWSConfig() WSConfig {
	return WSConfig{
		Path:            "/ws",
		PingInterval:    30 * time.Second,
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
	}
}

type WSConn struct {
	conn   *websocket.Conn
	cipher *crypto.AESCipher
	mu     sync.Mutex
}

func NewWSConn(conn *websocket.Conn, cipher *crypto.AESCipher) *WSConn {
	return &WSConn{
		conn:   conn,
		cipher: cipher,
	}
}

func (w *WSConn) ReadEncrypted() ([]byte, error) {
	_, message, err := w.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	encrypted, err := base64.StdEncoding.DecodeString(string(message))
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	return w.cipher.Decrypt(encrypted)
}

func (w *WSConn) WriteEncrypted(data []byte) error {
	encrypted, err := w.cipher.Encrypt(data)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(encrypted)

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.conn.WriteMessage(websocket.TextMessage, []byte(encoded))
}

func (w *WSConn) Close() error {
	return w.conn.Close()
}

func (w *WSConn) RemoteAddr() net.Addr {
	return w.conn.RemoteAddr()
}

func (w *WSConn) StartPing(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			w.mu.Lock()
			err := w.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			w.mu.Unlock()

			if err != nil {
				return
			}
		}
	}()
}

type WSServer struct {
	config   WSConfig
	cipher   *crypto.AESCipher
	upgrader websocket.Upgrader
	handler  func(*WSConn)
}

func NewWSServer(config WSConfig, cipher *crypto.AESCipher, handler func(*WSConn)) *WSServer {
	return &WSServer{
		config: config,
		cipher: cipher,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		handler: handler,
	}
}

func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.config.Path {
		s.serveFakePage(w, r)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS-Server] ⚠️ 升级 WebSocket 失败: %v", err)
		return
	}

	wsConn := NewWSConn(conn, s.cipher)
	wsConn.StartPing(s.config.PingInterval)

	log.Printf("[WS-Server] 📥 新 WebSocket 连接: %s", conn.RemoteAddr())

	s.handler(wsConn)
}

func (s *WSServer) serveFakePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Welcome</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        h1 { color: #333; }
    </style>
</head>
<body>
    <h1>Welcome to our website</h1>
    <p>This server is running normally.</p>
</body>
</html>`
	w.Write([]byte(html))
}

func (s *WSServer) Start(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: s,
	}

	if s.config.EnableTLS {
		log.Printf("[WS-Server] 🔒 启用 TLS，监听地址: %s%s", addr, s.config.Path)
		return server.ListenAndServeTLS(s.config.TLSCert, s.config.TLSKey)
	}

	log.Printf("[WS-Server] 🚀 启动成功，监听地址: ws://%s%s", addr, s.config.Path)
	return server.ListenAndServe()
}

type WSClient struct {
	config WSConfig
	cipher *crypto.AESCipher
}

func NewWSClient(config WSConfig, cipher *crypto.AESCipher) *WSClient {
	return &WSClient{
		config: config,
		cipher: cipher,
	}
}

func (c *WSClient) Connect(serverAddr string) (*WSConn, error) {
	var scheme string
	if c.config.EnableTLS {
		scheme = "wss"
	} else {
		scheme = "ws"
	}

	url := fmt.Sprintf("%s://%s%s", scheme, serverAddr, c.config.Path)

	dialer := websocket.Dialer{
		ReadBufferSize:   c.config.ReadBufferSize,
		WriteBufferSize:  c.config.WriteBufferSize,
		HandshakeTimeout: 10 * time.Second,
	}

	if c.config.EnableTLS && c.config.SkipVerify {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	headers := http.Header{}
	if c.config.Origin != "" {
		headers.Set("Origin", c.config.Origin)
	}

	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	wsConn := NewWSConn(conn, c.cipher)
	wsConn.StartPing(c.config.PingInterval)

	log.Printf("[WS-Client] ✅ 连接成功: %s", url)

	return wsConn, nil
}

func BridgeWSToTCP(ws *WSConn, tcp net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			data, err := ws.ReadEncrypted()
			if err != nil {
				if err != io.EOF {
					log.Printf("[Bridge] WS->TCP 读取错误: %v", err)
				}
				return
			}
			if _, err := tcp.Write(data); err != nil {
				log.Printf("[Bridge] WS->TCP 写入错误: %v", err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcp.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Bridge] TCP->WS 读取错误: %v", err)
				}
				return
			}
			if err := ws.WriteEncrypted(buf[:n]); err != nil {
				log.Printf("[Bridge] TCP->WS 写入错误: %v", err)
				return
			}
		}
	}()

	wg.Wait()
}
