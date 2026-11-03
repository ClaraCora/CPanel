import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import { Boxes, ChevronDown, CircleGauge, FileClock, Fingerprint, Group, History, LogOut, Menu, Network, Rss, Route, Server, Settings, ShieldCheck, Users, Waypoints, X } from "lucide-react";
import type { Admin } from "../types";
import { AdminThemeToggle } from "./AdminTheme";
import { useGuardedAction } from "./AdminNavigation";
import { RowMenu } from "./ui";

const primary = [
  { to: "/", label: "总览", icon: CircleGauge },
  { to: "/machines", label: "服务器", icon: Server },
  { to: "/nodes", label: "节点", icon: Boxes },
];
const groups = [
  { label: "账号与订阅", items: [
    { to: "/users", label: "订阅账号", icon: Users },
    { to: "/plans", label: "套餐", icon: ShieldCheck },
    { to: "/access-groups", label: "权限组", icon: Group },
    { to: "/subscription", label: "订阅", icon: Rss },
    { to: "/access-ips", label: "访问 IP", icon: Fingerprint },
  ] },
  { label: "流量策略", items: [{ to: "/routes", label: "路由策略", icon: Route }, { to: "/outbounds", label: "出站", icon: Waypoints }] },
  { label: "系统", items: [{ to: "/history", label: "历史数据", icon: History }, { to: "/settings", label: "系统设置", icon: Settings }, { to: "/audit", label: "审计日志", icon: FileClock }] },
];

export function AppShell({ admin, demo, onLogout, children }: { admin: Admin; demo: boolean; onLogout: () => void; children: ReactNode }) {
  const [location] = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const header = useRef<HTMLElement>(null);
  const request = useGuardedAction();
  const active = (to: string) => to === "/" ? location === "/" : location === to || location.startsWith(to + "/");
  useEffect(() => { setMobileOpen(false); }, [location]);
  useEffect(() => {
    if (!mobileOpen) return;
    header.current?.querySelector<HTMLElement>(".admin-mobile-navigation a")?.focus();
    const close = (event: KeyboardEvent) => { if (event.key === "Escape") { setMobileOpen(false); menuButton.current?.focus(); } };
    const outside = (event: PointerEvent) => { if (!header.current?.contains(event.target as Node)) setMobileOpen(false); };
    document.addEventListener("keydown", close);
    document.addEventListener("pointerdown", outside);
    return () => { document.removeEventListener("keydown", close); document.removeEventListener("pointerdown", outside); };
  }, [mobileOpen]);

  return <div className="admin-shell">
    <a className="skip-link" href="#main-content">跳到主要内容</a>
    <header className="admin-header" ref={header}>
      <div className="admin-header__inner">
        <Link to="/" className="admin-brand" aria-label="CPanel 总览"><span className="admin-brand__mark">C</span><strong>CPanel</strong></Link>
        <nav className="admin-desktop-navigation" aria-label="主导航">
          {primary.map((item) => <Link key={item.to} to={item.to} className={`admin-nav-item ${active(item.to) ? "is-active" : ""}`} aria-current={active(item.to) ? "page" : undefined}>{item.label}</Link>)}
          {groups.map((group) => <RowMenu key={group.label} label={group.label} triggerClassName={`admin-nav-item ${group.items.some((item) => active(item.to)) ? "is-active" : ""}`} trigger={<>{group.label}<ChevronDown size={14} aria-hidden="true" /></>}>
            {group.items.map((item) => <Link key={item.to} to={item.to} aria-current={active(item.to) ? "page" : undefined}><item.icon size={16} aria-hidden="true" />{item.label}</Link>)}
          </RowMenu>)}
        </nav>
        <div className="admin-header__tools">
          {demo && <span className="admin-demo-label">演示数据</span>}
          <AdminThemeToggle />
          <RowMenu label="管理员菜单" triggerClassName="admin-account" trigger={<><span className="admin-avatar">{admin.name.slice(0, 1)}</span><span className="admin-account__name">{admin.name}</span><ChevronDown size={14} aria-hidden="true" /></>}>
            <Link to="/settings?section=account"><Settings size={16} />账户设置</Link>
            <button type="button" onClick={() => request(onLogout)}><LogOut size={16} />退出登录</button>
          </RowMenu>
          <button ref={menuButton} type="button" className="icon-button admin-menu-toggle" aria-label={mobileOpen ? "关闭导航" : "打开导航"} aria-expanded={mobileOpen} aria-controls="admin-mobile-navigation" onClick={() => setMobileOpen((value) => !value)}>{mobileOpen ? <X size={20} /> : <Menu size={20} />}</button>
        </div>
      </div>
      {mobileOpen && <nav id="admin-mobile-navigation" className="admin-mobile-navigation" aria-label="移动主导航">
        <div className="admin-mobile-navigation__group">{primary.map((item) => <Link key={item.to} to={item.to} className={active(item.to) ? "is-active" : ""} aria-current={active(item.to) ? "page" : undefined} onClick={() => setMobileOpen(false)}><item.icon size={17} />{item.label}</Link>)}</div>
        {groups.map((group) => <div key={group.label} className="admin-mobile-navigation__group"><strong>{group.label}</strong>{group.items.map((item) => <Link key={item.to} to={item.to} className={active(item.to) ? "is-active" : ""} aria-current={active(item.to) ? "page" : undefined} onClick={() => setMobileOpen(false)}><item.icon size={17} />{item.label}</Link>)}</div>)}
      </nav>}
    </header>
    <main className="admin-main" id="main-content">
      {demo && <div className="admin-demo-banner"><Network size={15} aria-hidden="true" /><span>当前使用演示数据，操作不会写入数据库。</span></div>}
      {children}
    </main>
  </div>;
}
