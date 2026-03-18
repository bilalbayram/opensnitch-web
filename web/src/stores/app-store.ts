import { create } from 'zustand';

export interface StatsData {
  node: string;
  daemon_version: string;
  uptime: number;
  rules: number;
  connections: number;
  dropped: number;
  accepted: number;
  ignored: number;
  dns_responses: number;
  rule_hits: number;
  rule_misses: number;
  by_proto: Record<string, number>;
  by_address: Record<string, number>;
  by_host: Record<string, number>;
  by_port: Record<string, number>;
  by_uid: Record<string, number>;
  by_executable: Record<string, number>;
}

export interface Prompt {
  id: string;
  node_addr: string;
  created_at: string;
  process: string;
  dst_host: string;
  dst_ip: string;
  dst_port: number;
  protocol: string;
  src_ip: string;
  src_port: number;
  uid: number;
  pid: number;
  args: string[];
  cwd: string;
  checksums: Record<string, string>;
}

export interface ConnectionEvent {
  time: string;
  node: string;
  action: string;
  rule: string;
  protocol: string;
  src_ip: string;
  src_port: number;
  dst_ip: string;
  dst_host: string;
  dst_port: number;
  uid: number;
  pid: number;
  process: string;
  process_args: string[];
}

function connectionFingerprint(conn: ConnectionEvent): string {
  return [
    conn.node,
    conn.time,
    conn.action,
    conn.protocol,
    conn.src_ip,
    conn.src_port,
    conn.dst_ip,
    conn.dst_host,
    conn.dst_port,
    conn.process,
    conn.rule,
  ].join("|");
}

function dedupeConnections(connections: ConnectionEvent[]): ConnectionEvent[] {
  const seen = new Set<string>();
  const deduped: ConnectionEvent[] = [];

  for (const conn of connections) {
    const fingerprint = connectionFingerprint(conn);
    if (seen.has(fingerprint)) {
      continue;
    }
    seen.add(fingerprint);
    deduped.push(conn);
    if (deduped.length >= 200) {
      break;
    }
  }

  return deduped;
}

interface AppState {
  user: string | null;
  token: string | null;
  wsConnected: boolean;
  stats: Record<string, StatsData>;
  prompts: Prompt[];
  recentConnections: ConnectionEvent[];
  nodesOnline: Set<string>;

  setUser: (user: string | null, token: string | null) => void;
  setWSConnected: (connected: boolean) => void;
  updateStats: (data: StatsData) => void;
  addPrompt: (prompt: Prompt) => void;
  removePrompt: (id: string) => void;
  setRecentConnections: (connections: ConnectionEvent[]) => void;
  addConnection: (conn: ConnectionEvent) => void;
  addNodeOnline: (addr: string) => void;
  removeNodeOnline: (addr: string) => void;
}

export const useAppStore = create<AppState>((set) => ({
  user: null,
  token: localStorage.getItem('token'),
  wsConnected: false,
  stats: {},
  prompts: [],
  recentConnections: [],
  nodesOnline: new Set(),

  setUser: (user, token) => {
    if (token) {
      localStorage.setItem('token', token);
    } else {
      localStorage.removeItem('token');
    }
    set({ user, token });
  },

  setWSConnected: (connected) => set({ wsConnected: connected }),

  updateStats: (data) =>
    set((state) => ({
      stats: { ...state.stats, [data.node]: data },
    })),

  addPrompt: (prompt) =>
    set((state) => ({
      prompts: [...state.prompts.filter((p) => p.id !== prompt.id), prompt],
    })),

  removePrompt: (id) =>
    set((state) => ({
      prompts: state.prompts.filter((p) => p.id !== id),
    })),

  setRecentConnections: (connections) =>
    set(() => ({
      recentConnections: dedupeConnections(connections),
    })),

  addConnection: (conn) =>
    set((state) => ({
      recentConnections: dedupeConnections([conn, ...state.recentConnections]),
    })),

  addNodeOnline: (addr) =>
    set((state) => {
      const s = new Set(state.nodesOnline);
      s.add(addr);
      return { nodesOnline: s };
    }),

  removeNodeOnline: (addr) =>
    set((state) => {
      const s = new Set(state.nodesOnline);
      s.delete(addr);
      return { nodesOnline: s };
    }),
}));
