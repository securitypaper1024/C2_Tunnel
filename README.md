# C2_Tunnel

一个基于 Go 的加密隧道工具，用于在 Client 与 Server 之间建立安全转发通道。
当前支持：
- 业务协议：`tcp`、`udp`
- 传输承载：TCP、WebSocket（WS/WSS）

## 功能特性

- AES-256-CFB 加密传输
- `TCP over TCP` / `TCP over WS`
- `UDP over TCP` / `UDP over WS`
- Client 支持 HTTPS CONNECT（仅 TCP 业务协议）
- Server 支持 ACL（白名单 / 黑名单 / 混合模式）
- 支持 JSON/YAML 配置文件
- 支持配置文件安全删除（`-secure-delete`）
- Server 支持日志、静默、后台运行

## 项目结构

```text
cmd/
  client/        Client 启动入口
  server/        Server 启动入口
pkg/
  acl/           ACL 访问控制
  client/        Client 核心逻辑（含 UDP 会话转发）
  config/        配置加载与示例生成
  crypto/        加密与连接封装
  daemon/        后台运行支持
  logger/        日志模块
  server/        Server 核心逻辑（含 UDP 桥接）
  transport/     WebSocket 传输层
scripts/
  e2e_local.go   本机端到端联调脚本（4 种链路）
```

## 快速开始

### 1. 编译

```bash
# Server
go build -ldflags="-s -w" -o tunnel-server ./cmd/server

# Client
go build -ldflags="-s -w" -o tunnel-client ./cmd/client
```

### 2. TCP 业务协议（默认）

```bash
# Server
./tunnel-server -l 0.0.0.0:8888 -t 127.0.0.1:50050 -protocol tcp -p "YourPass"

# Client
./tunnel-client -l 127.0.0.1:443 -s vps.example.com:8888 -protocol tcp -p "YourPass"
```

### 3. UDP 业务协议

```bash
# Server（示例把 UDP 53 暴露到客户端）
./tunnel-server -l 0.0.0.0:8888 -t 127.0.0.1:53 -protocol udp -p "YourPass"

# Client
./tunnel-client -l 127.0.0.1:5300 -s vps.example.com:8888 -protocol udp -p "YourPass"
```

## WebSocket 模式

### TCP over WS

```bash
# Server
./tunnel-server -l :8888 -t 127.0.0.1:50050 -protocol tcp -p "YourPass" -ws -ws-path /ws

# Client
./tunnel-client -l 127.0.0.1:443 -s vps.example.com:8888 -protocol tcp -p "YourPass" -ws -ws-path /ws
```

### UDP over WS

```bash
# Server
./tunnel-server -l :8888 -t 127.0.0.1:53 -protocol udp -p "YourPass" -ws -ws-path /ws

# Client
./tunnel-client -l 127.0.0.1:5300 -s vps.example.com:8888 -protocol udp -p "YourPass" -ws -ws-path /ws
```

## 命令行参数

### Server（`tunnel-server`）

- `-l, -listen`：监听地址（必填）
- `-t, -target`：目标地址（必填）
- `-protocol`：业务协议 `tcp|udp`（默认 `tcp`）
- `-p, -password`：加密密码
- `-ws`：启用 WebSocket 承载
- `-ws-path`：WebSocket 路径（默认 `/ws`）
- `-ws-tls`：启用 WSS
- `-ws-cert`：TLS 证书路径
- `-ws-key`：TLS 私钥路径
- `-acl`：启用 ACL
- `-acl-mode`：`whitelist|blacklist|both`
- `-acl-whitelist`：白名单（逗号分隔）
- `-acl-blacklist`：黑名单（逗号分隔）
- `-c, -config`：配置文件路径
- `-secure-delete`：启动后安全删除配置文件
- `-gen-config`：生成示例配置文件
- `-log`：日志文件路径
- `-d, -daemon`：后台运行
- `-q, -quiet`：静默模式
- `-v, -version`：显示版本
- `-h`：帮助

### Client（`tunnel-client`）

- `-l, -listen`：监听地址（必填）
- `-s, -server`：Server 地址（必填）
- `-t, -target`：默认目标地址（可选）
- `-protocol`：业务协议 `tcp|udp`（默认 `tcp`）
- `-p, -password`：加密密码
- `-https`：启用 HTTPS CONNECT（仅 TCP）
- `-ws`：启用 WebSocket 承载
- `-ws-path`：WebSocket 路径（默认 `/ws`）
- `-ws-tls`：启用 WSS
- `-ws-skip-verify`：跳过证书校验
- `-c, -config`：配置文件路径
- `-secure-delete`：启动后安全删除配置文件
- `-gen-config`：生成示例配置文件
- `-d, -daemon`：后台运行
- `-v, -version`：显示版本
- `-h`：帮助

## 配置文件示例

### Server（YAML）

```yaml
mode: server

server:
  listen: 0.0.0.0:8888
  target: 127.0.0.1:50050
  protocol: tcp   # tcp | udp
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

### Client（YAML）

```yaml
mode: client

client:
  listen: 127.0.0.1:443
  server: vps.example.com:8888
  target: ""
  protocol: tcp   # tcp | udp
  password: YourSecurePassword

  enable_https: false

  enable_ws: false
  ws_path: /ws
  ws_tls: false
  ws_skip_verify: false

  # 当前版本 Client 已移除日志功能，以下字段保留但不生效
  log_path: ""
  quiet: false

  daemon: false
```

## 本机 E2E 联调

已提供脚本：`scripts/e2e_local.go`。

脚本会自动验证 4 条链路：
- `TCP over TCP`
- `TCP over WS`
- `UDP over TCP`
- `UDP over WS`

执行：

```bash
go run ./scripts/e2e_local.go
```

成功输出应包含：
- `[E2E] TCP over TCP passed`
- `[E2E] TCP over WS passed`
- `[E2E] UDP over TCP passed`
- `[E2E] UDP over WS passed`
- `[E2E] all cases passed`

## 安全建议

- 使用强密码并定期轮换
- 公网部署优先使用 WSS（`-ws-tls`）
- 为 Server 配置 ACL
- 避免长期明文保存敏感配置
- 建议结合 `-secure-delete` 使用一次性配置

## 最近更新（2026-03-19）

- 新增 UDP 业务协议支持（含 UDP over TCP / UDP over WS）
- 新增 `-protocol tcp|udp` 参数与配置字段 `protocol`
- 仅保留 `-secure-delete`，移除 `-delete-config`
- Client 端移除日志输出与日志参数
- 修复加密帧部分写入风险
- 修复 WebSocket ACL 头伪造绕过问题
- ACL 模式增加严格校验（非法值拒绝）
- 修复 Client 双向转发可能卡住的问题

## 开发验证

```bash
go test ./...
go vet ./...
```
