import type { ReactNode } from "react";
import type { MultiSelectOption } from "./components/NodeMultiSelect";
import type { Outbound } from "./types";

export type Resource = { id: string; status: string; created_at: string; updated_at: string };
export type Column<T> = { label: string; render: (item: T) => ReactNode; className?: string; width?: string };

export type ClearListOptions<T> = {
  kind: "machines" | "users" | "plans" | "groups" | "routes" | "outbounds";
  searchPlaceholder: string;
  note: string;
  categories?: { value: string; label: string; matches: (item: T) => boolean }[];
  filter?: { label: string; allLabel: string; value: (item: T) => string };
};
export type FieldSpec = {
  key: string;
  label: string;
  type?: "text" | "number" | "textarea" | "select" | "node-picker" | "route-rules" | "outbound-config" | "json" | "email" | "datetime-local" | "password";
  required?: boolean;
  helper?: string;
  options?: (MultiSelectOption & { exclusive?: boolean })[];
	routeOutbounds?: Outbound[];
  placeholder?: string;
  editOnly?: boolean;
  lockedValue?: string;
	disabled?: (item: any | null) => boolean;
};

export type ResourceDefinition<T extends Resource> = {
  title: string;
  description: string;
  endpoint: string;
  createLabel: string;
  columns: Column<T>[];
  fields: FieldSpec[];
  defaults: Record<string, string>;
  transform: (values: Record<string, string>, editing: boolean) => unknown;
  toValues: (item: T) => Record<string, string>;
  enabledStatus?: string;
  disabledStatus?: string;
  installable?: boolean;
  upgradeable?: boolean;
  identityResettable?: boolean;
  subscriptionActions?: boolean;
	portalActions?: boolean;
  quickAccountCreation?: boolean;
  deletable?: boolean;
  identifierAction?: { label: string; value: (item: T) => string };
  protectedItem?: (item: T) => boolean;
  refreshIntervalMs?: number;
  list?: ClearListOptions<T>;
  related?: { loading: boolean; error: string; retry: () => void };
};

