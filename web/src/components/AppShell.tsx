import { useState, type ReactNode } from "react";
import { Link } from "wouter";
import {
  Activity,
  Boxes,
  ChevronDown,
  CircleGauge,
  FileClock,
  History,
  Group,
  LogOut,
  Menu,
  Network,
  Rss,
  Route,
  Server,
  Settings,
  ShieldCheck,
  Users,
  Waypoints,
  X,
} from "lucide-react";
import type { Admin } from "../types";

const groups = [
  { label: "", items: [{ to: "/", label: "总览", icon: CircleGauge }] },
  {
    label: "基础设施",
    items: [
      { to: "/machines", label: "服务器", icon: Server },
      { to: "/nodes", label: "节点", icon: Boxes },
    ],
  },
  {
    label: "访问控制",
    items: [
      { to: "/users", label: "订阅账号", icon: Users },
      { to: "/plans", label: "套餐", icon: ShieldCheck },
      { to: "/access-groups", label: "权限组", icon: Group },
      { to: "/subscription", label: "订阅", icon: Rss },
    ],
  },
  {
    label: "流量策略",
    items: [
      { to: "/routes", label: "路由策略", icon: Route },
      { to: "/outbounds", label: "出站", icon: Waypoints },
    ],
  },
  {
    label: "系统",
    items: [
      { to: "/history", label: "历史数据", icon: History },
      { to: "/settings", label: "系统设置", icon: Settings },
      { to: "/audit", label: "审计日志", icon: FileClock },
    ],
  },
];

export function AppShell({
  admin,
  demo,
  onLogout,
  children,
}: {
  admin: Admin;
  demo: boolean;
  onLogout: () => void;
  children: ReactNode;
}) {
  const [mobileOpen, setMobileOpen] = useState(false);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>
      <header className="mobile-header">
        <button
          className="icon-button icon-button--dark"
          aria-label="打开导航"
          onClick={() => setMobileOpen(true)}
        >
          <Menu size={20} />
        </button>
        <Brand />
        {demo && <span className="demo-label">演示数据</span>}
      </header>
      <aside className={`sidebar ${mobileOpen ? "sidebar--open" : ""}`}>
        <div className="sidebar__brand">
          <Brand />
          <button
            className="icon-button icon-button--dark sidebar__close"
            aria-label="关闭导航"
            onClick={() => setMobileOpen(false)}
          >
            <X size={20} />
          </button>
        </div>
        <div className="sidebar__health">
          <Activity size={15} />
          <span>控制服务</span>
          <strong>正常</strong>
        </div>
        <nav aria-label="主导航">
          {groups.map((group) => (
            <div className="nav-group" key={group.label || "root"}>
              {group.label && (
                <div className="nav-group__label">{group.label}</div>
              )}
              {group.items.map((item) => (
                <Link
                  key={item.to}
                  to={item.to}
                  onClick={() => setMobileOpen(false)}
                  className={(isActive) =>
                    `nav-link ${isActive ? "nav-link--active" : ""}`
                  }
                >
                  <item.icon size={17} aria-hidden="true" />
                  <span>{item.label}</span>
                </Link>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar__account">
          <div className="account-avatar">{admin.name.slice(0, 1)}</div>
          <div className="account-copy">
            <strong>{admin.name}</strong>
            <span>{admin.email}</span>
          </div>
          <button
            className="icon-button icon-button--dark"
            aria-label="退出登录"
            title="退出登录"
            onClick={onLogout}
          >
            <LogOut size={17} />
          </button>
        </div>
      </aside>
      {mobileOpen && (
        <button
          className="mobile-scrim"
          aria-label="关闭导航"
          onClick={() => setMobileOpen(false)}
        />
      )}
      <main className="app-main" id="main-content">
        {demo && (
          <div className="demo-banner">
            <Network size={15} />
            <span>当前使用演示数据，操作不会写入数据库。</span>
            <ChevronDown size={14} />
          </div>
        )}
        {children}
      </main>
    </div>
  );
}

function Brand() {
  return (
    <div className="brand">
      <span className="brand__mark">C</span>
      <span>
        <strong>CPanel</strong>
        <small>设备管理平台</small>
      </span>
    </div>
  );
}
