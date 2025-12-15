# SecureTunnel - AES-256-CFB 加密隧道

一个基于 Go 语言的安全隧道工具，专为 CobaltStrike 等 C2 框架设计，提供 AES-256-CFB 加密传输。

## ✨ v1.1.0 新功能

- 🌐 **WebSocket 传输模式** - 流量伪装为正常 WebSocket 通信
- 🔒 **WSS (WebSocket over TLS)** - 支持 TLS 加密的 WebSocket
- 🎭 **伪装页面** - 非 WebSocket 请求返回正常网页

## 📋 功能特点

- **AES-256-CFB 加密**: 所有传输数据均经过 AES-256-CFB 加密
- **双向加密**: 请求和响应均加密传输
- **HTTPS CONNECT 代理**: 支持 HTTP/HTTPS CONNECT 代理模式
- **WebSocket 传输**: 支持 WS/WSS 协议，流量更隐蔽
- **高并发**: 基于 Go 协程，支持大量并发连接
- **跨平台**: 支持 Windows、Linux、macOS

## 🏗️ 架构设计

### TCP 模式
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Owner Client   │     │   Proxy Client  │     │   Proxy Server  │     ┌─────────────────┐
│   (Beacon)      │────▶│   (本地/跳板)    │────▶│    (VPS)        │────▶│  Owner Server   │
│                 │◀────│                 │◀────│                 │◀────│  (TeamServer)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └─────────────────┘
                              │                        │
                              └────── AES-256-CFB ─────┘
                                   TCP 加密传输
```

### WebSocket 模式
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Owner Client   │     │   Proxy Client  │     │   Proxy Server  │     ┌─────────────────┐
│   (Beacon)      │────▶│   (本地/跳板)    │────▶│    (VPS)        │────▶│  Owner Server   │
│                 │◀────│                 │◀────│                 │◀────│  (TeamServer)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └─────────────────┘
                              │                        │
                              └─── WebSocket + AES ────┘
                                流量伪装为正常 WS 通信
```

## 🚀 快速开始

### 编译

```bash
# 编译当前平台
go build -ldflags="-s -w" -o tunnel.exe ./cmd/tunnel

# 交叉编译 Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o tunnel_linux ./cmd/tunnel

# 使用编译脚本
./build.bat   # Windows
./build.sh    # Linux/macOS
```

---

## 📡 TCP 模式 (传统加密隧道)

### Server 端部署 (VPS)

```bash
./tunnel -mode server -listen 0.0.0.0:8888 -target 127.0.0.1:50050 -password "YourSecretPassword"
```

### Client 端部署 (本地/跳板机)

**直接转发模式** (用于 CobaltStrike Beacon):
```bash
./tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:8888 -password "YourSecretPassword"
```

**HTTPS 代理模式** (用于浏览器/工具):
```bash
./tunnel -mode client -listen 127.0.0.1:8080 -server vps.example.com:8888 -password "YourSecretPassword" -https
```

---

## 🌐 WebSocket 模式 (流量伪装)

WebSocket 模式让隧道流量看起来像正常的 WebSocket 通信，更难被检测。

### Server 端 - WebSocket 模式

```bash
# 基础 WebSocket
./tunnel -mode server -listen 0.0.0.0:80 -target 127.0.0.1:50050 -password "YourPass" -ws -ws-path /chat

# WebSocket + TLS (推荐)
./tunnel -mode server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password "YourPass" -ws -ws-tls -ws-cert cert.pem -ws-key key.pem
```

### Client 端 - WebSocket 模式

```bash
# 基础 WebSocket
./tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:80 -password "YourPass" -ws -ws-path /chat

# WebSocket + TLS
./tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:443 -password "YourPass" -ws -ws-tls

# 跳过证书验证 (自签名证书)
./tunnel -mode client -listen 127.0.0.1:443 -server vps.example.com:443 -password "YourPass" -ws -ws-tls -ws-skip-verify
```

---

## 📖 参数说明

### 基础参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-mode` | 运行模式 | `server` 或 `client` |
| `-listen` | 监听地址 | `0.0.0.0:8888` |
| `-target` | 目标地址 (Server模式) | `127.0.0.1:50050` |
| `-server` | Server 地址 (Client模式) | `vps.example.com:8888` |
| `-password` | 加密密码 | `YourSecretPassword` |
| `-https` | 启用 HTTPS CONNECT 代理 | 无需参数 |

### WebSocket 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-ws` | 启用 WebSocket 模式 | `false` |
| `-ws-path` | WebSocket 路径 | `/ws` |
| `-ws-tls` | 启用 TLS (wss://) | `false` |
| `-ws-cert` | TLS 证书路径 | - |
| `-ws-key` | TLS 密钥路径 | - |
| `-ws-skip-verify` | 跳过证书验证 (Client) | `false` |

---

## 🔧 CobaltStrike 配置示例

### 方案一：TCP 模式

```bash
# VPS: TeamServer + 隧道 Server
./teamserver 127.0.0.1 password
./tunnel -mode server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password "MySecurePass!"

# 跳板机: 隧道 Client
./tunnel -mode client -listen 0.0.0.0:443 -server <VPS_IP>:443 -password "MySecurePass!"
```

### 方案二：WebSocket 模式 (推荐)

```bash
# VPS: TeamServer + WebSocket Server
./teamserver 127.0.0.1 password
./tunnel -mode server -listen 0.0.0.0:443 -target 127.0.0.1:50050 -password "MySecurePass!" -ws -ws-path /api/v1/stream

# 跳板机: WebSocket Client
./tunnel -mode client -listen 0.0.0.0:443 -server <VPS_IP>:443 -password "MySecurePass!" -ws -ws-path /api/v1/stream
```

---

## 🛡️ 安全说明

- 请使用强密码 (建议 16+ 字符)
- 密码通过 SHA-256 转换为 AES 密钥
- 每个数据包使用随机 IV
- WebSocket 模式数据额外使用 Base64 编码
- 建议使用 WSS (WebSocket over TLS) 模式

---

## 📁 项目结构

```
Tunnel/
├── cmd/
│   └── tunnel/
│       └── main.go              # 主程序入口
├── pkg/
│   ├── crypto/
│   │   └── crypto.go            # AES 加解密模块
│   ├── client/
│   │   └── client.go            # Client 端实现
│   ├── server/
│   │   └── server.go            # Server 端实现
│   └── transport/
│       └── websocket.go         # WebSocket 传输模块
├── docs/
│   └── 公众号文章.md             # 技术文档
├── build.bat                    # Windows 编译脚本
├── build.sh                     # Linux 编译脚本
├── go.mod
└── README.md
```

---

## 📝 License

MIT License
