import { render } from "@testing-library/react";
import { Route, Switch } from "wouter";
import type { ComponentType } from "react";
import { AdminNavigation } from "../components/AdminNavigation";

export function renderResource(Component: ComponentType, base: string) {
  window.history.replaceState({}, "", base + window.location.search);
  return render(<AdminNavigation><Switch>
    <Route path={base} component={Component} />
    <Route path={`${base}/new`} component={Component} />
    <Route path={`${base}/:id/edit`} component={Component} />
  </Switch></AdminNavigation>);
}
