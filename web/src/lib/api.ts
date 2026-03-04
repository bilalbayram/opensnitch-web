const API_BASE = '/api/v1';

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
};
