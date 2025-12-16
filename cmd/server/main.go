package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tunnel/pkg/acl"
	"tunnel/pkg/config"
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
║                      Server v1.2.0                            ║
║          + WebSocket + Config File + ACL Support              ║
╚═══════════════════════════════════════════════════════════════╝
`

func main() {
	// 命令行参数
	listen := flag.String("listen", "", "监听地址 (例: 0.0.0.0:8888)")
	target := flag.String("target", "", "目标地址 (例: 127.0.0.1:50050)")
	password := flag.String("password", "SecureTunnel@2024", "加密密码")

	// WebSocket 参数
	enableWS := flag.Bool("ws", false, "启用 WebSocket 传输模式")
	wsPath := flag.String("ws-path", "/ws", "WebSocket 路径")
	wsTLS := flag.Bool("ws-tls", false, "启用 WebSocket TLS (wss://)")
	wsCert := flag.String("ws-cert", "", "TLS 证书文件路径")
	wsKey := flag.String("ws-key", "", "TLS 密钥文件路径")

	// 配置文件参数
	configFile := flag.String("config", "", "配置文件路径 (JSON/YAML)")
	deleteConfig := flag.Bool("delete-config", false, "启动后删除配置文件")
	secureDelete := flag.Bool("secure-delete", false, "安全删除配置文件 (覆写后删除)")
	genConfig := flag.String("gen-config", "", "生成示例配置文件")

	// ACL 参数
	aclEnable := flag.Bool("acl", false, "启用访问控制")
	aclMode := flag.String("acl-mode", "whitelist", "ACL 模式: whitelist 或 blacklist")
	aclWhitelist := flag.String("acl-whitelist", "", "白名单 (逗号分隔，支持 CIDR)")
	aclBlacklist := flag.String("acl-blacklist", "", "黑名单 (逗号分隔，支持 CIDR)")

	flag.Usage = func() {
		fmt.Println(banner)
		fmt.Println("使用方法:")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  配置文件模式")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  生成示例配置文件:")
		fmt.Println("    tunnel-server -gen-config server.yaml")
		fmt.Println()
		fmt.Println("  使用配置文件启动:")
		fmt.Println("    tunnel-server -config server.yaml")
		fmt.Println()
		fmt.Println("  启动后删除配置文件:")
		fmt.Println("    tunnel-server -config server.yaml -delete-config")
		fmt.Println()
		fmt.Println("  安全删除配置文件 (覆写后删除):")
		fmt.Println("    tunnel-server -config server.yaml -secure-delete")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  TCP 模式 (传统加密隧道)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  基本模式:")
		fmt.Println("    tunnel-server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass")
		fmt.Println()
		fmt.Println("  ACL 白名单:")
		fmt.Println("    tunnel-server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass -acl -acl-mode whitelist -acl-whitelist \"192.168.1.0/24,10.0.0.1\"")
		fmt.Println()
		fmt.Println("  ACL 黑名单:")
		fmt.Println("    tunnel-server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass -acl -acl-mode blacklist -acl-blacklist \"192.168.1.100,10.0.0.0/8\"")
		fmt.Println()
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  WebSocket 模式 (流量伪装，更隐蔽)")
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Println("  WebSocket 模式:")
		fmt.Println("    tunnel-server -listen 0.0.0.0:80 -target 127.0.0.1:50050 -password mypass -ws -ws-path /chat")
		fmt.Println()
		fmt.Println("  WebSocket TLS 模式:")
		fmt.Println("    tunnel-server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password mypass -ws -ws-path /chat -ws-tls -ws-cert cert.pem -ws-key key.pem")
		fmt.Println()
		fmt.Println("参数说明:")
		flag.PrintDefaults()
	}

	flag.Parse()

	fmt.Println(banner)

	// 生成示例配置文件
	if *genConfig != "" {
		generateServerExampleConfig(*genConfig)
		return
	}

	// 从配置文件加载
	if *configFile != "" {
		runFromConfig(*configFile, *deleteConfig, *secureDelete)
		return
	}

	// 构建 WebSocket 配置
	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = *wsPath
	wsConfig.EnableTLS = *wsTLS
	wsConfig.TLSCert = *wsCert
	wsConfig.TLSKey = *wsKey

	// 构建 ACL 配置
	aclConfig := acl.Config{
		Enable: *aclEnable,
		Mode:   *aclMode,
	}
	if *aclWhitelist != "" {
		aclConfig.Whitelist = splitAndTrim(*aclWhitelist)
	}
	if *aclBlacklist != "" {
		aclConfig.Blacklist = splitAndTrim(*aclBlacklist)
	}

	runServer(*listen, *target, *password, *enableWS, wsConfig, aclConfig)
}

// generateServerExampleConfig 生成 Server 示例配置文件
func generateServerExampleConfig(path string) {
	cfg := config.GenerateServerExampleConfig()
	if err := config.SaveConfig(cfg, path); err != nil {
		log.Fatalf("❌ 生成配置文件失败: %v", err)
	}
	log.Printf("✅ 示例配置文件已生成: %s", path)
}

// runFromConfig 从配置文件启动
func runFromConfig(configPath string, deleteConf, secureDelete bool) {
	log.Printf("[Config] 📄 加载配置文件: %s", configPath)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置文件失败: %v", err)
	}

	// 检查模式
	if cfg.Mode != "" && cfg.Mode != "server" {
		log.Fatalf("❌ 配置文件中的 mode 不是 server，请使用 tunnel-client")
	}

	// 删除配置文件
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
	wsConfig.Path = cfg.Server.WSPath
	wsConfig.EnableTLS = cfg.Server.WSTLS
	wsConfig.TLSCert = cfg.Server.WSCert
	wsConfig.TLSKey = cfg.Server.WSKey

	aclConfig := acl.Config{
		Enable:    cfg.Server.ACL.Enable,
		Mode:      cfg.Server.ACL.Mode,
		Whitelist: cfg.Server.ACL.Whitelist,
		Blacklist: cfg.Server.ACL.Blacklist,
	}

	runServer(cfg.Server.Listen, cfg.Server.Target, cfg.Server.Password,
		cfg.Server.EnableWS, wsConfig, aclConfig)
}

func runServer(listen, target, password string, enableWS bool, wsConfig transport.WSConfig, aclConfig acl.Config) {
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
		ACLConfig:    aclConfig,
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

// splitAndTrim 分割并去除空格
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, part := range splitString(s, ",") {
		part = trimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

