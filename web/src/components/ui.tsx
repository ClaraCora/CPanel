import { createContext, useCallback, useContext, useEffect, useId, useLayoutEffect, useRef, useState, type ButtonHTMLAttributes, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { AlertCircle, CheckCircle2, Info, LoaderCircle, X } from "lucide-react";
import type { StatusTone } from "../types";

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";

export function Button({ variant = "secondary", loading, children, className = "", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; loading?: boolean }) {
  return <button className={`button button--${variant} ${className}`} disabled={loading || props.disabled} {...props}>{loading && <LoaderCircle className="spin" size={16} aria-hidden="true" />}{children}</button>;
}

const statusMap: Record<string, { label: string; tone: StatusTone }> = {
  active: { label: "启用", tone: "success" }, online: { label: "在线", tone: "success" }, published: { label: "已发布", tone: "success" },
  pending: { label: "待接入", tone: "warning" }, draft: { label: "待发布", tone: "warning" }, disabled: { label: "已停用", tone: "neutral" },
  paused: { label: "已暂停", tone: "warning" }, expired: { label: "已到期", tone: "danger" },
  offline: { label: "离线", tone: "danger" }, error: { label: "异常", tone: "danger" }, archived: { label: "已归档", tone: "neutral" },
};

export function StatusBadge({ status }: { status: string }) {
  const item = statusMap[status] ?? { label: status || "未知", tone: "neutral" as StatusTone };
  return <span className={`status status--${item.tone}`}><span className="status__dot" aria-hidden="true" />{item.label}</span>;
}

export function PageHeader({ title, description, actions, children }: { title: string; description?: string; actions?: ReactNode; children?: ReactNode }) {
  return <><header className="page-header"><div><h1 tabIndex={-1}>{title}</h1>{description && <p>{description}</p>}</div>{actions && <div className="page-header__actions">{actions}</div>}</header>{children}</>;
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-state__icon"><Info size={20} /></div><strong>{title}</strong><p>{description}</p>{action}</div>;
}

export function TableSkeleton({ columns = 6, rows = 6 }: { columns?: number; rows?: number }) {
  return <div className="table-skeleton" aria-label="正在加载"><div className="skeleton-row" style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }}>{Array.from({ length: columns }).map((_, i) => <i key={i} />)}</div>{Array.from({ length: rows }).map((_, row) => <div className="skeleton-row" style={{ gridTemplateColumns: `repeat(${columns}, 1fr)` }} key={row}>{Array.from({ length: columns }).map((_, col) => <i key={col} />)}</div>)}</div>;
}

export function Drawer({ open, wide = false, title, description, children, onClose }: { open: boolean; wide?: boolean; title: string; description?: string; children: ReactNode; onClose: () => void }) {
  const titleID = useId();
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    document.addEventListener("keydown", onKey);
    document.body.style.overflow = "hidden";
    return () => { document.removeEventListener("keydown", onKey); document.body.style.overflow = ""; };
  }, [open, onClose]);
  if (!open) return null;
  return createPortal(<div className="drawer-layer"><button className="drawer-backdrop" aria-label="关闭" onClick={onClose} /><aside className={`drawer ${wide ? "drawer--wide" : ""}`} role="dialog" aria-modal="true" aria-labelledby={titleID}><header className="drawer__header"><div><h2 id={titleID}>{title}</h2>{description && <p>{description}</p>}</div><button className="icon-button" aria-label="关闭" title="关闭" onClick={onClose}><X size={19} /></button></header><div className="drawer__body">{children}</div></aside></div>, document.body);
}

export function RowMenu({ label, children }: { label: string; children: ReactNode }) {
  const menuID = useId();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);
  const close = useCallback(() => setOpen(false), []);

  const place = useCallback(() => {
    if (!anchorRef.current || !panelRef.current) return;
    const anchor = anchorRef.current.getBoundingClientRect();
    const panel = panelRef.current.getBoundingClientRect();
    const left = Math.max(8, Math.min(anchor.right - panel.width, window.innerWidth - panel.width - 8));
    const below = anchor.bottom + 4;
    const top = below + panel.height <= window.innerHeight - 8 ? below : Math.max(8, anchor.top - panel.height - 4);
    setPosition({ left, top });
  }, []);

  useLayoutEffect(() => { if (open) place(); }, [open, place]);
  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!anchorRef.current?.contains(target) && !panelRef.current?.contains(target)) close();
    };
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape") { close(); anchorRef.current?.focus(); } };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
    };
  }, [close, open]);

  return <>
    <button ref={anchorRef} type="button" className="icon-button" aria-label={label} title="更多操作" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuID : undefined} onClick={() => { setPosition(null); setOpen((value) => !value); }}>
      <MoreHorizontalIcon />
    </button>
    {open && createPortal(<div ref={panelRef} id={menuID} role="menu" className="row-menu__panel row-menu__panel--portal" style={{ left: position?.left ?? 0, top: position?.top ?? 0, visibility: position ? "visible" : "hidden" }} onClick={(event) => { if ((event.target as Element).closest("button, a")) close(); }}>{children}</div>, document.body)}
  </>;
}

function MoreHorizontalIcon() {
  return <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true"><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /><circle cx="5" cy="12" r="1" /></svg>;
}

export function ConfirmDialog({ open, title, description, confirmLabel = "确认", confirmVariant = "danger", loading = false, onConfirm, onClose }: { open: boolean; title: string; description: string; confirmLabel?: string; confirmVariant?: ButtonVariant; loading?: boolean; onConfirm: () => void; onClose: () => void }) {
  const titleID = useId();
  const descriptionID = useId();
  const cancelRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    cancelRef.current?.focus();
    const onKey = (event: KeyboardEvent) => { if (event.key === "Escape" && !loading) onClose(); };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [loading, onClose, open]);
  if (!open) return null;
  return createPortal(<div className="confirm-layer"><button type="button" className="confirm-backdrop" aria-label="取消并关闭" onClick={() => { if (!loading) onClose(); }} /><section className="confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby={titleID} aria-describedby={descriptionID}><h2 id={titleID}>{title}</h2><p id={descriptionID}>{description}</p><footer><button ref={cancelRef} type="button" className="button button--secondary" onClick={onClose} disabled={loading}>取消</button><Button type="button" variant={confirmVariant} loading={loading} onClick={onConfirm}>{confirmLabel}</Button></footer></section></div>, document.body);
}

export function Field({ label, required, error, helper, group = false, children }: { label: string; required?: boolean; error?: string; helper?: string; group?: boolean; children: ReactNode }) {
  const content = <><span className="field__label">{label}{required && <b aria-hidden="true"> *</b>}</span>{children}{error ? <span className="field__error" role="alert">{error}</span> : helper ? <span className="field__helper">{helper}</span> : null}</>;
  return group ? <div className={`field ${error ? "field--error" : ""}`}>{content}</div> : <label className={`field ${error ? "field--error" : ""}`}>{content}</label>;
}

type ToastItem = { id: number; tone: "success" | "error"; message: string };
const ToastContext = createContext<(message: string, tone?: "success" | "error") => void>(() => undefined);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const show = useCallback((message: string, tone: "success" | "error" = "success") => {
    const id = Date.now();
    setItems((value) => [...value, { id, tone, message }]);
    window.setTimeout(() => setItems((value) => value.filter((item) => item.id !== id)), 4000);
  }, []);
  return <ToastContext.Provider value={show}>{children}{createPortal(<div className="toast-region" aria-live="polite">{items.map((item) => <div className={`toast toast--${item.tone}`} key={item.id}>{item.tone === "success" ? <CheckCircle2 size={17} /> : <AlertCircle size={17} />}<span>{item.message}</span><button aria-label="关闭通知" onClick={() => setItems((value) => value.filter((entry) => entry.id !== item.id))}><X size={15} /></button></div>)}</div>, document.body)}</ToastContext.Provider>;
}

export const useToast = () => useContext(ToastContext);

export function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${(value / 1024 ** index).toLocaleString("zh-CN", { maximumFractionDigits: index > 2 ? 2 : 1 })} ${units[index]}`;
}

export function formatDate(value: string | null | undefined) {
  return formatDateValue(value, { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
}

export function formatPreciseDate(value: string | null | undefined) {
  return formatDateValue(value, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" });
}

export function formatDateWithYear(value: string | null | undefined) {
  return formatDateValue(value, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
}

function formatDateValue(value: string | null | undefined, options: Intl.DateTimeFormatOptions) {
  if (!value) return "从未";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "时间无效";
  return new Intl.DateTimeFormat("zh-CN", options).format(date);
}
