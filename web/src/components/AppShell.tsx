import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
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

function matchesRoute(location: string, to: string) {
  return location === to || (to !== "/" && location.startsWith(to + "/"));
}

function NavigationLinks({ location, expanded, toggle, onNavigate }: {
  location: string; expanded: string[]; toggle: (label: string) => void; onNavigate?: () => void;
}) {
  const id = useId();
  function link(item: typeof primary[number], nested: boolean) {
    const current = matchesRoute(location, item.to);
    return <Link key={item.to} to={item.to} className={`admin-nav-item${nested ? " admin-nav-child" : ""}${current ? " is-active" : ""}`}
      aria-current={current ? "page" : undefined} onClick={onNavigate}><item.icon className="nav-icon" size={nested ? 15 : 17} aria-hidden="true" />{item.label}</Link>;
  }

  function groupKey(event: KeyboardEvent<HTMLDivElement>, label: string) {
    if (event.altKey || event.metaKey || event.ctrlKey || event.shiftKey || !expanded.includes(label)) return;
    if (event.key === "Escape" && !onNavigate) {
      event.preventDefault();
      event.currentTarget.querySelector("button")?.focus();
      toggle(label);
      return;
    }
    const links = Array.from(event.currentTarget.querySelectorAll("a"));
    const index = links.indexOf(document.activeElement as HTMLAnchorElement);
    const last = links.length - 1;
    let target: number;
    switch (event.key) {
      case "Home":
        target = 0;
        break;
      case "End":
        target = last;
        break;
      case "ArrowDown":
        target = index === last ? 0 : index + 1;
        break;
      case "ArrowUp":
        target = index <= 0 ? last : index - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    links[target]?.focus();
  }

  return <>
    {primary.map((item) => link(item, false))}
    {groups.map((group, index) => {
      const open = expanded.includes(group.label);
      const current = group.items.some((item) => matchesRoute(location, item.to));
      const controls = `${id}-group-${index}`;
      return <div className="admin-nav-group" key={group.label} onKeyDown={(event) => groupKey(event, group.label)}>
        <button type="button" className={`admin-nav-item admin-nav-group__toggle${current ? " has-current" : ""}`}
          aria-expanded={open} aria-controls={controls} onClick={() => toggle(group.label)}>
          <group.icon className="nav-icon" size={17} aria-hidden="true" />{group.label}<ChevronDown size={14} aria-hidden="true" />
        </button>
        <div className="admin-nav-children" id={controls} hidden={!open}>{open && group.items.map((item) => link(item, true))}</div>
      </div>;
    })}
  </>;
}

export function AppShell({ admin, demo, onLogout, children }: { admin: Admin; demo: boolean; onLogout: () => void; children: ReactNode }) {
  const [location] = useLocation();
  const currentGroup = groups.find((group) => group.items.some((item) => matchesRoute(location, item.to)))?.label;
  const [expanded, setExpanded] = useState<string[]>(() => currentGroup ? [currentGroup] : []);
  const [mobileOpen, setMobileOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const desktopNav = useRef<HTMLElement>(null);
  const request = useGuardedAction();
  useEffect(() => {
    setMobileOpen(false);
    if (currentGroup) setExpanded((previous) => previous.includes(currentGroup) ? previous : [...previous, currentGroup]);
  }, [location, currentGroup]);
  useEffect(() => {
    let desktopFocused = desktopNav.current?.contains(document.activeElement);
    function trackFocus(event: FocusEvent) {
      desktopFocused = desktopNav.current?.contains(event.target as Node);
    }
    function resize() {
      if (window.innerWidth < 1050) {
        if (desktopFocused) menuButton.current?.focus({ preventScroll: true });
      } else if (mobileOpen) {
        setMobileOpen(false);
        const nav = desktopNav.current;
        const target = nav?.querySelector<HTMLElement>('[aria-current="page"]') ?? nav?.querySelector<HTMLElement>(".has-current") ?? nav?.querySelector<HTMLElement>("a");
        target?.focus();
      }
    }
    document.addEventListener("focusin", trackFocus);
    window.addEventListener("resize", resize);
    return () => {
      document.removeEventListener("focusin", trackFocus);
      window.removeEventListener("resize", resize);
    };
  }, [mobileOpen]);

  function toggle(label: string) {
    setExpanded((previous) => previous.includes(label) ? previous.filter((item) => item !== label) : [...previous, label]);
  }

  return <div className="admin-shell">
    <a className="skip-link" href="#main-content">跳到主要内容</a>
    <header className="admin-header">
      <div className="admin-header__inner">
        <button ref={menuButton} type="button" className="icon-button admin-menu-toggle" aria-label="打开导航" aria-haspopup="dialog" aria-expanded={mobileOpen} aria-controls={mobileOpen ? "admin-mobile-navigation" : undefined} onClick={() => { menuButton.current?.focus({ preventScroll: true }); setMobileOpen(true); }}><Menu size={20} aria-hidden="true" /></button>
        <Link to="/" className="admin-brand" aria-label="CPanel 总览"><span className="admin-brand__mark">C</span><strong>CPanel</strong></Link>
        <nav ref={desktopNav} className="admin-desktop-navigation" aria-label="主导航">
          <span className="workspace-nav-label" aria-hidden="true">工作区</span>
          <NavigationLinks location={location} expanded={expanded} toggle={toggle} />
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
        <NavigationLinks location={location} expanded={expanded} toggle={toggle} onNavigate={() => setMobileOpen(false)} />
      </nav>
    </DetailDialog>
    <main className="admin-main" id="main-content">
      {demo && <div className="admin-demo-banner"><Network size={15} aria-hidden="true" /><span>当前使用演示数据，操作不会写入数据库。</span></div>}
      {children}
    </main>
  </div>;
}
