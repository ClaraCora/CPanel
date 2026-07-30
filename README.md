# CPanel

CPanel 是面向 Corade Agent 的轻量设备管理平台，提供服务器、节点、权限组、路由、套餐、用户和系统设置管理。

## 一键部署

支持 Debian 12 和 Ubuntu 22.04 及以上版本。使用 root 用户执行：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPanel/main/install.sh | sudo bash
```

默认仅监听 `127.0.0.1:8256`，不会直接暴露到公网。配置 HTTPS 反向代理后，更新 `/etc/cpanel/cpanel.env`：

```dotenv
CPANEL_ADDR=127.0.0.1:8256
CPANEL_EXTERNAL_URL=https://panel.example.com
CPANEL_COOKIE_SECURE=true
```

然后运行 `systemctl restart cpanel`。反向代理的上游地址应设置为 `http://127.0.0.1:8256`。

脚本会自动安装并初始化 PostgreSQL、下载已校验的 CPanel 与 CPanelde Agent 文件、创建 systemd 服务，并生成首个管理员密码。也可以预先指定站点和管理员信息：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPanel/main/install.sh | \
  sudo CPANEL_ADMIN_PASSWORD='replace-with-a-strong-password' bash -s -- \
  --external-url https://panel.example.com \
  --admin-email admin@example.com \
  --admin-name 管理员
```

升级到仓库最新构建：

```bash
curl -fsSL https://raw.githubusercontent.com/ClaraCora/CPanel/main/install.sh | sudo bash -s -- upgrade
```

安装完成后可通过以下命令管理服务：

```bash
systemctl status cpanel
journalctl -u cpanel -f
```

环境配置保存在 `/etc/cpanel/cpanel.env`，运行数据和 Agent 文件保存在 `/var/lib/cpanel`。反向代理 HTTPS 后，应将 `CPANEL_EXTERNAL_URL` 设置为公开 HTTPS 地址并将 `CPANEL_COOKIE_SECURE` 设置为 `true`。

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
