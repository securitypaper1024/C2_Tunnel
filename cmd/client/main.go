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

const Version = "1.4.0"

func main() {
	var listen, target, serverAddr, password string
	var https, enableWS, wsTLS, wsSkipVerify bool
	var wsPath string
	var configFile, logPath string
	var deleteConfig, secureDelete, daemonMode, quiet, showVersion, showHelp bool
	var genConfig string

	flag.StringVar(&listen, "l", "", "监听地址 (简写)")
	flag.StringVar(&listen, "listen", "", "监听地址")
	flag.StringVar(&target, "t", "", "目标地址 (简写)")
	flag.StringVar(&target, "target", "", "目标地址")
	flag.StringVar(&serverAddr, "s", "", "Server 端地址 (简写)")
	flag.StringVar(&serverAddr, "server", "", "Server 端地址")
	flag.StringVar(&password, "p", "SecureTunnel@2024", "加密密码 (简写)")
	flag.StringVar(&password, "password", "SecureTunnel@2024", "加密密码")
	flag.BoolVar(&https, "https", false, "启用 HTTPS CONNECT 代理模式")

	flag.BoolVar(&enableWS, "ws", false, "启用 WebSocket 传输模式")
	flag.StringVar(&wsPath, "ws-path", "/ws", "WebSocket 路径")
	flag.BoolVar(&wsTLS, "ws-tls", false, "启用 WebSocket TLS")
	flag.BoolVar(&wsSkipVerify, "ws-skip-verify", false, "跳过 TLS 证书验证")

	flag.StringVar(&configFile, "c", "", "配置文件路径 (简写)")
	flag.StringVar(&configFile, "config", "", "配置文件路径")
	flag.BoolVar(&deleteConfig, "delete-config", false, "启动后删除配置文件")
	flag.BoolVar(&secureDelete, "secure-delete", false, "安全删除配置文件")
	flag.StringVar(&genConfig, "gen-config", "", "生成示例配置文件")

	flag.StringVar(&logPath, "log", "", "日志文件路径")
	flag.BoolVar(&daemonMode, "d", false, "后台运行模式 (简写)")
	flag.BoolVar(&daemonMode, "daemon", false, "后台运行模式")
	flag.BoolVar(&quiet, "q", false, "静默模式 (简写)")
	flag.BoolVar(&quiet, "quiet", false, "静默模式，不输出到终端")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CS_Tunnel Client v%s - C2 流量加密隧道\n\n", Version)
		fmt.Fprintf(os.Stderr, "用法:\n")
		fmt.Fprintf(os.Stderr, "  %s -l <监听地址> -s <服务器地址> [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "快速示例:\n")
		fmt.Fprintf(os.Stderr, "  %s -l 127.0.0.1:443 -s vps.example.com:8888 -p mypass\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -l :443 -s vps:8888 -ws              # WebSocket模式\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -c client.yaml                       # 使用配置文件\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		return
	}

	if showVersion {
		fmt.Printf("CS_Tunnel Client v%s\n", Version)
		return
	}

	if genConfig != "" {
		cfg := config.GenerateClientExampleConfig()
		if err := config.SaveConfig(cfg, genConfig); err != nil {
			fmt.Fprintf(os.Stderr, "生成配置文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("示例配置文件已生成: %s\n", genConfig)
		return
	}

	if daemonMode {
		if err := daemon.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "后台运行失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := logger.InitLogger(logPath, quiet); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	if configFile != "" {
		runFromConfig(configFile, deleteConfig, secureDelete)
		return
	}

	if listen == "" || serverAddr == "" {
		fmt.Fprintf(os.Stderr, "错误: 必须指定监听地址(-l)和服务器地址(-s)\n\n")
		fmt.Fprintf(os.Stderr, "快速示例:\n")
		fmt.Fprintf(os.Stderr, "  %s -l 127.0.0.1:443 -s vps.example.com:8888\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "使用 -h 查看帮助\n")
		os.Exit(1)
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = wsPath
	wsConfig.EnableTLS = wsTLS
	wsConfig.SkipVerify = wsSkipVerify

	runClient(listen, serverAddr, target, password, https, enableWS, wsConfig)
}

func runFromConfig(configPath string, deleteConf, secureDelete bool) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("加载配置文件失败: %v", err)
	}

	if cfg.Mode != "" && cfg.Mode != "client" {
		logger.Fatalf("配置文件中的 mode 不是 client")
	}

	if cfg.Client.Daemon {
		if err := daemon.Daemonize(); err != nil {
			logger.Fatalf("后台运行失败: %v", err)
		}
		os.Exit(0)
	}

	if cfg.Client.LogPath != "" || cfg.Client.Quiet {
		if err := logger.InitLogger(cfg.Client.LogPath, cfg.Client.Quiet); err != nil {
			logger.Fatalf("初始化日志失败: %v", err)
		}
	}

	logger.Printf("[Config] 加载配置文件: %s", configPath)

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

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = cfg.Client.WSPath
	wsConfig.EnableTLS = cfg.Client.WSTLS
	wsConfig.SkipVerify = cfg.Client.WSSkipVerify

	runClient(cfg.Client.Listen, cfg.Client.Server, cfg.Client.Target,
		cfg.Client.Password, cfg.Client.EnableHTTPS, cfg.Client.EnableWS, wsConfig)
}

func runClient(listen, serverAddr, target, password string, https, enableWS bool, wsConfig transport.WSConfig) {
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

