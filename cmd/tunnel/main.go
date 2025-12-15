package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tunnel/pkg/client"
	"tunnel/pkg/server"
	"tunnel/pkg/transport"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║   ____                            _____                  _    ║
║  / ___|  ___  ___ _   _ _ __ ___|_   _|   _ _ __  _ __ | |   ║
║  \___ \ / _ \/ __| | | | '__/ _ \ | || | | | '_ \| '_ \| |   ║
║   ___) |  __/ (__| |_| | | |  __/ | || |_| | | | | | | | |   ║
║  |____/ \___|\___|\__,_|_|  \___| |_| \__,_|_| |_|_| |_|_|   ║
║                                                               ║
║       AES-256-CFB Encrypted Tunnel for CobaltStrike           ║
║                        v1.1.0                                 ║
║                  + WebSocket Support                          ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	// 命令行参数
	mode := flag.String("mode", "", "运行模式: server 或 client")
	listen := flag.String("listen", "", "监听地址 (例: 0.0.0.0:8888)")
	target := flag.String("target", "", "目标地址 (例: 192.168.1.100:443)")
	serverAddr := flag.String("server", "", "[Client] Server 端地址 (例: vps.example.com:8888)")
	password := flag.String("password", "SecureTunnel@2024", "加密密码")
	https := flag.Bool("https", false, "[Client] 启用 HTTPS CONNECT 代理模式")

	// WebSocket 参数
	enableWS := flag.Bool("ws", false, "启用 WebSocket 传输模式")
	wsPath := flag.String("ws-path", "/ws", "WebSocket 路径")
	wsTLS := flag.Bool("ws-tls", false, "启用 WebSocket TLS (wss://)")
	wsCert := flag.String("ws-cert", "", "TLS 证书文件路径")
	wsKey := flag.String("ws-key", "", "TLS 密钥文件路径")
	wsSkipVerify := flag.Bool("ws-skip-verify", false, "[Client] 跳过 TLS 证书验证")

	flag.Usage = func() {
		fmt.Println(banner)
		fmt.Println("使用方法:")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  TCP 模式 (传统加密隧道)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  Server 模式 (部署在 VPS 上):")
		fmt.Println("    tunnel -mode server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass")
		fmt.Println()
		fmt.Println("  Client 模式 (部署在本地或受控机器上):")
		fmt.Println("    tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:8888 -password mypass")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  WebSocket 模式 (流量伪装，更隐蔽)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  Server WebSocket 模式:")
		fmt.Println("    tunnel -mode server -listen 0.0.0.0:80 -target 127.0.0.1:50050 -password mypass -ws -ws-path /chat")
		fmt.Println()
		fmt.Println("  Server WebSocket + TLS 模式:")
		fmt.Println("    tunnel -mode server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password mypass -ws -ws-tls -ws-cert cert.pem -ws-key key.pem")
		fmt.Println()
		fmt.Println("  Client WebSocket 模式:")
		fmt.Println("    tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:80 -password mypass -ws -ws-path /chat")
		fmt.Println()
		fmt.Println("  Client WebSocket + TLS 模式:")
		fmt.Println("    tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:443 -password mypass -ws -ws-tls -ws-skip-verify")
		fmt.Println()
		fmt.Println("参数说明:")
		flag.PrintDefaults()
	}

	flag.Parse()

	fmt.Println(banner)

	if *mode == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 构建 WebSocket 配置
	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = *wsPath
	wsConfig.EnableTLS = *wsTLS
	wsConfig.TLSCert = *wsCert
	wsConfig.TLSKey = *wsKey
	wsConfig.SkipVerify = *wsSkipVerify

	switch *mode {
	case "server":
		runServer(*listen, *target, *password, *enableWS, wsConfig)
	case "client":
		runClient(*listen, *serverAddr, *target, *password, *https, *enableWS, wsConfig)
	default:
		log.Fatalf("❌ 未知模式: %s，请使用 server 或 client", *mode)
	}
}

func runServer(listen, target, password string, enableWS bool, wsConfig transport.WSConfig) {
	if listen == "" {
		log.Fatal("❌ 请指定监听地址 (-listen)")
	}
	if target == "" {
		log.Fatal("❌ 请指定目标地址 (-target)，例如 CobaltStrike TeamServer 地址")
	}

	cfg := server.Config{
		ListenAddr:   listen,
		TargetAddr:   target,
		Password:     password,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableWS:     enableWS,
		WSConfig:     wsConfig,
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("❌ 创建 Server 失败: %v", err)
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("\n⏹️ 正在关闭 Server...")
		srv.Stop()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("❌ Server 启动失败: %v", err)
	}
}

func runClient(listen, serverAddr, target, password string, https, enableWS bool, wsConfig transport.WSConfig) {
	if listen == "" {
		log.Fatal("❌ 请指定监听地址 (-listen)")
	}
	if serverAddr == "" {
		log.Fatal("❌ 请指定 Server 地址 (-server)")
	}

	cfg := client.Config{
		ListenAddr:   listen,
		ServerAddr:   serverAddr,
		TargetAddr:   target,
		Password:     password,
		EnableHTTPS:  https,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableWS:     enableWS,
		WSConfig:     wsConfig,
	}

	cli, err := client.New(cfg)
	if err != nil {
		log.Fatalf("❌ 创建 Client 失败: %v", err)
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("\n⏹️ 正在关闭 Client...")
		cli.Stop()
		os.Exit(0)
	}()

	if err := cli.Start(); err != nil {
		log.Fatalf("❌ Client 启动失败: %v", err)
	}
}
