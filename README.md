# C2_Tunnel

一个基于 Go 的加密隧道工具，用于在 Client 与 Server 之间建立加密转发通道，支持 TCP 与 WebSocket（WS/WSS）传输，支持 ACL 访问控制。

## 功能概览

- AES-256-CFB 加密传输
- 支持 TCP 隧道与 WebSocket 隧道
- 支持 HTTPS CONNECT 代理场景（Client 端）
- 支持 ACL 白名单/黑名单/混合模式（Server 端）
- 支持配置文件（JSON / YAML）
- 支持配置文件删除与安全删除
- 支持 Server 端日志、静默模式、后台运行

## 目录结构

```text
cmd/
  client/        Client 启动入口
  server/        Server 启动入口
pkg/
  acl/           ACL 访问控制
  client/        Client 核心逻辑
  config/        配置加载与示例生成
  crypto/        加密与加密连接封装
  daemon/        后台运行支持
  logger/        日志模块
  server/        Server 核心逻辑
  transport/     WebSocket 传输层
```

## 快速开始

### 1. 编译

```bash
# 编译 Server
go build -ldflags="-s -w" -o tunnel-server ./cmd/server

# 编译 Client
go build -ldflags="-s -w" -o tunnel-client ./cmd/client
```

### 2. 启动 Server

```bash
./tunnel-server -l 0.0.0.0:8888 -t 127.0.0.1:50050 -p "YourPass"
```

### 3. 启动 Client

```bash
./tunnel-client -l 127.0.0.1:443 -s vps.example.com:8888 -p "YourPass"
```

## 运行模式

### TCP 模式

- Client 监听本地端口，接收本地流量
- 将流量加密后转发到 Server
- Server 解密后转发到目标地址

### WebSocket 模式

```bash
# Server
./tunnel-server -l :8888 -t 127.0.0.1:50050 -p "YourPass" -ws -ws-path /ws

# Client
./tunnel-client -l 127.0.0.1:443 -s vps.example.com:8888 -p "YourPass" -ws -ws-path /ws
```

## 命令行参数

### Server 参数（`tunnel-server`）

- `-l, -listen`：监听地址（必填）
- `-t, -target`：目标地址（必填）
- `-p, -password`：加密密码
- `-ws`：启用 WebSocket
- `-ws-path`：WebSocket 路径（默认 `/ws`）
- `-ws-tls`：启用 WSS
- `-ws-cert`：TLS 证书路径
- `-ws-key`：TLS 私钥路径
- `-acl`：启用 ACL
- `-acl-mode`：ACL 模式（`whitelist` / `blacklist` / `both`）
- `-acl-whitelist`：白名单（逗号分隔）
- `-acl-blacklist`：黑名单（逗号分隔）
- `-c, -config`：配置文件路径
- `-delete-config`：启动后删除配置文件
- `-secure-delete`：启动后安全删除配置文件
- `-gen-config`：生成示例配置文件
- `-log`：日志文件路径
- `-d, -daemon`：后台运行
- `-q, -quiet`：静默模式（不输出终端）
- `-v, -version`：显示版本
- `-h`：帮助

### Client 参数（`tunnel-client`）

- `-l, -listen`：监听地址（必填）
- `-s, -server`：Server 地址（必填）
- `-t, -target`：默认目标地址（可选）
- `-p, -password`：加密密码
- `-https`：启用 HTTPS CONNECT 代理模式
- `-ws`：启用 WebSocket
- `-ws-path`：WebSocket 路径（默认 `/ws`）
- `-ws-tls`：启用 WSS
- `-ws-skip-verify`：跳过证书校验
- `-c, -config`：配置文件路径
- `-delete-config`：启动后删除配置文件
- `-secure-delete`：启动后安全删除配置文件
- `-gen-config`：生成示例配置文件
- `-d, -daemon`：后台运行
- `-v, -version`：显示版本
- `-h`：帮助

## 配置文件

### Server 示例（YAML）

```yaml
mode: server

server:
  listen: 0.0.0.0:8888
  target: 127.0.0.1:50050
  password: YourSecurePassword

  enable_ws: false
  ws_path: /ws
  ws_tls: false
  ws_cert: ""
  ws_key: ""

  acl:
    enable: false
    mode: both
    whitelist: []
    blacklist: []

  log_path: /var/log/tunnel-server.log
  daemon: false
  quiet: false
```

### Client 示例（YAML）

```yaml
mode: client

client:
  listen: 127.0.0.1:443
  server: vps.example.com:8888
  target: ""
  password: YourSecurePassword

  enable_https: false

  enable_ws: false
  ws_path: /ws
  ws_tls: false
  ws_skip_verify: false

  # 注意：当前版本 Client 已移除日志功能，以下字段保留但不生效
  log_path: ""
  quiet: false

  daemon: false
```

## 安全建议

- 强密码并定期更换
- 公网部署优先使用 WSS（`-ws-tls`）
- 为 Server 配置 ACL
- 配置文件中避免明文长期保存敏感信息
- 建议结合 `-secure-delete` 使用一次性配置文件

## 近期维护更新（2026-03-19）

- Client 端移除全部日志记录逻辑
- Client 端移除日志相关参数：`-log`、`-q`、`-quiet`
- 修复加密帧部分写入导致的协议不完整问题
- 修复 WebSocket ACL 可被伪造请求头绕过的问题
- ACL 模式新增严格校验，非法值改为拒绝
- 修复 Client 双向转发可能卡住的连接回收问题

## 开发与验证

```bash
go test ./...
go vet ./...
```
