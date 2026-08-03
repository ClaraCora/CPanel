import { Check, Search, X } from "lucide-react";
import { useMemo, useState } from "react";

export type MultiSelectOption = {
  value: string;
  label: string;
  detail?: string;
  keywords?: string;
};

export function NodeMultiSelect({ value, options, onChange }: { value: string; options: MultiSelectOption[]; onChange: (value: string) => void }) {
  const [query, setQuery] = useState("");
  const selected = useMemo(() => new Set(value.split(",").filter(Boolean)), [value]);
  const visible = useMemo(() => {
    const term = query.trim().toLocaleLowerCase();
    return options
      .filter((option) => !term || `${option.label} ${option.detail ?? ""} ${option.keywords ?? ""}`.toLocaleLowerCase().includes(term))
      .sort((left, right) => Number(selected.has(right.value)) - Number(selected.has(left.value)) || left.label.localeCompare(right.label, "zh-CN"));
  }, [options, query, selected]);

  function commit(next: Set<string>) {
    onChange(options.filter((option) => next.has(option.value)).map((option) => option.value).join(","));
  }

  function toggle(id: string) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    commit(next);
  }

  function selectVisible() {
    const next = new Set(selected);
    visible.forEach((option) => next.add(option.value));
    commit(next);
  }

  return <div className="node-picker">
    <div className="node-picker__toolbar">
      <label className="node-picker__search">
        <Search size={15} aria-hidden="true" />
        <span className="sr-only">筛选节点</span>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="按名称、协议或服务器筛选" />
        {query && <button type="button" aria-label="清除筛选" title="清除筛选" onClick={() => setQuery("")}><X size={14} /></button>}
      </label>
      <span className="node-picker__count">已选择 <strong>{selected.size}</strong> 个</span>
    </div>
    <div className="node-picker__commands">
      <button type="button" disabled={visible.length === 0} onClick={selectVisible}>全选筛选结果</button>
      <button type="button" disabled={selected.size === 0} onClick={() => commit(new Set())}>清空</button>
    </div>
    <div className="node-picker__list" role="group" aria-label="可访问节点">
      {visible.length === 0 ? <p>没有符合条件的节点</p> : visible.map((option) => {
        const checked = selected.has(option.value);
        return <label className={checked ? "is-selected" : ""} key={option.value}>
          <input type="checkbox" checked={checked} onChange={() => toggle(option.value)} />
          <span className="node-picker__check" aria-hidden="true"><Check size={12} /></span>
          <span className="node-picker__copy"><strong>{option.label}</strong>{option.detail && <small>{option.detail}</small>}</span>
        </label>;
      })}
    </div>
  </div>;
}
