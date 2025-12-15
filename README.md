# SecureTunnel - AES-256-CFB 加密隧道

一个基于 Go 语言的安全隧道工具，专为 CobaltStrike 等 C2 框架设计，提供 AES-256-CFB 加密传输。

## ✨ v1.2.0 新功能

- 📄 **配置文件支持** - 支持 YAML/JSON 配置文件，启动后可自动删除
- 🛡️ **IP 黑白名单** - Server 端支持 IP/CIDR 访问控制
- 🔒 **安全删除** - 配置文件覆写后删除，防止恢复

## 📋 功能特点

- **AES-256-CFB 加密**: 所有传输数据均经过 AES-256-CFB 加密
- **双向加密**: 请求和响应均加密传输
- **HTTPS CONNECT 代理**: 支持 HTTP/HTTPS CONNECT 代理模式
- **WebSocket 传输**: 支持 WS/WSS 协议，流量更隐蔽
- **配置文件**: 支持 YAML/JSON 配置，启动后自动删除
- **访问控制**: Server 端支持 IP 黑白名单
- **高并发**: 基于 Go 协程，支持大量并发连接
- **跨平台**: 支持 Windows、Linux、macOS

## 🏗️ 架构设计

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Owner Client   │     │   Proxy Client  │     │   Proxy Server  │     │  Owner Server   │
│   (Beacon)      │────▶│   (本地/跳板)    │────▶│    (VPS)        │────▶│  (TeamServer)   │
│                 │◀────│                 │◀────│                 │◀────│                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └─────────────────┘
                              │                        │
                              └────── AES-256-CFB ─────┘
                                   加密传输通道
```

## 🚀 快速开始

### 编译

```bash
# 编译当前平台
go build -ldflags="-s -w" -o tunnel.exe ./cmd/tunnel

# 交叉编译 Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tunnel_linux ./cmd/tunnel
```

---

## 📄 配置文件模式

### 生成示例配置文件

```bash
# 生成 YAML 格式
tunnel -gen-config config.yaml

# 生成 JSON 格式
tunnel -gen-config config.json
```

### 使用配置文件启动

```bash
# 普通启动
tunnel -config config.yaml

# 启动后删除配置文件
tunnel -config config.yaml -delete-config

# 安全删除配置文件 (覆写后删除，防止数据恢复)
tunnel -config config.yaml -secure-delete
```

### 配置文件示例

**Server 配置 (server.yaml):**

```yaml
mode: server

server:
  listen: "0.0.0.0:8888"
  target: "127.0.0.1:50050"
  password: "YourSecurePassword@2024"
  
  # WebSocket 配置
  enable_ws: false
  ws_path: "/ws"
  
  # 访问控制
  acl:
    enable: true
    mode: "whitelist"  # whitelist 或 blacklist
    whitelist:
      - "192.168.1.0/24"
      - "10.0.0.0/8"
      - "127.0.0.1"
    blacklist:
      - "192.168.1.100"
```

**Client 配置 (client.yaml):**

```yaml
mode: client

client:
  listen: "127.0.0.1:443"
  server: "vps.example.com:8888"
  password: "YourSecurePassword@2024"
  enable_https: false
  enable_ws: false
```

---

## 🛡️ IP 黑白名单 (ACL)

Server 端支持基于 IP 的访问控制：

### 白名单模式

只允许名单内的 IP 连接：

```bash
tunnel -mode server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass \
  -acl -acl-mode whitelist -acl-whitelist "192.168.1.0/24,10.0.0.1,127.0.0.1"
```

### 黑名单模式

拒绝名单内的 IP 连接：

```bash
tunnel -mode server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password mypass \
  -acl -acl-mode blacklist -acl-blacklist "192.168.1.100,10.10.0.0/16"
```

### 支持的格式

- 单个 IP: `192.168.1.100`
- CIDR 格式: `192.168.1.0/24`
- 多个条目: 用逗号分隔

---

## 📡 TCP 模式

### Server 端

```bash
./tunnel -mode server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password "YourPass"
```

### Client 端

```bash
./tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:8888 -password "YourPass"
```

---

## 🌐 WebSocket 模式

### Server 端

```bash
# 基础 WebSocket
./tunnel -mode server -listen 0.0.0.0:80 -target 127.0.0.1:50050 -password "YourPass" \
  -ws -ws-path /api/stream

# WebSocket + TLS
./tunnel -mode server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password "YourPass" \
  -ws -ws-tls -ws-cert cert.pem -ws-key key.pem
```

### Client 端

```bash
# 基础 WebSocket
./tunnel -mode client -listen 127.0.0.1:443 -server vps.com:80 -password "YourPass" \
  -ws -ws-path /api/stream

# WebSocket + TLS
./tunnel -mode client -listen 127.0.0.1:443 -server vps.com:443 -password "YourPass" \
  -ws -ws-tls -ws-skip-verify
```

---

## 📖 完整参数列表

### 基础参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-mode` | 运行模式 (server/client) | - |
| `-listen` | 监听地址 | - |
| `-target` | 目标地址 | - |
| `-server` | Server 地址 (Client) | - |
| `-password` | 加密密码 | SecureTunnel@2024 |
| `-https` | HTTPS CONNECT 代理 | false |

### 配置文件参数

| 参数 | 说明 |
|------|------|
| `-config` | 配置文件路径 |
| `-gen-config` | 生成示例配置文件 |
| `-delete-config` | 启动后删除配置文件 |
| `-secure-delete` | 安全删除 (覆写后删除) |

### WebSocket 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-ws` | 启用 WebSocket | false |
| `-ws-path` | WebSocket 路径 | /ws |
| `-ws-tls` | 启用 TLS | false |
| `-ws-cert` | TLS 证书路径 | - |
| `-ws-key` | TLS 密钥路径 | - |
| `-ws-skip-verify` | 跳过证书验证 | false |

### ACL 参数 (Server)

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-acl` | 启用访问控制 | false |
| `-acl-mode` | 模式 (whitelist/blacklist) | whitelist |
| `-acl-whitelist` | 白名单 (逗号分隔) | - |
| `-acl-blacklist` | 黑名单 (逗号分隔) | - |

---

## 📁 项目结构

```
Tunnel/
├── cmd/tunnel/main.go           # 主程序入口
├── pkg/
│   ├── acl/acl.go               # IP 黑白名单模块
│   ├── config/config.go         # 配置文件模块
│   ├── crypto/crypto.go         # AES 加解密模块
│   ├── client/client.go         # Client 端实现
│   ├── server/server.go         # Server 端实现
│   └── transport/websocket.go   # WebSocket 传输模块
├── examples/                     # 配置文件示例
│   ├── server.yaml
│   ├── client.yaml
│   ├── server_websocket.yaml
│   └── config.json
├── docs/
│   └── 公众号文章.md
├── go.mod
└── README.md
```

---

## 🛡️ 安全说明

- 请使用强密码 (建议 16+ 字符)
- 密码通过 SHA-256 转换为 AES 密钥
- 每个数据包使用随机 IV
- 使用 `-secure-delete` 可安全删除配置文件
- 建议启用 ACL 限制访问来源

---

## 📝 License

MIT License
