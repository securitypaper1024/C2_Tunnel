package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tunnel/pkg/acl"
	"tunnel/pkg/config"
	"tunnel/pkg/daemon"
	"tunnel/pkg/logger"
	"tunnel/pkg/server"
	"tunnel/pkg/transport"
)

const Version = "1.4.0"

func main() {
	var listen, target, password string
	var enableWS, wsTLS bool
	var wsPath, wsCert, wsKey string
	var configFile, logPath string
	var secureDelete, daemonMode, quiet, showVersion, showHelp bool
	var genConfig string
	var aclEnable bool
	var aclMode, aclWhitelist, aclBlacklist string

	flag.StringVar(&listen, "l", "", "鐩戝惉鍦板潃 (绠€鍐?")
	flag.StringVar(&listen, "listen", "", "鐩戝惉鍦板潃")
	flag.StringVar(&target, "t", "", "鐩爣鍦板潃 (绠€鍐?")
	flag.StringVar(&target, "target", "", "鐩爣鍦板潃")
	flag.StringVar(&password, "p", "SecureTunnel@2024", "鍔犲瘑瀵嗙爜 (绠€鍐?")
	flag.StringVar(&password, "password", "SecureTunnel@2024", "鍔犲瘑瀵嗙爜")

	flag.BoolVar(&enableWS, "ws", false, "鍚敤 WebSocket 浼犺緭妯″紡")
	flag.StringVar(&wsPath, "ws-path", "/ws", "WebSocket 璺緞")
	flag.BoolVar(&wsTLS, "ws-tls", false, "鍚敤 WebSocket TLS")
	flag.StringVar(&wsCert, "ws-cert", "", "TLS 璇佷功鏂囦欢璺緞")
	flag.StringVar(&wsKey, "ws-key", "", "TLS 瀵嗛挜鏂囦欢璺緞")

	flag.StringVar(&configFile, "c", "", "閰嶇疆鏂囦欢璺緞 (绠€鍐?")
	flag.StringVar(&configFile, "config", "", "閰嶇疆鏂囦欢璺緞")
	flag.BoolVar(&secureDelete, "secure-delete", false, "瀹夊叏鍒犻櫎閰嶇疆鏂囦欢")
	flag.StringVar(&genConfig, "gen-config", "", "鐢熸垚绀轰緥閰嶇疆鏂囦欢")

	flag.BoolVar(&aclEnable, "acl", false, "鍚敤璁块棶鎺у埗")
	flag.StringVar(&aclMode, "acl-mode", "both", "ACL 妯″紡: whitelist/blacklist/both")
	flag.StringVar(&aclWhitelist, "acl-whitelist", "", "鐧藉悕鍗?(閫楀彿鍒嗛殧)")
	flag.StringVar(&aclBlacklist, "acl-blacklist", "", "榛戝悕鍗?(閫楀彿鍒嗛殧)")

	flag.StringVar(&logPath, "log", "", "鏃ュ織鏂囦欢璺緞")
	flag.BoolVar(&daemonMode, "d", false, "鍚庡彴杩愯妯″紡 (绠€鍐?")
	flag.BoolVar(&daemonMode, "daemon", false, "鍚庡彴杩愯妯″紡")
	flag.BoolVar(&quiet, "q", false, "闈欓粯妯″紡 (绠€鍐?")
	flag.BoolVar(&quiet, "quiet", false, "quiet mode, no terminal output")
	flag.BoolVar(&showVersion, "v", false, "鏄剧ず鐗堟湰淇℃伅")
	flag.BoolVar(&showVersion, "version", false, "鏄剧ず鐗堟湰淇℃伅")
	flag.BoolVar(&showHelp, "h", false, "鏄剧ず甯姪淇℃伅")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "CS_Tunnel Server v%s - C2 娴侀噺鍔犲瘑闅ч亾\n\n", Version)
		fmt.Fprintf(os.Stderr, "鐢ㄦ硶:\n")
		fmt.Fprintf(os.Stderr, "  %s -l <鐩戝惉鍦板潃> -t <鐩爣鍦板潃> [閫夐」]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "蹇€熺ず渚?\n")
		fmt.Fprintf(os.Stderr, "  %s -l 0.0.0.0:8888 -t 127.0.0.1:50050 -p mypass\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -l :8888 -t :50050 -ws                    # WebSocket妯″紡\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -l :8888 -t :50050 -acl -acl-whitelist 10.0.0.0/8 -acl-blacklist 10.1.1.1\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -c server.yaml                            # 浣跨敤閰嶇疆鏂囦欢\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "閫夐」:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		return
	}

	if showVersion {
		fmt.Printf("CS_Tunnel Server v%s\n", Version)
		return
	}

	if genConfig != "" {
		cfg := config.GenerateServerExampleConfig()
		if err := config.SaveConfig(cfg, genConfig); err != nil {
			fmt.Fprintf(os.Stderr, "鐢熸垚閰嶇疆鏂囦欢澶辫触: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("绀轰緥閰嶇疆鏂囦欢宸茬敓鎴? %s\n", genConfig)
		return
	}

	if daemonMode {
		if err := daemon.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "鍚庡彴杩愯澶辫触: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := logger.InitLogger(logPath, quiet); err != nil {
		fmt.Fprintf(os.Stderr, "鍒濆鍖栨棩蹇楀け璐? %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	if configFile != "" {
		runFromConfig(configFile, secureDelete)
		return
	}

	if listen == "" || target == "" {
		fmt.Fprintf(os.Stderr, "閿欒: 蹇呴』鎸囧畾鐩戝惉鍦板潃(-l)鍜岀洰鏍囧湴鍧€(-t)\n\n")
		fmt.Fprintf(os.Stderr, "蹇€熺ず渚?\n")
		fmt.Fprintf(os.Stderr, "  %s -l 0.0.0.0:8888 -t 127.0.0.1:50050\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "浣跨敤 -h 鏌ョ湅甯姪\n")
		os.Exit(1)
	}

	wsConfig := transport.DefaultWSConfig()
	wsConfig.Path = wsPath
	wsConfig.EnableTLS = wsTLS
	wsConfig.TLSCert = wsCert
	wsConfig.TLSKey = wsKey

	aclConfig := acl.Config{
		Enable: aclEnable,
		Mode:   aclMode,
	}
	if aclWhitelist != "" {
		aclConfig.Whitelist = splitAndTrim(aclWhitelist)
	}
	if aclBlacklist != "" {
		aclConfig.Blacklist = splitAndTrim(aclBlacklist)
	}

	runServer(listen, target, password, enableWS, wsConfig, aclConfig)
}

func runFromConfig(configPath string, secureDelete bool) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Fatalf("鍔犺浇閰嶇疆鏂囦欢澶辫触: %v", err)
	}

	if cfg.Mode != "" && cfg.Mode != "server" {
		logger.Fatalf("閰嶇疆鏂囦欢涓殑 mode 涓嶆槸 server")
	}

	if cfg.Server.Daemon {
		if err := daemon.Daemonize(); err != nil {
			logger.Fatalf("鍚庡彴杩愯澶辫触: %v", err)
		}
		os.Exit(0)
	}

	if cfg.Server.LogPath != "" || cfg.Server.Quiet {
		if err := logger.InitLogger(cfg.Server.LogPath, cfg.Server.Quiet); err != nil {
			logger.Fatalf("鍒濆鍖栨棩蹇楀け璐? %v", err)
		}
	}

	logger.Printf("[Config] 鍔犺浇閰嶇疆鏂囦欢: %s", configPath)

	if secureDelete {
		logger.Printf("[Config] 瀹夊叏鍒犻櫎閰嶇疆鏂囦欢...")
		if err := config.SecureDeleteConfigFile(configPath); err != nil {
			logger.Printf("[Config] 瀹夊叏鍒犻櫎澶辫触: %v", err)
		} else {
			logger.Printf("[Config] config file securely deleted")
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
		logger.Fatalf("鍒涘缓 Server 澶辫触: %v", err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Println("姝ｅ湪鍏抽棴 Server...")
		srv.Stop()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		logger.Fatalf("Server 鍚姩澶辫触: %v", err)
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
