import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { useAppStore } from '@/stores/app-store';
import { formatNumber, formatUptime, actionColor, truncateMiddle } from '@/lib/utils';
import { Server, Network, ShieldCheck, ShieldX, Eye, Activity } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { ResponsiveDataView } from '@/components/ui/responsive-data-view';

function StatCard({ icon: Icon, label, value, color }: { icon: any; label: string; value: string | number; color?: string }) {
  return (
    <div className="bg-card border border-border rounded-xl p-3 md:p-4">
      <div className="flex items-center gap-2 text-muted-foreground mb-1 md:mb-2">
        <Icon className={`h-4 w-4 ${color || ''}`} />
        <span className="text-xs">{label}</span>
      </div>
      <div className="text-xl md:text-2xl font-bold">{value}</div>
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

  // Server-side traffic data (last 1 hour)
  const [trafficData, setTrafficData] = useState<Array<{ time: string; allow: number; deny: number }>>([]);
  useEffect(() => {
    const loadTraffic = () =>
      api.getTimeSeries(1).then((d) =>
        setTrafficData((d || []).map((p) => ({ time: p.bucket.slice(11, 16), allow: p.allow, deny: p.deny })))
      ).catch(console.error);
    loadTraffic();
    const interval = setInterval(loadTraffic, 30000);
    return () => clearInterval(interval);
  }, []);

  const recent = recentConnections.slice(0, 20);

  return (
    <div className="space-y-4 md:space-y-6">
      <h1 className="text-xl font-bold">Dashboard</h1>

      {/* Status cards */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 md:gap-4">
        <StatCard icon={Server} label="Nodes Online" value={generalStats?.nodes_online ?? nodesOnline.size} color="text-success" />
        <StatCard icon={Network} label="Connections" value={formatNumber(generalStats?.connections ?? totalConns)} />
        <StatCard icon={ShieldCheck} label="Accepted" value={formatNumber(generalStats?.accepted ?? totalAccepted)} color="text-success" />
        <StatCard icon={ShieldX} label="Dropped" value={formatNumber(generalStats?.dropped ?? totalDropped)} color="text-destructive" />
        <StatCard icon={Eye} label="Rules" value={formatNumber(generalStats?.rules ?? totalRules)} color="text-primary" />
        <StatCard icon={Activity} label="WS Clients" value={generalStats?.ws_clients ?? 0} color="text-accent" />
      </div>

      {/* Traffic chart — shorter on mobile */}
      {trafficData.length > 0 && (
        <div className="bg-card border border-border rounded-xl p-4">
          <h2 className="text-sm font-medium mb-4">Traffic</h2>
          <div className="h-[180px] md:h-[250px]">
            <ResponsiveContainer width="100%" height="100%">
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
        </div>
      )}

      {/* Recent connections */}
      <div>
        <h2 className="text-sm font-medium mb-3">Recent Connections</h2>
        <ResponsiveDataView
          data={recent}
          columns={7}
          emptyMessage="Waiting for connections..."
          tableHead={
            <tr className="border-b border-border text-left text-xs text-muted-foreground">
              <th className="px-4 py-2">Time</th>
              <th className="px-4 py-2">Action</th>
              <th className="px-4 py-2">Process</th>
              <th className="px-4 py-2">Destination</th>
              <th className="px-4 py-2">Port</th>
              <th className="px-4 py-2">Protocol</th>
              <th className="px-4 py-2">Rule</th>
            </tr>
          }
          renderRow={(conn: any, i: number) => (
            <tr key={i} className="border-b border-border/50 hover:bg-muted/50">
              <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{conn.time}</td>
              <td className={`px-4 py-2 font-medium ${actionColor(conn.action)}`}>{conn.action}</td>
              <td className="px-4 py-2 font-mono text-xs max-w-48 truncate">{conn.process}</td>
              <td className="px-4 py-2 text-xs">{conn.dst_host || conn.dst_ip}</td>
              <td className="px-4 py-2 text-xs">{conn.dst_port}</td>
              <td className="px-4 py-2 text-xs uppercase">{conn.protocol}</td>
              <td className="px-4 py-2 text-xs text-muted-foreground">{conn.rule}</td>
            </tr>
          )}
          renderCard={(conn: any, i: number) => (
            <div key={i} className="bg-card border border-border rounded-xl p-3 space-y-1.5">
              <div className="flex items-center justify-between">
                <span className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
                  conn.action === 'allow' ? 'bg-success/15 text-success'
                    : conn.action === 'deny' ? 'bg-destructive/15 text-destructive'
                    : 'bg-warning/15 text-warning'
                }`}>
                  {conn.action}
                </span>
                <span className="text-[10px] text-muted-foreground">{conn.time}</span>
              </div>
              <div className="font-mono text-xs break-all">{truncateMiddle(conn.process || '', 50)}</div>
              <div className="text-xs text-muted-foreground">
                → {conn.dst_host || conn.dst_ip}:{conn.dst_port} <span className="uppercase">{conn.protocol}</span>
              </div>
            </div>
          )}
        />
      </div>
    </div>
  );
}
