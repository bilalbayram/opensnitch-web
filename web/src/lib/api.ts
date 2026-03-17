const API_BASE = '/api/v1';

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

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('token');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { ...headers, ...options?.headers },
  });

  if (res.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    throw new Error('Unauthorized');
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
    request<{ token: string; user: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request('/auth/logout', { method: 'POST' }),
  me: () => request<{ user: string }>('/auth/me'),

  // Nodes
  getNodes: () => request<any[]>('/nodes'),
  getNode: (addr: string) => request<any>(`/nodes/${encodeURIComponent(addr)}`),
  updateNodeConfig: (addr: string, config: any) =>
    request(`/nodes/${encodeURIComponent(addr)}/config`, {
      method: 'PUT',
      body: JSON.stringify(config),
    }),
  enableInterception: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/interception/enable`, { method: 'POST' }),
  disableInterception: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/interception/disable`, { method: 'POST' }),
  enableFirewall: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/firewall/enable`, { method: 'POST' }),
  disableFirewall: (addr: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/firewall/disable`, { method: 'POST' }),

  // Rules
  getRules: (node?: string) =>
    request<any[]>(`/rules${node ? `?node=${encodeURIComponent(node)}` : ''}`),
  createRule: (rule: any) =>
    request('/rules', { method: 'POST', body: JSON.stringify(rule) }),
  updateRule: (name: string, rule: any) =>
    request(`/rules/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(rule) }),
  previewGeneratedRules: (payload: object) =>
    request<{ data: Array<Record<string, unknown>>; skipped_existing: number; skipped_excluded: number }>('/rules/generate/preview', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  applyGeneratedRules: (payload: object) =>
    request<{ status: string; mode: string; data: Array<Record<string, unknown>>; count: number }>('/rules/generate/apply', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  deleteRule: (name: string, node?: string) =>
    request(`/rules/${encodeURIComponent(name)}${node ? `?node=${encodeURIComponent(node)}` : ''}`, { method: 'DELETE' }),
  enableRule: (name: string, node?: string) =>
    request(`/rules/${encodeURIComponent(name)}/enable${node ? `?node=${encodeURIComponent(node)}` : ''}`, { method: 'POST' }),
  disableRule: (name: string, node?: string) =>
    request(`/rules/${encodeURIComponent(name)}/disable${node ? `?node=${encodeURIComponent(node)}` : ''}`, { method: 'POST' }),

  // Connections
  getConnections: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return request<{ data: any[]; total: number }>(`/connections${qs}`);
  },
  purgeConnections: () => request('/connections', { method: 'DELETE' }),
  getSeenFlows: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return request<{ data: SeenFlowRecord[]; total: number }>(`/seen-flows${qs}`);
  },

  // Stats
  getStats: () => request<any>('/stats'),
  getStatsByTable: (table: string, limit?: number) =>
    request<any[]>(`/stats/${table}${limit ? `?limit=${limit}` : ''}`),

  // Firewall
  getFirewall: () => request<any[]>('/firewall'),
  reloadFirewall: (node?: string) =>
    request(`/firewall/reload${node ? `?node=${encodeURIComponent(node)}` : ''}`, { method: 'POST' }),

  // Alerts
  getAlerts: (limit?: number, offset?: number) =>
    request<{ data: any[]; total: number }>(`/alerts?limit=${limit || 50}&offset=${offset || 0}`),
  deleteAlert: (id: number) =>
    request(`/alerts/${id}`, { method: 'DELETE' }),

  // Prompts
  getPendingPrompts: () => request<any[]>('/prompts/pending'),
  replyPrompt: (id: string, reply: any) =>
    request(`/prompts/${id}/reply`, { method: 'POST', body: JSON.stringify(reply) }),

  // Node mode
  setNodeMode: (addr: string, mode: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/mode`, {
      method: 'PUT',
      body: JSON.stringify({ mode }),
    }),

  // Process Trust
  getProcessTrust: (addr: string) =>
    request<any[]>(`/nodes/${encodeURIComponent(addr)}/trust`),
  addProcessTrust: (addr: string, processPath: string, trustLevel: string) =>
    request<any>(`/nodes/${encodeURIComponent(addr)}/trust`, {
      method: 'POST',
      body: JSON.stringify({ process_path: processPath, trust_level: trustLevel }),
    }),
  updateProcessTrust: (addr: string, id: number, trustLevel: string) =>
    request(`/nodes/${encodeURIComponent(addr)}/trust/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ trust_level: trustLevel }),
    }),
  deleteProcessTrust: (addr: string, id: number) =>
    request(`/nodes/${encodeURIComponent(addr)}/trust/${id}`, { method: 'DELETE' }),

  // Blocklists
  getBlocklists: () => request<any[]>('/blocklists'),
  createBlocklist: (name: string, url: string, category: string) =>
    request('/blocklists', { method: 'POST', body: JSON.stringify({ name, url, category }) }),
  deleteBlocklist: (id: number) =>
    request(`/blocklists/${id}`, { method: 'DELETE' }),
  enableBlocklist: (id: number) =>
    request(`/blocklists/${id}/enable`, { method: 'POST' }),
  disableBlocklist: (id: number) =>
    request(`/blocklists/${id}/disable`, { method: 'POST' }),
  syncBlocklist: (id: number) =>
    request<{ status: string; domain_count: number }>(`/blocklists/${id}/sync`, { method: 'POST' }),

  // Version & Updates
  getVersion: () =>
    request<{
      current_version: string;
      build_time?: string;
      latest_version?: string;
      update_available: boolean;
      last_check?: string;
      checking: boolean;
      downloading: boolean;
      error?: string;
      release?: {
        tag_name: string;
        published_at: string;
        html_url: string;
        body: string;
      };
    }>('/version'),
  checkUpdate: () =>
    request<{
      current_version: string;
      build_time?: string;
      latest_version?: string;
      update_available: boolean;
      last_check?: string;
      checking: boolean;
      downloading: boolean;
      error?: string;
      release?: {
        tag_name: string;
        published_at: string;
        html_url: string;
        body: string;
      };
    }>('/update/check', { method: 'POST' }),
  applyUpdate: () =>
    request<{ status: string }>('/update/apply', { method: 'POST' }),
};
