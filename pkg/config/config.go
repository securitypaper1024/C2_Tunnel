package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode   string       `json:"mode" yaml:"mode"`
	Server ServerConfig `json:"server" yaml:"server"`
	Client ClientConfig `json:"client" yaml:"client"`
}

type ServerConfig struct {
	Listen   string `json:"listen" yaml:"listen"`
	Target   string `json:"target" yaml:"target"`
	Password string `json:"password" yaml:"password"`

	EnableWS bool   `json:"enable_ws" yaml:"enable_ws"`
	WSPath   string `json:"ws_path" yaml:"ws_path"`
	WSTLS    bool   `json:"ws_tls" yaml:"ws_tls"`
	WSCert   string `json:"ws_cert" yaml:"ws_cert"`
	WSKey    string `json:"ws_key" yaml:"ws_key"`

	ACL ACLConfig `json:"acl" yaml:"acl"`
}

type ClientConfig struct {
	Listen   string `json:"listen" yaml:"listen"`
	Server   string `json:"server" yaml:"server"`
	Target   string `json:"target" yaml:"target"`
	Password string `json:"password" yaml:"password"`

	EnableHTTPS bool `json:"enable_https" yaml:"enable_https"`

	EnableWS     bool   `json:"enable_ws" yaml:"enable_ws"`
	WSPath       string `json:"ws_path" yaml:"ws_path"`
	WSTLS        bool   `json:"ws_tls" yaml:"ws_tls"`
	WSSkipVerify bool   `json:"ws_skip_verify" yaml:"ws_skip_verify"`
}

type ACLConfig struct {
	Enable    bool     `json:"enable" yaml:"enable"`
	Mode      string   `json:"mode" yaml:"mode"`
	Whitelist []string `json:"whitelist" yaml:"whitelist"`
	Blacklist []string `json:"blacklist" yaml:"blacklist"`
}

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
		if err := json.Unmarshal(data, config); err != nil {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config (tried JSON and YAML): %w", err)
			}
		}
	}

	return config, nil
}

func DeleteConfigFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	return nil
}

func SecureDeleteConfigFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open config file for overwrite: %w", err)
	}

	zeros := make([]byte, info.Size())
	for i := range zeros {
		zeros[i] = 0x00
	}
	file.Write(zeros)
	file.Sync()
	file.Close()

	return os.Remove(path)
}

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

func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Listen:   "127.0.0.1:443",
		Password: "SecureTunnel@2024",
		WSPath:   "/ws",
	}
}

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

func GenerateServerExampleConfig() *Config {
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
	}
}

func GenerateClientExampleConfig() *Config {
	return &Config{
		Mode: "client",
		Client: ClientConfig{
			Listen:       "127.0.0.1:443",
			Server:       "vps.example.com:8888",
			Password:     "YourSecurePassword@2024",
			EnableHTTPS:  false,
			EnableWS:     false,
			WSPath:       "/ws",
			WSSkipVerify: false,
		},
	}
}

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
