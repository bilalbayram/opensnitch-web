import { useEffect, useState, useMemo, lazy, Suspense } from 'react';
import { api, type ConnectionRecord, type DashboardStats } from '@/lib/api';
import { useAppStore, type ConnectionEvent } from '@/stores/app-store';
import { formatNumber, cn } from '@/lib/utils';
import { ShieldCheck, ShieldX, ShieldAlert, Server } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer } from 'recharts';
import type { GeoPoint } from '@/components/ui/geo-map';
import { QuickRulePopover } from '@/components/quick-rule-popover';
import { RuleEditorSheet } from '@/components/rule-editor-sheet';
import type { RuleForm } from '@/components/rule-editor-sheet';

const GeoMap = lazy(() => import('@/components/ui/geo-map').then((m) => ({ default: m.GeoMap })));

type StatEntry = { what: string; hits: number; node: string };

// ─── Process List (left panel) ──────────────────────────────────────
function ProcessList({ processes }: { processes: StatEntry[] }) {
  const maxHits = processes[0]?.hits || 1;

  return (
    <div className="hidden lg:flex flex-col w-56 xl:w-64 shrink-0 bg-card border border-border rounded-xl overflow-hidden">
      <div className="px-3 py-2.5 border-b border-border flex items-center justify-between">
        <h2 className="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Processes</h2>
        <span className="text-[10px] text-muted-foreground">{processes.length}</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {processes.map((p, i) => {
          const name = p.what.split('/').pop() || p.what;
          const pct = (p.hits / maxHits) * 100;
          return (
            <div key={i} className="relative flex items-center gap-2 px-3 py-[5px] hover:bg-muted/30 transition-colors">
              <div
                className="absolute inset-y-0 left-0 bg-primary/[0.06]"
                style={{ width: `${pct}%` }}
              />
              <div className="relative flex items-center gap-2 w-full min-w-0">
                <div className="w-1.5 h-1.5 rounded-full bg-primary/50 shrink-0" />
                <span className="text-[11px] font-mono truncate flex-1" title={p.what}>{name}</span>
                <span className="text-[10px] text-muted-foreground tabular-nums shrink-0">{formatNumber(p.hits)}</span>
              </div>
            </div>
          );
        })}
        {processes.length === 0 && (
          <div className="px-3 py-8 text-xs text-muted-foreground text-center">No data</div>
        )}
      </div>
    </div>
  );
}

// ─── Summary Panel (right panel) ────────────────────────────────────
function SummaryPanel({
  stats,
  nodesOnline,
  topProcesses,
  topHosts,
}: {
  stats: DashboardStats | null;
  nodesOnline: number;
  topProcesses: StatEntry[];
  topHosts: StatEntry[];
}) {
  const allowed = stats?.allowed ?? 0;
  const denied = stats?.denied ?? 0;
  const total = stats?.total ?? 0;

  return (
    <div className="hidden lg:flex flex-col w-64 xl:w-72 shrink-0 bg-card border border-border rounded-xl overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-border">
        <h2 className="text-sm font-semibold">Summary</h2>
        <p className="text-[10px] text-muted-foreground mt-0.5">
          {topProcesses.length} processes &middot; {topHosts.length} domains
        </p>
      </div>

      {/* Connection stats */}
      <div className="px-4 py-3 space-y-2.5 border-b border-border">
        <StatRow icon={Server} label="Nodes online" value={nodesOnline} color="text-success" />
        <StatRow icon={ShieldCheck} label="Allowed" value={formatNumber(allowed)} color="text-success" />
        <StatRow icon={ShieldX} label="Denied" value={formatNumber(denied)} color="text-destructive" />
        <StatRow icon={ShieldAlert} label="Total" value={formatNumber(total)} />

        {/* Allow/deny ratio bar */}
        {(allowed + denied) > 0 && (
          <div className="pt-1">
            <div className="flex h-1.5 rounded-full bg-muted overflow-hidden">
              <div className="bg-success h-full" style={{ width: `${(allowed / (allowed + denied)) * 100}%` }} />
              <div className="bg-destructive h-full" style={{ width: `${(denied / (allowed + denied)) * 100}%` }} />
            </div>
          </div>
        )}
      </div>

      {/* Top Processes */}
      <div className="px-4 py-3 border-b border-border overflow-y-auto" style={{ maxHeight: '30%' }}>
        <h3 className="text-[10px] text-muted-foreground uppercase tracking-wider mb-2">Top Processes</h3>
        {topProcesses.slice(0, 6).map((p, i) => (
          <div key={i} className="flex items-center justify-between py-0.5">
            <span className="text-xs font-mono truncate max-w-[140px]" title={p.what}>
              {p.what.split('/').pop() || p.what}
            </span>
            <span className="text-[10px] text-muted-foreground tabular-nums">{formatNumber(p.hits)}</span>
          </div>
        ))}
        {topProcesses.length === 0 && <Empty />}
      </div>

      {/* Top Domains */}
      <div className="px-4 py-3 flex-1 overflow-y-auto">
        <h3 className="text-[10px] text-muted-foreground uppercase tracking-wider mb-2">Top Domains</h3>
        {topHosts.slice(0, 6).map((h, i) => (
          <div key={i} className="flex items-center justify-between py-0.5">
            <span className="text-xs truncate max-w-[140px]" title={h.what}>{h.what}</span>
            <span className="text-[10px] text-muted-foreground tabular-nums">{formatNumber(h.hits)}</span>
          </div>
        ))}
        {topHosts.length === 0 && <Empty />}
      </div>
    </div>
  );
}

function StatRow({ icon: Icon, label, value, color }: { icon: typeof Server; label: string; value: string | number; color?: string }) {
  return (
    <div className="flex items-center gap-2 text-xs">
      <Icon className={cn('h-3.5 w-3.5', color || 'text-muted-foreground')} />
      <span className="text-muted-foreground">{label}</span>
      <span className={cn('ml-auto font-medium tabular-nums', color)}>{value}</span>
    </div>
  );
}

function Empty() {
  return <div className="text-[10px] text-muted-foreground py-1">&mdash;</div>;
}

// ─── Traffic Timeline (compact sparkline) ───────────────────────────
function TrafficTimeline({ data }: { data: Array<{ time: string; allow: number; deny: number }> }) {
  if (data.length === 0) return null;

  return (
    <div className="bg-card border border-border rounded-xl p-3 shrink-0">
      <div className="flex items-center justify-between mb-1">
        <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Traffic (1h)</span>
        <div className="flex items-center gap-3">
          <Legend color="bg-success" label="Allow" />
          <Legend color="bg-destructive" label="Deny" />
        </div>
      </div>
      <div className="h-[64px]">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 2, right: 0, bottom: 0, left: 0 }}>
            <XAxis dataKey="time" hide />
            <YAxis hide />
            <Area type="monotone" dataKey="allow" stroke="#22c55e" fill="#22c55e" fillOpacity={0.08} strokeWidth={1.5} />
            <Area type="monotone" dataKey="deny" stroke="#ef4444" fill="#ef4444" fillOpacity={0.08} strokeWidth={1.5} />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1 text-[10px] text-muted-foreground">
      <span className={cn('w-1.5 h-1.5 rounded-full', color)} />
      {label}
    </span>
  );
}

// ─── Live Connections Feed ──────────────────────────────────────────
function LiveConnections({ connections, onAdvanced }: { connections: ConnectionEvent[]; onAdvanced?: (prefill: Partial<RuleForm>) => void }) {
  return (
    <div className="flex-1 min-h-0 bg-card border border-border rounded-xl overflow-hidden flex flex-col">
      <div className="px-3 py-2 border-b border-border shrink-0 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Live Connections</span>
          <span className="relative flex h-1.5 w-1.5">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-50" />
            <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-success" />
          </span>
        </div>
        <span className="text-[10px] text-muted-foreground">{connections.length}</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {connections.map((conn, i) => (
          <div
            key={`${conn.time}-${conn.dst_ip}-${conn.process}-${i}`}
            className="flex items-center gap-2 px-3 py-[5px] border-b border-border/20 hover:bg-muted/20 transition-colors text-[11px]"
          >
            <span className={cn(
              'w-1.5 h-1.5 rounded-full shrink-0',
              conn.action === 'allow' ? 'bg-success' : conn.action === 'deny' ? 'bg-destructive' : 'bg-warning',
            )} />
            <span className="font-mono truncate w-20 shrink-0 text-foreground/80" title={conn.process}>
              {conn.process?.split('/').pop() || '?'}
            </span>
            <span className="text-muted-foreground/40 shrink-0">&rarr;</span>
            <span className="truncate flex-1" title={conn.dst_host || conn.dst_ip}>
              {conn.dst_host || conn.dst_ip}
            </span>
            <span className="text-[10px] text-muted-foreground shrink-0 tabular-nums">{conn.dst_port}</span>
            <span className="text-[10px] text-muted-foreground/40 uppercase shrink-0 w-7 text-right">{conn.protocol}</span>
            <QuickRulePopover connection={conn} onAdvanced={onAdvanced} compact />
          </div>
        ))}
        {connections.length === 0 && (
          <div className="flex items-center justify-center h-full text-xs text-muted-foreground">
            Waiting for connections&hellip;
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Mobile Summary (compact) ───────────────────────────────────────
function MobileSummary({ stats }: { stats: DashboardStats | null }) {
  return (
    <div className="lg:hidden grid grid-cols-3 gap-2">
      <MiniStat label="Connections" value={formatNumber(stats?.total ?? 0)} />
      <MiniStat label="Allowed" value={formatNumber(stats?.allowed ?? 0)} color="text-success" />
      <MiniStat label="Denied" value={formatNumber(stats?.denied ?? 0)} color="text-destructive" />
    </div>
  );
}

function MiniStat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div className="bg-card border border-border rounded-lg p-2 text-center">
      <div className={cn('text-lg font-bold tabular-nums', color)}>{value}</div>
      <div className="text-[10px] text-muted-foreground">{label}</div>
    </div>
  );
}

// ─── Main Dashboard ─────────────────────────────────────────────────
function mapConnectionRecord(conn: ConnectionRecord): ConnectionEvent {
  return {
    time: conn.time,
    node: conn.node,
    action: conn.action,
    rule: conn.rule,
    protocol: conn.protocol,
    src_ip: conn.src_ip,
    src_port: conn.src_port,
    dst_ip: conn.dst_ip,
    dst_host: conn.dst_host,
    dst_port: conn.dst_port,
    uid: conn.uid,
    pid: conn.pid,
    process: conn.process,
    process_args: conn.process_args ? conn.process_args.split(' ') : [],
  };
}

export default function DashboardPage() {
  const { stats: nodeStats, nodesOnline, recentConnections, setRecentConnections } = useAppStore();
  const [generalStats, setGeneralStats] = useState<DashboardStats | null>(null);
  const [geoData, setGeoData] = useState<GeoPoint[]>([]);
  const [topProcesses, setTopProcesses] = useState<StatEntry[]>([]);
  const [topHosts, setTopHosts] = useState<StatEntry[]>([]);
  const [trafficData, setTrafficData] = useState<Array<{ time: string; allow: number; deny: number }>>([]);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorPrefill, setEditorPrefill] = useState<Partial<RuleForm> | undefined>();

  const handleAdvanced = (prefill: Partial<RuleForm>) => {
    setEditorPrefill(prefill);
    setEditorOpen(true);
  };

  const handleEditorSave = async (form: RuleForm) => {
    await api.createRule(form);
    setEditorOpen(false);
    setEditorPrefill(undefined);
  };

  // Fetch dashboard stats
  useEffect(() => {
    const load = () => api.getStats().then(setGeneralStats).catch(console.error);
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, []);

  // Seed recent connections from API on first load
  useEffect(() => {
    if (recentConnections.length > 0) return;
    api.getConnections({ limit: '50', offset: '0' })
      .then((res) => {
        if (useAppStore.getState().recentConnections.length === 0) {
          setRecentConnections((res.data || []).map(mapConnectionRecord));
        }
      })
      .catch(console.error);
  }, [recentConnections.length, setRecentConnections]);

  // Geo data for map
  useEffect(() => {
    const load = () => api.getGeoSummary(24, 100).then((d) => setGeoData(d || [])).catch(console.error);
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, []);

  // Top processes + hosts for sidebar and summary
  useEffect(() => {
    const load = () => {
      api.getStatsByTable('processes', 50).then((d) => setTopProcesses((d || []) as StatEntry[])).catch(console.error);
      api.getStatsByTable('hosts', 50).then((d) => setTopHosts((d || []) as StatEntry[])).catch(console.error);
    };
    load();
    const id = setInterval(load, 10000);
    return () => clearInterval(id);
  }, []);

  // Traffic timeline
  useEffect(() => {
    const load = () =>
      api.getTimeSeries(1).then((d) =>
        setTrafficData((d || []).map((p) => ({ time: p.bucket.slice(11, 16), allow: p.allow, deny: p.deny }))),
      ).catch(console.error);
    load();
    const id = setInterval(load, 30000);
    return () => clearInterval(id);
  }, []);

  // Merge API process data with live WebSocket stats
  const processesForSidebar = useMemo(() => {
    if (topProcesses.length > 0) return topProcesses;
    const merged: Record<string, number> = {};
    for (const s of Object.values(nodeStats)) {
      for (const [exe, count] of Object.entries(s.by_executable || {})) {
        merged[exe] = (merged[exe] || 0) + count;
      }
    }
    return Object.entries(merged)
      .map(([what, hits]) => ({ what, hits, node: '' }))
      .sort((a, b) => b.hits - a.hits);
  }, [topProcesses, nodeStats]);

  const recent = recentConnections.slice(0, 50);

  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:h-[calc(100vh-6rem)] lg:overflow-hidden">
      {/* Mobile: compact stats */}
      <MobileSummary stats={generalStats} />

      {/* Left: Process List */}
      <ProcessList processes={processesForSidebar} />

      {/* Center: Map + Timeline + Live Feed */}
      <div className="flex-1 flex flex-col gap-3 min-w-0 lg:min-h-0 lg:overflow-hidden">
        <Suspense fallback={<div className="shrink-0 rounded-xl border border-border bg-[#080a12]" style={{ height: '40vh', minHeight: 200 }} />}>
          <GeoMap geoData={geoData} className="shrink-0" />
        </Suspense>
        <TrafficTimeline data={trafficData} />
        <LiveConnections connections={recent} onAdvanced={handleAdvanced} />
      </div>

      {/* Right: Summary Panel */}
      <SummaryPanel
        stats={generalStats}
        nodesOnline={generalStats?.nodes_online ?? nodesOnline.size}
        topProcesses={processesForSidebar}
        topHosts={topHosts}
      />

      {/* Advanced Rule Editor */}
      <RuleEditorSheet
        open={editorOpen}
        onClose={() => { setEditorOpen(false); setEditorPrefill(undefined); }}
        initialValues={editorPrefill}
        onSave={handleEditorSave}
        title="Create Rule from Connection"
      />
    </div>
  );
}
