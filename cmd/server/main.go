package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tunnel/pkg/acl"
	"tunnel/pkg/config"
	"tunnel/pkg/daemon"
	"tunnel/pkg/logger"
	"tunnel/pkg/server"
	"tunnel/pkg/transport"
)

func main() {
	listen := flag.String("listen", "", "监听地址")
	target := flag.String("target", "", "目标地址")
	password := flag.String("password", "SecureTunnel@2024", "加密密码")

	enableWS := flag.Bool("ws", false, "启用 WebSocket 传输模式")
	wsPath := flag.String("ws-path", "/ws", "WebSocket 路径")
	wsTLS := flag.Bool("ws-tls", false, "启用 WebSocket TLS")
	wsCert := flag.String("ws-cert", "", "TLS 证书文件路径")
	wsKey := flag.String("ws-key", "", "TLS 密钥文件路径")

	configFile := flag.String("config", "", "配置文件路径")
	deleteConfig := flag.Bool("delete-config", false, "启动后删除配置文件")
	secureDelete := flag.Bool("secure-delete", false, "安全删除配置文件")
	genConfig := flag.String("gen-config", "", "生成示例配置文件")

	aclEnable := flag.Bool("acl", false, "启用访问控制")
	aclMode := flag.String("acl-mode", "whitelist", "ACL 模式: whitelist 或 blacklist")
	aclWhitelist := flag.String("acl-whitelist", "", "白名单")
	aclBlacklist := flag.String("acl-blacklist", "", "黑名单")

	logPath := flag.String("log", "", "日志文件路径")
	daemonMode := flag.Bool("daemon", false, "后台运行模式")
	quiet := flag.Bool("quiet", false, "静默模式，不输出到终端")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *genConfig != "" {
		cfg := config.GenerateServerExampleConfig()
		if err := config.SaveConfig(cfg, *genConfig); err != nil {
			fmt.Fprintf(os.Stderr, "生成配置文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("示例配置文件已生成: %s\n", *genConfig)
		return
	}

	if *daemonMode {
		if err := daemon.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "后台运行失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := logger.InitLogger(*logPath, *quiet); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	if *configFile != "" {
		runFromConfig(*configFile, *deleteConfig, *secureDelete)
		return
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = *wsPath
	wsConfig.EnableTLS = *wsTLS
	wsConfig.TLSCert = *wsCert
	wsConfig.TLSKey = *wsKey

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

func runFromConfig(configPath string, deleteConf, secureDelete bool) {
	logger.Printf("[Config] 加载配置文件: %s", configPath)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("加载配置文件失败: %v", err)
	}

	if cfg.Mode != "" && cfg.Mode != "server" {
		logger.Fatalf("配置文件中的 mode 不是 server")
	}

	if deleteConf || secureDelete {
		if secureDelete {
			logger.Printf("[Config] 安全删除配置文件...")
			if err := config.SecureDeleteConfigFile(configPath); err != nil {
				logger.Printf("[Config] 安全删除失败: %v", err)
			} else {
				logger.Printf("[Config] 配置文件已安全删除")
			}
		} else {
			logger.Printf("[Config] 删除配置文件...")
			if err := config.DeleteConfigFile(configPath); err != nil {
				logger.Printf("[Config] 删除失败: %v", err)
			} else {
				logger.Printf("[Config] 配置文件已删除")
			}
		}
	}

	if cfg.Server.LogPath != "" || cfg.Server.Quiet {
		if err := logger.InitLogger(cfg.Server.LogPath, cfg.Server.Quiet); err != nil {
			logger.Fatalf("初始化日志失败: %v", err)
		}
	}

	if cfg.Server.Daemon {
		if err := daemon.Daemonize(); err != nil {
			logger.Fatalf("后台运行失败: %v", err)
		}
		os.Exit(0)
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
		logger.Fatal("请指定监听地址 (-listen)")
	}
	if target == "" {
		logger.Fatal("请指定目标地址 (-target)")
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
		logger.Fatalf("创建 Server 失败: %v", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Println("正在关闭 Server...")
		srv.Stop()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		logger.Fatalf("Server 启动失败: %v", err)
	}
}

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

