import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { useAppStore } from '@/stores/app-store';
import { formatNumber, formatUptime, actionColor } from '@/lib/utils';
import { Server, Network, ShieldCheck, ShieldX, Eye, Activity } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

function StatCard({ icon: Icon, label, value, color }: { icon: any; label: string; value: string | number; color?: string }) {
  return (
    <div className="bg-card border border-border rounded-xl p-4">
      <div className="flex items-center gap-2 text-muted-foreground mb-2">
        <Icon className={`h-4 w-4 ${color || ''}`} />
        <span className="text-xs">{label}</span>
      </div>
      <div className="text-2xl font-bold">{value}</div>
    </div>
  );
}

export default function DashboardPage() {
  const { stats, nodesOnline, recentConnections } = useAppStore();
  const [generalStats, setGeneralStats] = useState<any>(null);

  useEffect(() => {
    api.getStats().then(setGeneralStats).catch(console.error);
    const interval = setInterval(() => {
      api.getStats().then(setGeneralStats).catch(console.error);
    }, 5000);
    return () => clearInterval(interval);
  }, []);

  // Aggregate stats from all nodes
  let totalConns = 0, totalDropped = 0, totalAccepted = 0, totalRules = 0;
  const nodeEntries = Object.values(stats);
  for (const s of nodeEntries) {
    totalConns += s.connections || 0;
    totalDropped += s.dropped || 0;
    totalAccepted += s.accepted || 0;
    totalRules += s.rules || 0;
  }

  // Build traffic data from recent connections
  const trafficBuckets = new Map<string, { time: string; allow: number; deny: number }>();
  for (const conn of recentConnections.slice(0, 100)) {
    const key = conn.time?.substring(0, 16) || 'now';
    if (!trafficBuckets.has(key)) {
      trafficBuckets.set(key, { time: key, allow: 0, deny: 0 });
    }
    const bucket = trafficBuckets.get(key)!;
    if (conn.action === 'allow') bucket.allow++;
    else bucket.deny++;
  }
  const trafficData = Array.from(trafficBuckets.values()).reverse().slice(-20);

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">Dashboard</h1>

      {/* Status cards */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <StatCard icon={Server} label="Nodes Online" value={generalStats?.nodes_online ?? nodesOnline.size} color="text-success" />
        <StatCard icon={Network} label="Connections" value={formatNumber(generalStats?.connections ?? totalConns)} />
        <StatCard icon={ShieldCheck} label="Accepted" value={formatNumber(generalStats?.accepted ?? totalAccepted)} color="text-success" />
        <StatCard icon={ShieldX} label="Dropped" value={formatNumber(generalStats?.dropped ?? totalDropped)} color="text-destructive" />
        <StatCard icon={Eye} label="Rules" value={formatNumber(generalStats?.rules ?? totalRules)} color="text-primary" />
        <StatCard icon={Activity} label="WS Clients" value={generalStats?.ws_clients ?? 0} color="text-accent" />
      </div>

      {/* Traffic chart */}
      {trafficData.length > 0 && (
        <div className="bg-card border border-border rounded-xl p-4">
          <h2 className="text-sm font-medium mb-4">Traffic</h2>
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={trafficData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#a1a1aa' }} />
              <YAxis tick={{ fontSize: 10, fill: '#a1a1aa' }} />
              <Tooltip contentStyle={{ background: '#111118', border: '1px solid #27272a', borderRadius: 8 }} />
              <Area type="monotone" dataKey="allow" stroke="#22c55e" fill="#22c55e" fillOpacity={0.1} />
              <Area type="monotone" dataKey="deny" stroke="#ef4444" fill="#ef4444" fillOpacity={0.1} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Recent connections */}
      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-border">
          <h2 className="text-sm font-medium">Recent Connections</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">Time</th>
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Process</th>
                <th className="px-4 py-2">Destination</th>
                <th className="px-4 py-2">Port</th>
                <th className="px-4 py-2">Protocol</th>
                <th className="px-4 py-2">Rule</th>
              </tr>
            </thead>
            <tbody>
              {recentConnections.slice(0, 20).map((conn, i) => (
                <tr key={i} className="border-b border-border/50 hover:bg-muted/50">
                  <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{conn.time}</td>
                  <td className={`px-4 py-2 font-medium ${actionColor(conn.action)}`}>{conn.action}</td>
                  <td className="px-4 py-2 font-mono text-xs max-w-48 truncate">{conn.process}</td>
                  <td className="px-4 py-2 text-xs">{conn.dst_host || conn.dst_ip}</td>
                  <td className="px-4 py-2 text-xs">{conn.dst_port}</td>
                  <td className="px-4 py-2 text-xs uppercase">{conn.protocol}</td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{conn.rule}</td>
                </tr>
              ))}
              {recentConnections.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                    Waiting for connections...
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
