package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 完整配置结构
type Config struct {
	Mode   string       `json:"mode" yaml:"mode"`     // server 或 client
	Server ServerConfig `json:"server" yaml:"server"` // Server 配置
	Client ClientConfig `json:"client" yaml:"client"` // Client 配置
}

// ServerConfig Server 端配置
type ServerConfig struct {
	Listen   string `json:"listen" yaml:"listen"`     // 监听地址
	Target   string `json:"target" yaml:"target"`     // 目标地址
	Password string `json:"password" yaml:"password"` // 加密密码

	// WebSocket 配置
	EnableWS     bool   `json:"enable_ws" yaml:"enable_ws"`
	WSPath       string `json:"ws_path" yaml:"ws_path"`
	WSTLS        bool   `json:"ws_tls" yaml:"ws_tls"`
	WSCert       string `json:"ws_cert" yaml:"ws_cert"`
	WSKey        string `json:"ws_key" yaml:"ws_key"`

	// 访问控制
	ACL ACLConfig `json:"acl" yaml:"acl"`
}

// ClientConfig Client 端配置
type ClientConfig struct {
	Listen   string `json:"listen" yaml:"listen"`     // 本地监听地址
	Server   string `json:"server" yaml:"server"`     // Server 端地址
	Target   string `json:"target" yaml:"target"`     // 目标地址 (可选)
	Password string `json:"password" yaml:"password"` // 加密密码

	// HTTPS 代理模式
	EnableHTTPS bool `json:"enable_https" yaml:"enable_https"`

	// WebSocket 配置
	EnableWS     bool   `json:"enable_ws" yaml:"enable_ws"`
	WSPath       string `json:"ws_path" yaml:"ws_path"`
	WSTLS        bool   `json:"ws_tls" yaml:"ws_tls"`
	WSSkipVerify bool   `json:"ws_skip_verify" yaml:"ws_skip_verify"`
}

// ACLConfig 访问控制配置
type ACLConfig struct {
	Enable    bool     `json:"enable" yaml:"enable"`       // 是否启用 ACL
	Mode      string   `json:"mode" yaml:"mode"`           // whitelist 或 blacklist
	Whitelist []string `json:"whitelist" yaml:"whitelist"` // 白名单 IP/CIDR
	Blacklist []string `json:"blacklist" yaml:"blacklist"` // 黑名单 IP/CIDR
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	ext := filepath.Ext(path)

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		// 尝试 JSON，失败则尝试 YAML
		if err := json.Unmarshal(data, config); err != nil {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config (tried JSON and YAML): %w", err)
			}
		}
	}

	return config, nil
}

// DeleteConfigFile 删除配置文件
func DeleteConfigFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	return nil
}

// SecureDeleteConfigFile 安全删除配置文件 (覆写后删除)
func SecureDeleteConfigFile(path string) error {
	// 获取文件大小
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	// 打开文件进行覆写
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open config file for overwrite: %w", err)
	}

	// 用随机数据覆写
	zeros := make([]byte, info.Size())
	for i := range zeros {
		zeros[i] = 0x00
	}
	file.Write(zeros)
	file.Sync()
	file.Close()

	// 删除文件
	return os.Remove(path)
}

// DefaultServerConfig 默认 Server 配置
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Listen:   "0.0.0.0:8888",
		Target:   "127.0.0.1:50050",
		Password: "SecureTunnel@2024",
		WSPath:   "/ws",
		ACL: ACLConfig{
			Enable: false,
			Mode:   "whitelist",
		},
	}
}

// DefaultClientConfig 默认 Client 配置
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Listen:   "127.0.0.1:443",
		Password: "SecureTunnel@2024",
		WSPath:   "/ws",
	}
}

// GenerateExampleConfig 生成示例配置
func GenerateExampleConfig() *Config {
	return &Config{
		Mode: "server",
		Server: ServerConfig{
			Listen:   "0.0.0.0:8888",
			Target:   "127.0.0.1:50050",
			Password: "YourSecurePassword@2024",
			EnableWS: false,
			WSPath:   "/ws",
			WSTLS:    false,
			ACL: ACLConfig{
				Enable: true,
				Mode:   "whitelist",
				Whitelist: []string{
					"192.168.1.0/24",
					"10.0.0.0/8",
					"127.0.0.1",
				},
				Blacklist: []string{
					"192.168.1.100",
				},
			},
		},
		Client: ClientConfig{
			Listen:      "127.0.0.1:443",
			Server:      "vps.example.com:8888",
			Password:    "YourSecurePassword@2024",
			EnableHTTPS: false,
			EnableWS:    false,
			WSPath:      "/ws",
		},
	}
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, path string) error {
	ext := filepath.Ext(path)
	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
	default:
		data, err = json.MarshalIndent(config, "", "  ")
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

