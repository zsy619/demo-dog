# `deploy/` —— 反向代理与运行时配置

该目录包含在反向代理后运行 demo-dog 所需的全部配置。
我们提供了两套并行的实现方案,可根据既有基础设施自由选择。

## 文件清单

| 文件 | 用途 | 对应关系 |
|---|---|---|
| `nginx.conf` | 仅前端的 nginx 配置(`:80`,上游 `backend:8080`) | —— |
| `nginx-fullstack.conf` | 单镜像 nginx 配置(`:8080`,上游 `127.0.0.1:8080`) | —— |
| `Caddyfile` | 仅前端的 Caddy 配置,与 `nginx.conf` 对应 | `nginx.conf` |
| `Caddyfile.fullstack` | 单镜像 Caddy 配置,与 `nginx-fullstack.conf` 对应 | `nginx-fullstack.conf` |
| `Caddyfile.https` | 生产环境 Caddy 配置,内置 Let's Encrypt 自动签发 | —— |
| `Dockerfile.caddy` | 多阶段 Dockerfile,用于构建 Caddy 镜像 | —— |
| `start-all.sh` | `--target all`(nginx)镜像的 supervisord 启动脚本 | —— |

## 各配置的作用

每份配置都承担以下三种职责:

1. **反向代理 `/api/*`** 到 Go 采集器。
2. **反向代理 `/api/stream`**,并启用 WebSocket 升级。
3. **托管 React SPA** 静态资源,通过 `try_files` 回退到 `index.html`,
   使前端路由可正常工作。

此外:

- `/assets/*` 设置 `Cache-Control: public, max-age=2592000,
  immutable`(Vite 会输出带内容哈希的文件名)。
- `/healthz` 直接返回 `ok`,供容器或编排器健康检查使用。

## nginx 与 Caddy 的取舍

| 选择 nginx | 选择 Caddy |
|---|---|
| 已有 nginx 镜像或 Ingress | 希望零配置启用自动 HTTPS(Let's Encrypt) |
| 需要 `mod_security` 等 nginx 专属模块 | 开箱即用 HTTP/3 + brotli + zstd |
| 团队熟悉 nginx 配置语法 | 偏好单文件 DSL,而非 50 行的 nginx 块 |

两套方案都运行同一份后端二进制,托管同一份前端静态资源。

## 切换方式

### 使用 nginx compose 栈(默认)

```bash
make compose-up
# 或直接:
docker compose up --build
```

### 使用 Caddy compose 栈

```bash
make compose-up-caddy
# 或直接:
docker compose -f docker-compose.caddy.yml up --build
```

### 在公网域名上使用 Caddy 启用真实 HTTPS

1. 编辑 `deploy/Caddyfile.https`,将 `demo-dog.example.com` 和
   `ops@example.com` 替换为真实域名与联系人邮箱。
2. 将一条 DNS A/AAAA 记录指向 Caddy 所在的主机。
3. 将 `deploy/Caddyfile.https` 挂载为容器内的
   `/etc/caddy/Caddyfile`(官方 `caddy` 镜像默认已如此处理)。
4. 暴露 `80` 与 `443` 端口。Caddy 会在首次请求时自动申请证书。

## nginx ↔ Caddy 配置对照

下表将 `nginx.conf` 中的每条指令映射到 `deploy/Caddyfile` 中的 Caddy 等价写法。

| nginx | Caddy |
|---|---|
| `listen 80`(server 块) | `:80 { ... }`(site 块) |
| `server_name _` | 省略 —— `:80` 默认监听所有主机名 |
| `root /usr/share/nginx/html` | `root * /srv` |
| `index index.html` | 隐含(file_server 默认行为) |
| `gzip on; gzip_types ...;` | `encode gzip zstd` |
| `location /api/ { proxy_pass http://dog_backend; ... }` | `@api path /api/*` + `reverse_proxy @api backend:8080` |
| `proxy_http_version 1.1; proxy_set_header Connection "";` | 隐含 —— Caddy 默认使用 HTTP/1.1 并启用 keep-alive |
| `proxy_set_header Host $host;` | `header_up Host {host}` |
| `proxy_set_header X-Real-IP $remote_addr;` | `header_up X-Real-IP {remote_host}` |
| `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;` | `header_up X-Forwarded-For {header.X-Forwarded-For}` |
| `proxy_set_header X-Forwarded-Proto $scheme;` | `header_up X-Forwarded-Proto {scheme}` |
| `proxy_set_header Upgrade $http_upgrade; Connection "upgrade";` | 隐含 —— Caddy 自动识别 WebSocket 升级 |
| `proxy_read_timeout 60s;` | `transport http { read_timeout 60s }` |
| `proxy_read_timeout 3600s;`(/api/stream 使用) | `transport http { read_timeout 3600s; write_timeout 3600s }` |
| `upstream dog_backend { server backend:8080; keepalive 16; }` | `reverse_proxy @api backend:8080 { lb_policy round_robin; ... }` |
| `location / { try_files $uri $uri/ /index.html; }` | `@spa not path /api/* /assets/* /healthz` + `try_files {path} /index.html` |
| `location /assets/ { expires 30d; add_header Cache-Control ...; }` | `@assets path /assets/*` + `header @assets Cache-Control "public, max-age=2592000, immutable"` |
| `location = /healthz { return 200 "ok\n"; }` | `@healthz path /healthz` + `respond \`ok\` 200` |
| `access_log off;` | `log { ... }`(按站点配置) |

## 配置校验

若本机已安装 `caddy` 可执行文件,可执行:

```bash
make validate-caddyfile
# 或
caddy adapt --config deploy/Caddyfile         --pretty
caddy adapt --config deploy/Caddyfile.fullstack --pretty
caddy adapt --config deploy/Caddyfile.https     --pretty
```

`caddy adapt` 会输出 Caddy 将加载的 JSON 形式;无报错地完成转换即代表文件语法正确。

nginx 可使用:

```bash
nginx -t -c deploy/nginx.conf
```
