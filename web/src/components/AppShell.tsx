import { useEffect, useRef, useState, type ReactNode } from "react";
import { Link, useLocation } from "wouter";
import { Boxes, ChevronDown, CircleGauge, FileClock, Fingerprint, Group, History, LogOut, Menu, Network, Rss, Route, Server, Settings, ShieldCheck, Users, Waypoints } from "lucide-react";
import type { Admin } from "../types";
import { AdminThemeToggle } from "./AdminTheme";
import { useGuardedAction } from "./AdminNavigation";
import { DetailDialog, RowMenu } from "./ui";

const primary = [
  { to: "/", label: "总览", icon: CircleGauge },
  { to: "/machines", label: "服务器", icon: Server },
  { to: "/nodes", label: "节点", icon: Boxes },
];
const groups = [
  { label: "账号与订阅", icon: Users, items: [
    { to: "/users", label: "订阅账号", icon: Users },
    { to: "/plans", label: "套餐", icon: ShieldCheck },
    { to: "/access-groups", label: "权限组", icon: Group },
    { to: "/subscription", label: "订阅", icon: Rss },
    { to: "/access-ips", label: "访问 IP", icon: Fingerprint },
  ] },
  { label: "流量策略", icon: Route, items: [{ to: "/routes", label: "路由策略", icon: Route }, { to: "/outbounds", label: "出站", icon: Waypoints }] },
  { label: "系统", icon: Settings, items: [{ to: "/history", label: "历史数据", icon: History }, { to: "/settings", label: "系统设置", icon: Settings }, { to: "/audit", label: "审计日志", icon: FileClock }] },
];

export function AppShell({ admin, demo, onLogout, children }: { admin: Admin; demo: boolean; onLogout: () => void; children: ReactNode }) {
  const [location] = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const desktopNav = useRef<HTMLElement>(null);
  const request = useGuardedAction();
  const active = (to: string) => to === "/" ? location === "/" : location === to || location.startsWith(to + "/");
  useEffect(() => { setMobileOpen(false); }, [location]);
  useEffect(() => {
    if (!mobileOpen) return;
    function closeOnDesktop() {
      if (window.innerWidth < 1050) return;
      setMobileOpen(false);
      const nav = desktopNav.current;
      (nav?.querySelector<HTMLElement>(".is-active") ?? nav?.querySelector<HTMLElement>("a"))?.focus({ preventScroll: true });
    }
    window.addEventListener("resize", closeOnDesktop);
    return () => window.removeEventListener("resize", closeOnDesktop);
  }, [mobileOpen]);

  function drawerLink(item: typeof primary[number]) {
    const current = active(item.to);
    return <Link key={item.to} to={item.to} className={current ? "is-active" : undefined} aria-current={current ? "page" : undefined} onClick={() => setMobileOpen(false)}><item.icon size={17} aria-hidden="true" />{item.label}</Link>;
  }

  return <div className="admin-shell">
    <a className="skip-link" href="#main-content">跳到主要内容</a>
    <header className="admin-header">
      <div className="admin-header__inner">
        <button ref={menuButton} type="button" className="icon-button admin-menu-toggle" aria-label="打开导航" aria-haspopup="dialog" aria-expanded={mobileOpen} aria-controls={mobileOpen ? "admin-mobile-navigation" : undefined} onClick={() => { menuButton.current?.focus({ preventScroll: true }); setMobileOpen(true); }}><Menu size={20} aria-hidden="true" /></button>
        <Link to="/" className="admin-brand" aria-label="CPanel 总览"><span className="admin-brand__mark">C</span><strong>CPanel</strong></Link>
        <nav ref={desktopNav} className="admin-desktop-navigation" aria-label="主导航">
          <span className="workspace-nav-label" aria-hidden="true">工作区</span>
          {primary.map((item) => <Link key={item.to} to={item.to} className={`admin-nav-item ${active(item.to) ? "is-active" : ""}`} aria-current={active(item.to) ? "page" : undefined}><item.icon className="nav-icon" size={17} aria-hidden="true" />{item.label}</Link>)}
          {groups.map((group) => <RowMenu key={group.label} label={group.label} fallbackFocus={menuButton} triggerClassName={`admin-nav-item ${group.items.some((item) => active(item.to)) ? "is-active" : ""}`} trigger={<><group.icon className="nav-icon" size={17} aria-hidden="true" />{group.label}<ChevronDown size={14} aria-hidden="true" /></>}>
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
        </div>
      </div>
    </header>
    <DetailDialog open={mobileOpen} className="admin-navigation-drawer" title="主导航" onClose={() => setMobileOpen(false)}>
      <nav id="admin-mobile-navigation" className="admin-mobile-navigation" aria-label="移动主导航">
        <div className="admin-mobile-navigation__group">{primary.map(drawerLink)}</div>
        {groups.map((group) => <div key={group.label} className="admin-mobile-navigation__group" role="group" aria-label={group.label}>
          <strong aria-hidden="true">{group.label}</strong>{group.items.map(drawerLink)}
        </div>)}
      </nav>
    </DetailDialog>
    <main className="admin-main" id="main-content">
      {demo && <div className="admin-demo-banner"><Network size={15} aria-hidden="true" /><span>当前使用演示数据，操作不会写入数据库。</span></div>}
      {children}
    </main>
  </div>;
}
