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
	"tunnel/pkg/config"
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
║                      Client v1.2.0                            ║
║          + WebSocket + Config File + ACL Support              ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	listen := flag.String("listen", "", "监听地址 (例: 127.0.0.1:443)")
	target := flag.String("target", "", "目标地址 (用于 HTTPS CONNECT 模式)")
	serverAddr := flag.String("server", "", "Server 端地址 (例: vps.example.com:8888)")
	password := flag.String("password", "SecureTunnel@2024", "加密密码")
	https := flag.Bool("https", false, "启用 HTTPS CONNECT 代理模式")

	enableWS := flag.Bool("ws", false, "启用 WebSocket 传输模式")
	wsPath := flag.String("ws-path", "/ws", "WebSocket 路径")
	wsTLS := flag.Bool("ws-tls", false, "启用 WebSocket TLS (wss://)")
	wsSkipVerify := flag.Bool("ws-skip-verify", false, "跳过 TLS 证书验证")

	configFile := flag.String("config", "", "配置文件路径 (JSON/YAML)")
	deleteConfig := flag.Bool("delete-config", false, "启动后删除配置文件")
	secureDelete := flag.Bool("secure-delete", false, "安全删除配置文件 (覆写后删除)")
	genConfig := flag.String("gen-config", "", "生成示例配置文件")

	flag.Usage = func() {
		fmt.Print(banner)
		fmt.Println("使用方法:")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  配置文件模式")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  生成示例配置文件:")
		fmt.Println("    tunnel-client -gen-config client.yaml")
		fmt.Println()
		fmt.Println("  使用配置文件启动:")
		fmt.Println("    tunnel-client -config client.yaml")
		fmt.Println()
		fmt.Println("  启动后删除配置文件:")
		fmt.Println("    tunnel-client -config client.yaml -delete-config")
		fmt.Println()
		fmt.Println("  安全删除配置文件 (覆写后删除):")
		fmt.Println("    tunnel-client -config client.yaml -secure-delete")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  TCP 模式 (传统加密隧道)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  基本模式:")
		fmt.Println("    tunnel-client -listen 127.0.0.1:443 -server vps.example.com:8888 -password mypass")
		fmt.Println()
		fmt.Println("  HTTPS CONNECT 代理模式:")
		fmt.Println("    tunnel-client -listen 127.0.0.1:443 -server vps.example.com:8888 -password mypass -https")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  WebSocket 模式 (流量伪装，更隐蔽)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  WebSocket 模式:")
		fmt.Println("    tunnel-client -listen 127.0.0.1:443 -server vps.example.com:80 -password mypass -ws -ws-path /chat")
		fmt.Println()
		fmt.Println("  WebSocket TLS 模式:")
		fmt.Println("    tunnel-client -listen 127.0.0.1:443 -server vps.example.com:443 -password mypass -ws -ws-path /chat -ws-tls")
		fmt.Println()
		fmt.Println("  WebSocket TLS 跳过证书验证:")
		fmt.Println("    tunnel-client -listen 127.0.0.1:443 -server vps.example.com:443 -password mypass -ws -ws-path /chat -ws-tls -ws-skip-verify")
		fmt.Println()
		fmt.Print("参数说明:")
		flag.PrintDefaults()
	}

	flag.Parse()

	fmt.Print(banner)

	if *genConfig != "" {
		generateClientExampleConfig(*genConfig)
		return
	}

	if *configFile != "" {
		runFromConfig(*configFile, *deleteConfig, *secureDelete)
		return
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = *wsPath
	wsConfig.EnableTLS = *wsTLS
	wsConfig.SkipVerify = *wsSkipVerify

	runClient(*listen, *serverAddr, *target, *password, *https, *enableWS, wsConfig)
}

func generateClientExampleConfig(path string) {
	cfg := config.GenerateClientExampleConfig()
	if err := config.SaveConfig(cfg, path); err != nil {
		log.Fatalf("❌ 生成配置文件失败: %v", err)
	}
	log.Printf("✅ 示例配置文件已生成: %s", path)
}

func runFromConfig(configPath string, deleteConf, secureDelete bool) {
	log.Printf("[Config] 📄 加载配置文件: %s", configPath)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置文件失败: %v", err)
	}

	if cfg.Mode != "" && cfg.Mode != "client" {
		log.Fatalf("❌ 配置文件中的 mode 不是 client，请使用 tunnel-server")
	}

	if deleteConf || secureDelete {
		if secureDelete {
			log.Printf("[Config] 🔒 安全删除配置文件...")
			if err := config.SecureDeleteConfigFile(configPath); err != nil {
				log.Printf("[Config] ⚠️ 安全删除失败: %v", err)
			} else {
				log.Printf("[Config] ✅ 配置文件已安全删除")
			}
		} else {
			log.Printf("[Config] 🗑️ 删除配置文件...")
			if err := config.DeleteConfigFile(configPath); err != nil {
				log.Printf("[Config] ⚠️ 删除失败: %v", err)
			} else {
				log.Printf("[Config] ✅ 配置文件已删除")
			}
		}
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = cfg.Client.WSPath
	wsConfig.EnableTLS = cfg.Client.WSTLS
	wsConfig.SkipVerify = cfg.Client.WSSkipVerify

	runClient(cfg.Client.Listen, cfg.Client.Server, cfg.Client.Target,
		cfg.Client.Password, cfg.Client.EnableHTTPS, cfg.Client.EnableWS, wsConfig)
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
