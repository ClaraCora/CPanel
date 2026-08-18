# CPanel

CPanel 是面向 Corade Agent 的轻量设备管理平台，提供服务器、节点、权限组、路由、套餐、用户和系统设置管理。

## 管理后台加载边界

未登录时浏览器只加载登录页面、会话检查和登录请求所需的公开入口，不加载节点、流量、订阅、服务器或系统设置等后台页面与演示数据。管理员会话验证成功后，浏览器才会请求 `/assets/secure/` 下的后台代码块；服务端会再次校验管理员 HttpOnly Cookie，未登录请求返回 `401 Unauthorized`。受保护代码块使用私有缓存，退出登录后页面会完整刷新并卸载后台模块。

生产构建不会启用 `?demo=1` 演示模式；该参数仅供本地 Vite 开发预览使用。

## 一键部署

支持 Debian 12 和 Ubuntu 22.04 及以上版本。使用 root 用户执行：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPP/main/install.sh | sudo bash
```

默认仅监听 `127.0.0.1:8256`，不会直接暴露到公网。配置 HTTPS 反向代理后，更新 `/etc/cpanel/cpanel.env`：

```dotenv
CPANEL_ADDR=127.0.0.1:8256
CPANEL_EXTERNAL_URL=https://panel.example.com
CPANEL_COOKIE_SECURE=true
```

然后运行 `systemctl restart cpanel`。反向代理的上游地址应设置为 `http://127.0.0.1:8256`。

脚本会自动安装并初始化 PostgreSQL、下载已校验的 CPanel 与 Corade Agent 文件、创建 systemd 服务，并生成首个管理员密码。公开脚本与二进制发行均由 CPP 仓库提供。也可以预先指定站点和管理员信息：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPP/main/install.sh | \
  sudo CPANEL_ADMIN_PASSWORD='replace-with-a-strong-password' bash -s -- \
  --external-url https://panel.example.com \
  --admin-email admin@example.com \
  --admin-name 管理员
```

升级到仓库最新构建：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPP/main/install.sh | sudo bash -s -- upgrade
```

安装完成后可通过以下命令管理服务：

```bash
systemctl status cpanel
journalctl -u cpanel -f
```

环境配置保存在 `/etc/cpanel/cpanel.env`，运行数据和 Agent 文件保存在 `/var/lib/cpanel`。反向代理 HTTPS 后，应将 `CPANEL_EXTERNAL_URL` 设置为公开 HTTPS 地址并将 `CPANEL_COOKIE_SECURE` 设置为 `true`。

## 套餐流量重置

套餐可以选择“自然月重置”或“不自动重置”。自然月使用“系统设置 → 站点”中的时区，在每月 1 日 `00:00` 将套餐账号的已用流量校准为新月份的日流量汇总；面板启动、套餐策略变更、账号列表读取以及后台每分钟校准都会执行该逻辑。选择“不自动重置”后会保留账号当前已用流量并继续累计。

总览、TG Bot 本月排行与自然月套餐账号均以 `traffic_daily` 和导入日流量为统一数据源。升级自旧版本时，已有自然月套餐保持原策略，并在服务启动后自动剔除上月累计，不需要手工清零。

## Google/YouTube 指定出口分流（Xray）

以下配置用于让 Google 和 YouTube 流量通过指定代理出站，其余流量保持直连。该方案要求节点使用 Xray，并将 Agent 升级到 `v2.0.2` 或更高版本。新版本 Agent 会对 HTTP、TLS 和 QUIC 启用仅用于路由判断的嗅探，并在规则需要时自动下载 `geoip.dat` 和 `geosite.dat`，不需要在节点配置中手工填写嗅探 JSON。

推荐将路由策略的“默认流量出口”设置为“默认直连（仅按下方规则处理）”，然后只添加一条 Google 分流规则：

| 配置项 | 填写内容 |
| --- | --- |
| 规则名称 | `Google 系分流` |
| 精确域名 | `geosite:google, geosite:youtube` |
| 域名后缀 | 使用下方完整列表 |
| GeoIP 分类 | `google` |
| 执行动作 | 指定出站 |
| 出站目标 | 选择实际的落地出站，例如 `us.sjc-rn` |

域名后缀列表：

```text
google.com, google.com.hk, google.cn, googleapis.com, gstatic.com,
googleusercontent.com, ggpht.com, gvt1.com, gvt2.com, 1e100.net,
recaptcha.net, youtube.com, youtube-nocookie.com, youtu.be,
googlevideo.com, ytimg.com, gmail.com, googlemail.com, google.dev,
firebaseio.com, firebaseapp.com, doubleclick.net, googlesyndication.com,
googleadservices.com, googletagmanager.com, google-analytics.com
```

GeoIP 分类只填写 `google`，不要填写 `youtube`。YouTube 使用 Google 网络地址，而当前 GeoIP 数据通常没有独立的 `youtube` 分类；填写不存在的分类可能导致 Xray 无法加载路由配置。`geosite:youtube` 属于域名分类，应填写在“精确域名”中。

当默认出口已经是直连时，不需要再添加 TCP/UDP 直连规则。路由未匹配 Google 规则后会自动使用默认直连。如果业务上必须把默认出口设置为落地，则需要在 Google 规则后增加第二条规则：

1. 网络协议同时选择 `TCP` 和 `UDP`。
2. 执行动作选择“直连”。

此时规则顺序必须是“Google 指定出站”在前、“TCP/UDP 直连”在后。路由策略按顺序匹配，顺序颠倒会导致 Google 流量先命中直连规则。

保存策略后，节点会自动收到新配置。首次使用 GeoIP/GeoSite 时，Agent 日志应出现数据库下载完成和 Xray 重启成功的信息：

```bash
journalctl -u corade -f
```

正常日志会包含 `geo database ready`、`xray started` 和 `config updated`，且不应出现 `this rule has no effective fields` 或 GeoIP/GeoSite 分类不存在的错误。产生实际流量后，Google 请求应显示为 `[vless-in -> 指定出站标记]`，非 Google 请求应显示为 `[vless-in -> direct]`。

## `/ca/jk/` 健康检查

`/ca/jk/` 是 CPanel 面板进程的本机健康检查路径，不是 Agent 通讯接口，也不应对公网开放。Agent 使用的通讯路径是 `/ca/cc`。

| 路径 | 作用 | 正常状态码 |
| --- | --- | --- |
| `GET /ca/jk/ch` | 存活检查：确认 CPanel 进程能够响应 HTTP 请求 | `204 No Content` |
| `GET /ca/jk/jx` | 就绪检查：确认 CPanel 进程和 PostgreSQL 数据库均可正常工作 | `204 No Content` |

当数据库尚未就绪时，`/ca/jk/jx` 返回 `503 Service Unavailable`。安装和升级脚本会在重启服务后请求此端点，只有返回成功才会判定部署完成。

这两个端点同时校验请求来源和 `Host`：仅接受来自本机回环地址，且 `Host` 为 `127.0.0.1`、`::1` 或 `localhost` 的请求。其他请求固定返回 `404 Not Found`。请勿把它们作为公网监控地址；如需健康监控，应在面板主机本机执行。

## 新部署后的操作

1. 确认服务已启动，并在面板主机本机检查存活和就绪状态：

```bash
systemctl status cpanel --no-pager
curl -i http://127.0.0.1:8256/ca/jk/ch
curl -i http://127.0.0.1:8256/ca/jk/jx
```

两个 `curl` 请求均应返回 `204 No Content`。如果 `/ca/jk/jx` 返回 `503`，使用 `journalctl -u cpanel -n 100 --no-pager` 检查面板或数据库启动错误，不要继续配置公网入口。

2. 在 `/etc/cpanel/cpanel.env` 中设置正式域名，然后重启 CPanel：

```dotenv
CPANEL_ADDR=127.0.0.1:8256
CPANEL_EXTERNAL_URL=https://panel.example.com
CPANEL_COOKIE_SECURE=true
```

```bash
systemctl restart cpanel
```

3. 配置 HTTPS 反向代理。公网代理必须先屏蔽整个 `/ca/jk/`，再把其他请求转发至 `127.0.0.1:8256`。

Nginx 的 `server` 配置示例：

```nginx
location ^~ /ca/jk/ {
    return 404;
}

location / {
    proxy_pass http://127.0.0.1:8256;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

Caddy 配置示例：

```caddy
panel.example.com {
    @private_health path /ca/jk/*
    respond @private_health 404
    reverse_proxy 127.0.0.1:8256
}
```

4. 重载反向代理后，从另一台机器验证公网无法访问健康检查路径：

```bash
curl -o /dev/null -s -w '%{http_code}\n' https://panel.example.com/ca/jk/ch
curl -o /dev/null -s -w '%{http_code}\n' https://panel.example.com/ca/jk/jx
```

两个请求均应输出 `404`。随后访问正式域名登录后台，在“系统设置”中确认外部控制地址和统一通讯密钥，再到“服务器”页面复制该服务器生成的 Agent 安装命令。不要手工拼接 Agent 参数，页面生成的命令会包含服务器 ID、首次登记通讯密钥和面板公钥。

## TG Bot

TG Bot 使用 Telegram Bot API 长轮询，不需要配置 Webhook。BotFather 密钥使用面板加密密钥加密保存，日志和设置读取接口不会返回密钥明文。面板服务器需要能够访问 `api.telegram.org:443`。

首次配置顺序：

1. 在 Telegram 的 `@BotFather` 创建 Bot 并取得完整密钥。
2. 打开“系统设置 → TG Bot”，填写密钥、开启 TG Bot，管理员 Telegram ID 暂时留空并保存。
3. 私聊该 Bot 发送 `/id`，将返回的数字填写到“管理员 Telegram ID”并再次保存。
4. 点击“发送测试消息”确认连接。只有绑定的管理员私聊能够查询面板数据，其他账号只能使用 `/id`。

支持的查询命令：

| 命令 | 内容 |
| --- | --- |
| `/status` | 服务器、节点、订阅账号和今日总流量 |
| `/traffic` | 今日上传、下载和合计流量 |
| `/today` | 今日节点与用户流量 Top 5 |
| `/machines` | 服务器在线状态与最后心跳 |
| `/ranking` | 昨日节点与用户流量 Top 5 |
| `/month` | 本月用户使用量 Top 10（本月 1 日至今） |
| `/id` | 当前 Telegram 数字 ID |
| `/help` | 命令说明 |

Bot 回复使用 Telegram HTML 等宽表格。排行榜会使用 `G`、`T` 等紧凑流量单位并控制在 36 个显示单位以内，状态、名称、上传、下载和合计列会按中英文显示宽度独立对齐；过长名称会在表格内截断，不会在手机端挤压合计列换行。

“推送昨日排行榜”启用后，面板按“系统设置 → 站点”的时区，在配置时间推送昨日上传、下载、节点 Top 5 和用户 Top 5。发送日期和 Telegram update offset 会持久化，面板重启不会重复处理旧消息或重复推送当日日报。

## 订阅访问控制

“访问控制 → 订阅”提供常规浏览器拦截开关、UA 白名单和最近 200 条订阅拉取记录。拦截默认关闭，开启后 Chrome、Safari、Firefox、Edge 等常规浏览器 UA 会收到 `404 Not Found`；Clash、sing-box、Shadowrocket 等非浏览器客户端不受影响。UA 白名单按行配置，使用不区分大小写的关键字匹配。

拉取记录包含匹配到的订阅账号、结果、HTTP 状态、客户端 IP、User-Agent 和精确时间，不保存订阅令牌。记录默认保留 30 天，可在“系统设置 → 数据保留”中修改。

面板仅在请求直接来自本机回环地址或“系统设置 → 安全”中的可信代理 CIDR 时解析 `X-Forwarded-For`。按上方 Nginx 示例反向代理时，无需额外配置；经过其他代理层时，应将代理地址或网段加入可信代理 CIDR，否则记录中会显示直接连接面板的代理 IP。

已确认的产品范围记录在 [项目详情.md](./项目详情.md)。

## Development prerequisites

- Go 1.26+
- Node.js 24+
- PostgreSQL 16+

## Commands

```powershell
# Use the workspace-local Go toolchain on this machine.
$go = '.\.tools\go\bin\go.exe'

# Build the administrator UI embedded by the Go server.
Push-Location .\web
npm ci
npm run typecheck
npm run build
Pop-Location

# Run all backend and embedded UI handler tests.
& $go test ./...

# Produce the single deployable binary.
New-Item -ItemType Directory -Force .\bin | Out-Null
& $go build -trimpath -o .\bin\cpanel.exe .\cmd\cpanel
```

For frontend development, run `npm run dev -- --port 5173` from `web` and open
`http://127.0.0.1:5173/?demo=1` to use the explicit in-browser demo data.
