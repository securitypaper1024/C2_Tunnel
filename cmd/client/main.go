package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tunnel/pkg/client"
	"tunnel/pkg/config"
	"tunnel/pkg/daemon"
	"tunnel/pkg/logger"
	"tunnel/pkg/transport"
)

func main() {
	listen := flag.String("listen", "", "监听地址")
	target := flag.String("target", "", "目标地址")
	serverAddr := flag.String("server", "", "Server 端地址")
	password := flag.String("password", "SecureTunnel@2024", "加密密码")
	https := flag.Bool("https", false, "启用 HTTPS CONNECT 代理模式")

	enableWS := flag.Bool("ws", false, "启用 WebSocket 传输模式")
	wsPath := flag.String("ws-path", "/ws", "WebSocket 路径")
	wsTLS := flag.Bool("ws-tls", false, "启用 WebSocket TLS")
	wsSkipVerify := flag.Bool("ws-skip-verify", false, "跳过 TLS 证书验证")

	configFile := flag.String("config", "", "配置文件路径")
	deleteConfig := flag.Bool("delete-config", false, "启动后删除配置文件")
	secureDelete := flag.Bool("secure-delete", false, "安全删除配置文件")
	genConfig := flag.String("gen-config", "", "生成示例配置文件")

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
		cfg := config.GenerateClientExampleConfig()
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
	wsConfig.SkipVerify = *wsSkipVerify

	runClient(*listen, *serverAddr, *target, *password, *https, *enableWS, wsConfig)
}

func runFromConfig(configPath string, deleteConf, secureDelete bool) {
	logger.Printf("[Config] 加载配置文件: %s", configPath)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("加载配置文件失败: %v", err)
	}

	if cfg.Mode != "" && cfg.Mode != "client" {
		logger.Fatalf("配置文件中的 mode 不是 client")
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

	if cfg.Client.LogPath != "" || cfg.Client.Quiet {
		if err := logger.InitLogger(cfg.Client.LogPath, cfg.Client.Quiet); err != nil {
			logger.Fatalf("初始化日志失败: %v", err)
		}
	}

	if cfg.Client.Daemon {
		if err := daemon.Daemonize(); err != nil {
			logger.Fatalf("后台运行失败: %v", err)
		}
		os.Exit(0)
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
		logger.Fatal("请指定监听地址 (-listen)")
	}
	if serverAddr == "" {
		logger.Fatal("请指定 Server 地址 (-server)")
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
		logger.Fatalf("创建 Client 失败: %v", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Println("正在关闭 Client...")
		cli.Stop()
		os.Exit(0)
	}()

	if err := cli.Start(); err != nil {
		logger.Fatalf("Client 启动失败: %v", err)
	}
}

