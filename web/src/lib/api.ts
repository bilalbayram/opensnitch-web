const API_BASE = "/api/v1";

export interface NodeRecord {
  addr: string;
  hostname: string;
  daemon_version: string;
  daemon_uptime: number;
  daemon_rules: number;
  cons: number;
  cons_dropped: number;
  status: string;
  last_connection: string;
  online: boolean;
  mode: string;
  tags: string[];
  template_sync_pending: boolean;
  template_sync_error: string;
}

export interface ConnectionRecord {
  id: number;
  time: string;
  node: string;
  action: string;
  protocol: string;
  src_ip: string;
  src_port: number;
  dst_ip: string;
  dst_host: string;
  dst_port: number;
  uid: number;
  pid: number;
  process: string;
  process_args: string;
  process_cwd: string;
  rule: string;
}

export interface VersionInfo {
  current_version: string;
  build_time?: string;
}

export interface AlertRecord {
  id: number;
  time: string;
  node: string;
  type: number;
  priority: number;
  what: number;
  body: string;
}

export interface DashboardStats {
  nodes_online: number;
  connections: number;
  accepted: number;
  dropped: number;
  rules: number;
  ws_clients: number;
}

export interface RuleOperator {
  type?: string;
  operand?: string;
  data?: string;
  sensitive?: boolean;
  list?: RuleOperator[];
}

export interface RuleRecord {
  id: number;
  time: string;
  node: string;
  name: string;
  display_name: string;
  source_kind: string;
  template_id: number;
  template_name?: string;
  template_rule_id: number;
  enabled: boolean;
  precedence: boolean;
  action: string;
  duration: string;
  operator_type: string;
  operator_sensitive: boolean;
  operator_operand: string;
  operator_data: string;
  operator?: RuleOperator;
  is_compound: boolean;
  description: string;
  nolog: boolean;
  created: string;
}

export interface SeenFlowRecord {
  id: number;
  node: string;
  process: string;
  protocol: string;
  dst_port: number;
  destination_operand: string;
  destination: string;
  action: string;
  first_seen: string;
  last_seen: string;
  count: number;
}

export interface TemplateRuleRecord {
  id: number;
  template_id: number;
  position: number;
  name: string;
  enabled: boolean;
  precedence: boolean;
  action: string;
  duration: string;
  operator_type: string;
  operator_sensitive: boolean;
  operator_operand: string;
  operator_data: string;
  operator?: RuleOperator;
  is_compound: boolean;
  description: string;
  nolog: boolean;
  created_at: string;
  updated_at: string;
}

export interface TemplateAttachmentRecord {
  id: number;
  template_id: number;
  target_type: "node" | "tag";
  target_ref: string;
  priority: number;
  created_at: string;
  updated_at: string;
}

export interface TemplateRecord {
  id: number;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  rules: TemplateRuleRecord[];
  attachments: TemplateAttachmentRecord[];
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem("token");
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { ...headers, ...options?.headers },
  });

  if (res.status === 401) {
    localStorage.removeItem("token");
    window.location.href = "/login";
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }

  const text = await res.text();
  if (!text) return {} as T;
  return JSON.parse(text);
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ token: string; user: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request("/auth/logout", { method: "POST" }),
  me: () => request<{ user: string }>("/auth/me"),

  // Nodes
  getNodes: () => request<NodeRecord[]>("/nodes"),
  getNode: (addr: string) =>
    request<NodeRecord>(`/nodes/${encodeURIComponent(addr)}`),
  updateNodeConfig: (addr: string, config: object) =>
    request(`/nodes/${encodeURIComponent(addr)}/config`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  replaceNodeTags: (addr: string, tags: string[]) =>
    request<{
      tags: string[];
      template_sync_pending: boolean;
      template_sync_error: string;
    }>(`/nodes/${encodeURIComponent(addr)}/tags`, {
      method: "PUT",
      body: JSON.stringify({ tags }),
    }),
  enableInterception: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/interception/enable`, {
      method: "POST",
    }),
  disableInterception: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/interception/disable`, {
      method: "POST",
    }),
  enableFirewall: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/firewall/enable`, {
      method: "POST",
    }),
  disableFirewall: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/firewall/disable`, {
      method: "POST",
    }),

  // Rules
  getRules: (node?: string) =>
    request<RuleRecord[]>(
      `/rules${node ? `?node=${encodeURIComponent(node)}` : ""}`,
    ),
  createRule: (rule: object) =>
    request("/rules", { method: "POST", body: JSON.stringify(rule) }),
  updateRule: (name: string, rule: object) =>
    request(`/rules/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(rule),
    }),
  previewGeneratedRules: (payload: object) =>
    request<{
      data: Array<Record<string, unknown>>;
      skipped_existing: number;
      skipped_excluded: number;
    }>("/rules/generate/preview", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  applyGeneratedRules: (payload: object) =>
    request<{
      status: string;
      mode: string;
      data: Array<Record<string, unknown>>;
      count: number;
    }>("/rules/generate/apply", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  deleteRule: (name: string, node?: string) =>
    request(
      `/rules/${encodeURIComponent(name)}${node ? `?node=${encodeURIComponent(node)}` : ""}`,
      { method: "DELETE" },
    ),
  enableRule: (name: string, node?: string) =>
    request(
      `/rules/${encodeURIComponent(name)}/enable${node ? `?node=${encodeURIComponent(node)}` : ""}`,
      { method: "POST" },
    ),
  disableRule: (name: string, node?: string) =>
    request(
      `/rules/${encodeURIComponent(name)}/disable${node ? `?node=${encodeURIComponent(node)}` : ""}`,
      { method: "POST" },
    ),

  // Connections
  getConnections: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return request<{ data: ConnectionRecord[]; total: number }>(
      `/connections${qs}`,
    );
  },
  purgeConnections: () => request("/connections", { method: "DELETE" }),
  getSeenFlows: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return request<{ data: SeenFlowRecord[]; total: number }>(
      `/seen-flows${qs}`,
    );
  },

  // Stats
  getStats: () => request<DashboardStats>("/stats"),
  getStatsByTable: (table: string, limit?: number) =>
    request<Array<Record<string, unknown>>>(
      `/stats/${table}${limit ? `?limit=${limit}` : ""}`,
    ),
  getTimeSeries: (hours?: number, bucket?: number, node?: string) => {
    const params = new URLSearchParams();
    if (hours) params.set("hours", String(hours));
    if (bucket) params.set("bucket", String(bucket));
    if (node) params.set("node", node);
    const qs = params.toString();
    return request<
      Array<{ bucket: string; allow: number; deny: number; total: number }>
    >(`/stats/timeseries${qs ? "?" + qs : ""}`);
  },
  getTopBlocked: (dimension?: string, limit?: number, hours?: number) => {
    const params = new URLSearchParams();
    if (dimension) params.set("dimension", dimension);
    if (limit) params.set("limit", String(limit));
    if (hours) params.set("hours", String(hours));
    const qs = params.toString();
    return request<Array<{ what: string; hits: number; node: string }>>(
      `/stats/top-blocked${qs ? "?" + qs : ""}`,
    );
  },
  getGeoSummary: (hours?: number, limit?: number) => {
    const params = new URLSearchParams();
    if (hours) params.set("hours", String(hours));
    if (limit) params.set("limit", String(limit));
    const qs = params.toString();
    return request<
      Array<{
        ip: string;
        country: string;
        country_code: string;
        city: string;
        lat: number;
        lon: number;
        hits: number;
      }>
    >(`/stats/geo${qs ? "?" + qs : ""}`);
  },

  // Firewall
  getFirewall: () => request<any[]>("/firewall"),
  reloadFirewall: (node?: string) =>
    request(
      `/firewall/reload${node ? `?node=${encodeURIComponent(node)}` : ""}`,
      { method: "POST" },
    ),

  // Alerts
  getAlerts: (limit?: number, offset?: number) =>
    request<{ data: AlertRecord[]; total: number }>(
      `/alerts?limit=${limit || 50}&offset=${offset || 0}`,
    ),
  deleteAlert: (id: number) => request(`/alerts/${id}`, { method: "DELETE" }),

  // Prompts
  getPendingPrompts: () =>
    request<Array<Record<string, unknown>>>("/prompts/pending"),
  replyPrompt: (id: string, reply: object) =>
    request(`/prompts/${id}/reply`, {
      method: "POST",
      body: JSON.stringify(reply),
    }),

  // Node mode
  setNodeMode: (addr: string, mode: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/mode`, {
      method: "PUT",
      body: JSON.stringify({ mode }),
    }),

  // Process Trust
  getProcessTrust: (addr: string) =>
    request<Array<Record<string, unknown>>>(
      `/nodes/${encodeURIComponent(addr)}/trust`,
    ),
  addProcessTrust: (addr: string, processPath: string, trustLevel: string) =>
    request<Record<string, unknown>>(
      `/nodes/${encodeURIComponent(addr)}/trust`,
      {
        method: "POST",
        body: JSON.stringify({
          process_path: processPath,
          trust_level: trustLevel,
        }),
      },
    ),
  updateProcessTrust: (addr: string, id: number, trustLevel: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/trust/${id}`, {
      method: "PUT",
      body: JSON.stringify({ trust_level: trustLevel }),
    }),
  deleteProcessTrust: (addr: string, id: number) =>
    request(`/nodes/${encodeURIComponent(addr)}/trust/${id}`, {
      method: "DELETE",
    }),

  // DNS
  getDNSDomains: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return request<{ data: any[]; total: number }>(`/dns/domains${qs}`);
  },
  purgeDNSDomains: () => request("/dns/domains", { method: "DELETE" }),
  getDNSServers: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return request<{ data: any[]; total: number }>(`/dns/servers${qs}`);
  },
  createDNSServerRules: (payload: {
    node: string;
    allowed_ips: string[];
    description?: string;
  }) =>
    request<{ status: string; data: any[]; count: number }>(
      "/dns/server-rules",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
    ),

  // Blocklists
  getBlocklists: () => request<any[]>("/blocklists"),
  createBlocklist: (name: string, url: string, category: string) =>
    request("/blocklists", {
      method: "POST",
      body: JSON.stringify({ name, url, category }),
    }),
  deleteBlocklist: (id: number) =>
    request(`/blocklists/${id}`, { method: "DELETE" }),
  enableBlocklist: (id: number) =>
    request(`/blocklists/${id}/enable`, { method: "POST" }),
  disableBlocklist: (id: number) =>
    request(`/blocklists/${id}/disable`, { method: "POST" }),
  syncBlocklist: (id: number) =>
    request<{ status: string; domain_count: number }>(
      `/blocklists/${id}/sync`,
      { method: "POST" },
    ),

  // Templates
  getTemplates: () => request<TemplateRecord[]>("/templates"),
  getTemplate: (id: number) => request<TemplateRecord>(`/templates/${id}`),
  createTemplate: (payload: { name: string; description: string }) =>
    request<TemplateRecord>("/templates", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateTemplate: (
    id: number,
    payload: { name: string; description: string },
  ) =>
    request<TemplateRecord>(`/templates/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  deleteTemplate: (id: number) =>
    request<{ status: string }>(`/templates/${id}`, { method: "DELETE" }),
  createTemplateRule: (templateId: number, payload: object) =>
    request<TemplateRuleRecord>(`/templates/${templateId}/rules`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateTemplateRule: (templateId: number, ruleId: number, payload: object) =>
    request<TemplateRuleRecord>(`/templates/${templateId}/rules/${ruleId}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  deleteTemplateRule: (templateId: number, ruleId: number) =>
    request<{ status: string }>(`/templates/${templateId}/rules/${ruleId}`, {
      method: "DELETE",
    }),
  createTemplateAttachment: (
    templateId: number,
    payload: { target_type: string; target_ref: string; priority: number },
  ) =>
    request<TemplateAttachmentRecord>(`/templates/${templateId}/attachments`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateTemplateAttachment: (
    templateId: number,
    attachmentId: number,
    payload: { target_type: string; target_ref: string; priority: number },
  ) =>
    request<TemplateAttachmentRecord>(
      `/templates/${templateId}/attachments/${attachmentId}`,
      { method: "PUT", body: JSON.stringify(payload) },
    ),
  deleteTemplateAttachment: (templateId: number, attachmentId: number) =>
    request<{ status: string }>(
      `/templates/${templateId}/attachments/${attachmentId}`,
      { method: "DELETE" },
    ),

  // Version & Updates
  getVersion: () => request<VersionInfo>("/version"),
};
