import type { ReactNode } from "react";
import { ShieldCheck } from "lucide-react";

export function ResourceListSurface({ enabled = true, children }: { enabled?: boolean; children: ReactNode }) {
  return enabled ? <section className="clear-list">{children}</section> : <>{children}</>;
}

export function ResourceRowActions({ enabled, children }: { enabled: boolean; children: ReactNode }) {
  return enabled ? <div className="clear-row-actions">{children}</div> : <>{children}</>;
}

export function ResourceCategoryFilters({ label, value, options, onChange }: {
  label: string;
  value: string;
  options: { value: string; label: string; count: number }[];
  onChange: (value: string) => void;
}) {
  return <div className="resource-categories" role="group" aria-label={label}>
    {options.map((option) => <button type="button" key={option.value} className={option.value === value ? "is-active" : ""} aria-pressed={option.value === value} onClick={() => onChange(option.value)}>
      {option.label}<span>{option.count}</span>
    </button>)}
  </div>;
}

export function ResourceListFooter({ visible, total }: { visible: number; total: number }) {
  return <footer className="resource-list-footer"><span>显示 {visible} 条，共 {total} 条</span><span>{visible === total ? "已显示全部记录" : "已应用筛选条件"}</span></footer>;
}

export function ResourceListNote({ children }: { children: ReactNode }) {
  return <p className="resource-list-note"><ShieldCheck size={15} aria-hidden="true" /><span>{children}</span></p>;
}

export function ResourceDate({ value, formatted }: { value: string | null | undefined; formatted: string }) {
  const [date, time] = formatted.split(" ");
  return <time className="resource-date" dateTime={value || undefined} title={formatted}><span className="mono">{date}</span>{time && <span className="resource-id mono">{time}</span>}</time>;
}
