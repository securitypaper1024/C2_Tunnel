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
	"tunnel/pkg/transport"
)

const Version = "1.4.0"

func main() {
	var listen, target, serverAddr, password, protocol string
	var https, enableWS, wsTLS, wsSkipVerify bool
	var wsPath string
	var configFile string
	var secureDelete, daemonMode, showVersion, showHelp bool
	var genConfig string

	flag.StringVar(&listen, "l", "", "listen address (short)")
	flag.StringVar(&listen, "listen", "", "listen address")
	flag.StringVar(&target, "t", "", "target address (short)")
	flag.StringVar(&target, "target", "", "target address")
	flag.StringVar(&serverAddr, "s", "", "server address (short)")
	flag.StringVar(&serverAddr, "server", "", "server address")
	flag.StringVar(&password, "p", "SecureTunnel@2024", "encryption password (short)")
	flag.StringVar(&password, "password", "SecureTunnel@2024", "encryption password")
	flag.StringVar(&protocol, "protocol", "tcp", "tunnel protocol: tcp|udp")
	flag.BoolVar(&https, "https", false, "enable HTTPS CONNECT proxy mode")

	flag.BoolVar(&enableWS, "ws", false, "enable WebSocket transport")
	flag.StringVar(&wsPath, "ws-path", "/ws", "WebSocket path")
	flag.BoolVar(&wsTLS, "ws-tls", false, "enable WebSocket TLS")
	flag.BoolVar(&wsSkipVerify, "ws-skip-verify", false, "skip TLS certificate verify")

	flag.StringVar(&configFile, "c", "", "config file path (short)")
	flag.StringVar(&configFile, "config", "", "config file path")
	flag.BoolVar(&secureDelete, "secure-delete", false, "secure delete config file")
	flag.StringVar(&genConfig, "gen-config", "", "generate sample config file")

	flag.BoolVar(&daemonMode, "d", false, "daemon mode (short)")
	flag.BoolVar(&daemonMode, "daemon", false, "daemon mode")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "h", false, "show help")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CS_Tunnel Client v%s - C2 encrypted tunnel\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s -l <listen_addr> -s <server_addr> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Quick examples:\n")
		fmt.Fprintf(os.Stderr, "  %s -l 127.0.0.1:443 -s vps.example.com:8888 -p mypass\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -l :443 -s vps:8888 -ws\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -c client.yaml\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
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
			fmt.Fprintf(os.Stderr, "failed to generate config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("sample config file generated: %s\n", genConfig)
		return
	}

	if daemonMode {
		if err := daemon.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "daemonize failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if configFile != "" {
		runFromConfig(configFile, secureDelete)
		return
	}

	if listen == "" || serverAddr == "" {
		fmt.Fprintf(os.Stderr, "error: must provide listen (-l) and server (-s)\n\n")
		fmt.Fprintf(os.Stderr, "quick example:\n")
		fmt.Fprintf(os.Stderr, "  %s -l 127.0.0.1:443 -s vps.example.com:8888\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "use -h for help\n")
		os.Exit(1)
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = wsPath
	wsConfig.EnableTLS = wsTLS
	wsConfig.SkipVerify = wsSkipVerify

	runClient(listen, serverAddr, target, protocol, password, https, enableWS, wsConfig)
}

func runFromConfig(configPath string, secureDelete bool) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file: %v\n", err)
		os.Exit(1)
	}

	if cfg.Mode != "" && cfg.Mode != "client" {
		fmt.Fprintln(os.Stderr, "the mode in config file is not client")
		os.Exit(1)
	}

	if cfg.Client.Daemon {
		if err := daemon.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "daemonize failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if secureDelete {
		if err := config.SecureDeleteConfigFile(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to secure delete config file: %v\n", err)
		}
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = cfg.Client.WSPath
	wsConfig.EnableTLS = cfg.Client.WSTLS
	wsConfig.SkipVerify = cfg.Client.WSSkipVerify

	proto := cfg.Client.Protocol
	if proto == "" {
		proto = "tcp"
	}

	runClient(cfg.Client.Listen, cfg.Client.Server, cfg.Client.Target, proto,
		cfg.Client.Password, cfg.Client.EnableHTTPS, cfg.Client.EnableWS, wsConfig)
}

func runClient(listen, serverAddr, target, protocol, password string, https, enableWS bool, wsConfig transport.WSConfig) {
	cfg := client.Config{
		ListenAddr:   listen,
		ServerAddr:   serverAddr,
		TargetAddr:   target,
		Protocol:     protocol,
		Password:     password,
		EnableHTTPS:  https,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableWS:     enableWS,
		WSConfig:     wsConfig,
	}

	cli, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client: %v\n", err)
		os.Exit(1)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cli.Stop()
		os.Exit(0)
	}()

	if err := cli.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "client start failed: %v\n", err)
		os.Exit(1)
	}
}
