import { Link } from "wouter";
import { editorHref } from "../navigation";
import type { ReactNode } from "react";
import type { Column, Resource } from "../resourceDefinitions";
import { resourceName, statusLabel } from "../resourceModel";

export function PlanBoard<T extends Resource>({ items, columns, children }: {
  items: T[]; columns: Column<T>[]; children: ReactNode;
}) {
  const lanes = new Map<string, T[]>();
  for (const item of items) {
    const lane = lanes.get(item.status);
    if (lane) lane.push(item);
    else lanes.set(item.status, [item]);
  }
  return <><div className="plan-board" aria-label="套餐状态看板">
    {Array.from(lanes, ([status, plans]) => <section className="plan-lane" key={status}>
      <h2>{statusLabel(status)} <span>{plans.length}</span></h2>
      <div className="plan-lane__items">{plans.map((plan) => <article className="plan-entry" key={plan.id} data-resource-id={plan.id}>
        <header>
          <div>{columns[0].render(plan)}</div>
          <Link to={editorHref("/plans", plan.id)} aria-label={`编辑 ${resourceName(plan)}`}>编辑</Link>
        </header>
        <dl>{columns.slice(1).map((column) => <div key={column.label}><dt>{column.label}</dt><dd>{column.render(plan)}</dd></div>)}</dl>
      </article>)}</div>
    </section>)}
  </div><details className="plan-table"><summary>完整表格与操作</summary>{children}</details></>;
}
