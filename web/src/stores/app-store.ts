import { create } from 'zustand';

interface StatsData {
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

interface Prompt {
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

interface ConnectionEvent {
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

  addConnection: (conn) =>
    set((state) => ({
      recentConnections: [conn, ...state.recentConnections].slice(0, 200),
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
