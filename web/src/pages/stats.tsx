import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api, type TopNRecord } from '@/lib/api';
import { formatNumber } from '@/lib/utils';
import {
  BarChart, Bar, AreaChart, Area,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import { useIsMobile } from '@/hooks/use-media-query';

const tabs = [
  { key: 'timeline', label: 'Timeline' },
  { key: 'blocked', label: 'Blocked' },
  { key: 'geo', label: 'GeoIP' },
  { key: 'hosts', label: 'Hosts' },
  { key: 'processes', label: 'Processes' },
  { key: 'addresses', label: 'Addresses' },
  { key: 'ports', label: 'Ports' },
  { key: 'users', label: 'Users' },
];

const tableTitles: Record<string, string> = {
  timeline: 'Connection Timeline',
  blocked: 'Top Blocked',
  geo: 'GeoIP Destinations',
  hosts: 'Top Hosts',
  processes: 'Top Processes',
  addresses: 'Top Addresses',
  ports: 'Top Ports',
  users: 'Top Users',
};

const timeRanges = [
  { value: 1, label: '1h' },
  { value: 6, label: '6h' },
  { value: 24, label: '24h' },
  { value: 72, label: '3d' },
  { value: 168, label: '7d' },
];

const tooltipStyle = { background: '#111118', border: '1px solid #27272a', borderRadius: 8 };
const tickStyle = { fontSize: 10, fill: '#a1a1aa' };

function TimeRangeSelector({ hours, onChange }: { hours: number; onChange: (h: number) => void }) {
  return (
    <div className="flex gap-1">
      {timeRanges.map((r) => (
        <button
          key={r.value}
          onClick={() => onChange(r.value)}
          className={`px-3 py-1 text-xs rounded-lg border transition-colors ${
            hours === r.value
              ? 'bg-primary/10 text-primary border-primary/30'
              : 'bg-card border-border text-muted-foreground hover:text-foreground hover:bg-muted'
          }`}
        >
          {r.label}
        </button>
      ))}
    </div>
  );
}

function formatBucketLabel(bucket: string) {
  // "2026-03-17 14:15" → "14:15"
  return bucket.slice(11, 16);
}

function TimelineView() {
  const [hours, setHours] = useState(24);
  const [data, setData] = useState<Array<{ bucket: string; allow: number; deny: number; total: number }>>([]);

  useEffect(() => {
    const fetch = () => api.getTimeSeries(hours).then((d) => setData(d || [])).catch(console.error);
    fetch();
    const interval = setInterval(fetch, 30000);
    return () => clearInterval(interval);
  }, [hours]);

  const totalAllow = data.reduce((s, d) => s + d.allow, 0);
  const totalDeny = data.reduce((s, d) => s + d.deny, 0);
  const totalAll = totalAllow + totalDeny;
  const allowPct = totalAll > 0 ? Math.round((totalAllow / totalAll) * 100) : 0;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium">Connections Over Time</h2>
        <TimeRangeSelector hours={hours} onChange={setHours} />
      </div>

      <div className="bg-card border border-border rounded-xl p-4">
        <div className="h-[200px] md:h-[300px]">
          {data.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={data}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis dataKey="bucket" tickFormatter={formatBucketLabel} tick={tickStyle} />
                <YAxis tick={tickStyle} />
                <Tooltip
                  contentStyle={tooltipStyle}
                  labelFormatter={(label) => String(label)}
                  formatter={(value?: number, name?: string) => [formatNumber(value ?? 0), name ?? '']}
                />
                <Area type="monotone" dataKey="allow" stroke="#22c55e" fill="#22c55e" fillOpacity={0.1} />
                <Area type="monotone" dataKey="deny" stroke="#ef4444" fill="#ef4444" fillOpacity={0.1} />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-full flex items-center justify-center text-muted-foreground text-sm">No data for this time range</div>
          )}
        </div>
      </div>

      {/* Allow/Deny ratio */}
      {totalAll > 0 && (
        <div className="bg-card border border-border rounded-xl p-4 space-y-2">
          <h2 className="text-sm font-medium">Allow / Deny Ratio</h2>
          <div className="flex items-center gap-3">
            <div className="flex-1 h-3 rounded-full bg-muted overflow-hidden flex">
              <div className="bg-success h-full transition-all" style={{ width: `${allowPct}%` }} />
              <div className="bg-destructive h-full transition-all" style={{ width: `${100 - allowPct}%` }} />
            </div>
            <span className="text-xs text-muted-foreground whitespace-nowrap">
              {formatNumber(totalAllow)} / {formatNumber(totalDeny)}
            </span>
          </div>
          <div className="flex gap-4 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-success" />
              Allow {allowPct}%
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-destructive" />
              Deny {100 - allowPct}%
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

function BlockedView() {
  const isMobile = useIsMobile();
  const [hours, setHours] = useState(24);
  const [blockedHosts, setBlockedHosts] = useState<Array<{ what: string; hits: number }>>([]);
  const [blockedProcs, setBlockedProcs] = useState<Array<{ what: string; hits: number }>>([]);

  useEffect(() => {
    const fetch = () => {
      api.getTopBlocked('hosts', 20, hours).then((d) => setBlockedHosts(d || [])).catch(console.error);
      api.getTopBlocked('processes', 20, hours).then((d) => setBlockedProcs(d || [])).catch(console.error);
    };
    fetch();
    const interval = setInterval(fetch, 30000);
    return () => clearInterval(interval);
  }, [hours]);

  const prepChart = (data: Array<{ what: string; hits: number }>, shorten?: boolean) =>
    data.slice(0, 10).map((d) => {
      const short = shorten ? (d.what.split('/').pop() || d.what) : d.what;
      return {
        name: short.length > 25 ? short.substring(0, 25) + '...' : short,
        hits: d.hits,
        fullName: d.what,
      };
    });

  const hostsChart = prepChart(blockedHosts);
  const procsChart = prepChart(blockedProcs, true);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium">Top Blocked</h2>
        <TimeRangeSelector hours={hours} onChange={setHours} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Blocked hosts */}
        <div className="bg-card border border-border rounded-xl p-4 space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground">Blocked Domains</h3>
          {hostsChart.length > 0 && !isMobile ? (
            <ResponsiveContainer width="100%" height={250}>
              <BarChart data={hostsChart} layout="vertical" margin={{ left: 120 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis type="number" tick={tickStyle} />
                <YAxis type="category" dataKey="name" tick={tickStyle} width={110} />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(value?: number) => [formatNumber(value ?? 0), 'Blocked']}
                  labelFormatter={(_, payload) => payload?.[0]?.payload?.fullName || ''}
                />
                <Bar dataKey="hits" fill="#ef4444" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : hostsChart.length > 0 ? (
            <div className="space-y-1.5">
              {blockedHosts.slice(0, 10).map((d, i) => (
                <div key={i} className="flex items-center justify-between text-xs">
                  <span className="font-mono truncate max-w-[200px]">{d.what}</span>
                  <span className="text-muted-foreground">{formatNumber(d.hits)}</span>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-6 text-center text-muted-foreground text-sm">No blocked hosts</div>
          )}
        </div>

        {/* Blocked processes */}
        <div className="bg-card border border-border rounded-xl p-4 space-y-2">
          <h3 className="text-xs font-medium text-muted-foreground">Blocked Processes</h3>
          {procsChart.length > 0 && !isMobile ? (
            <ResponsiveContainer width="100%" height={250}>
              <BarChart data={procsChart} layout="vertical" margin={{ left: 120 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis type="number" tick={tickStyle} />
                <YAxis type="category" dataKey="name" tick={tickStyle} width={110} />
                <Tooltip
                  contentStyle={tooltipStyle}
                  formatter={(value?: number) => [formatNumber(value ?? 0), 'Blocked']}
                  labelFormatter={(_, payload) => payload?.[0]?.payload?.fullName || ''}
                />
                <Bar dataKey="hits" fill="#f59e0b" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : procsChart.length > 0 ? (
            <div className="space-y-1.5">
              {blockedProcs.slice(0, 10).map((d, i) => (
                <div key={i} className="flex items-center justify-between text-xs">
                  <span className="font-mono truncate max-w-[200px]">{d.what.split('/').pop() || d.what}</span>
                  <span className="text-muted-foreground">{formatNumber(d.hits)}</span>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-6 text-center text-muted-foreground text-sm">No blocked processes</div>
          )}
        </div>
      </div>
    </div>
  );
}

function countryFlag(code: string): string {
  if (!code || code.length !== 2) return '';
  return String.fromCodePoint(
    ...code.toUpperCase().split('').map((c) => 127397 + c.charCodeAt(0))
  );
}

type GeoEntry = {
  ip: string;
  country: string;
  country_code: string;
  city: string;
  lat: number;
  lon: number;
  hits: number;
};

function GeoView() {
  const isMobile = useIsMobile();
  const [hours, setHours] = useState(24);
  const [data, setData] = useState<GeoEntry[]>([]);
  const [loading, setLoading] = useState(true);

  // Reset loading state synchronously when hours changes
  const [prevHours, setPrevHours] = useState(hours);
  if (hours !== prevHours) {
    setPrevHours(hours);
    setLoading(true);
  }

  useEffect(() => {
    const fetch = () => api.getGeoSummary(hours, 50).then((d) => { setData(d || []); setLoading(false); }).catch(() => setLoading(false));
    fetch();
    const interval = setInterval(fetch, 60000);
    return () => clearInterval(interval);
  }, [hours]);

  // Aggregate by country for the chart
  const byCountry = new Map<string, { country: string; code: string; hits: number }>();
  for (const d of data) {
    const existing = byCountry.get(d.country_code);
    if (existing) {
      existing.hits += d.hits;
    } else {
      byCountry.set(d.country_code, { country: d.country, code: d.country_code, hits: d.hits });
    }
  }
  const countryData = Array.from(byCountry.values())
    .sort((a, b) => b.hits - a.hits)
    .slice(0, 15)
    .map((d) => ({
      name: `${countryFlag(d.code)} ${d.country}`,
      hits: d.hits,
      fullName: d.country,
    }));

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium">Destination Countries</h2>
        <TimeRangeSelector hours={hours} onChange={setHours} />
      </div>

      {/* Country chart */}
      {countryData.length > 0 && !isMobile && (
        <div className="bg-card border border-border rounded-xl p-4">
          <ResponsiveContainer width="100%" height={Math.max(200, countryData.length * 28)}>
            <BarChart data={countryData} layout="vertical" margin={{ left: 130 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis type="number" tick={tickStyle} />
              <YAxis type="category" dataKey="name" tick={tickStyle} width={120} />
              <Tooltip
                contentStyle={tooltipStyle}
                formatter={(value?: number) => [formatNumber(value ?? 0), 'Connections']}
              />
              <Bar dataKey="hits" fill="#8b5cf6" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* IP table */}
      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">#</th>
                <th className="px-4 py-2">Country</th>
                <th className="px-4 py-2">City</th>
                <th className="px-4 py-2">IP</th>
                <th className="px-4 py-2">Connections</th>
              </tr>
            </thead>
            <tbody>
              {data.map((d, i) => (
                <tr key={i} className="border-b border-border/50 hover:bg-muted/50">
                  <td className="px-4 py-2 text-xs text-muted-foreground">{i + 1}</td>
                  <td className="px-4 py-2 text-xs">
                    {countryFlag(d.country_code)} {d.country}
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{d.city}</td>
                  <td className="px-4 py-2 font-mono text-xs">{d.ip}</td>
                  <td className="px-4 py-2 text-xs">{formatNumber(d.hits)}</td>
                </tr>
              ))}
              {data.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-muted-foreground">
                    {loading ? 'Loading...' : 'No geo data available'}
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

function TopNView({ table }: { table: string }) {
  const isMobile = useIsMobile();
  const [data, setData] = useState<TopNRecord[]>([]);

  useEffect(() => {
    if (!table) return;
    const fetch = () => api.getStatsByTable(table, 100).then((d) => setData(d || [])).catch(console.error);
    fetch();
    const interval = setInterval(fetch, 10000);
    return () => clearInterval(interval);
  }, [table]);

  const displayValue = (what: string) =>
    table === 'processes' ? what.split('/').pop() || what : what;

  const chartData = data.slice(0, 20).map((d) => ({
    name: displayValue(d.what as string)?.length > 30
      ? displayValue(d.what as string).substring(0, 30) + '...'
      : displayValue(d.what as string),
    hits: d.hits,
    fullName: d.what,
  }));

  return (
    <div className="space-y-4">
      {/* Chart — hidden on mobile */}
      {chartData.length > 0 && !isMobile && (
        <div className="bg-card border border-border rounded-xl p-4">
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 150 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis type="number" tick={tickStyle} />
              <YAxis type="category" dataKey="name" tick={tickStyle} width={140} />
              <Tooltip
                contentStyle={tooltipStyle}
                formatter={(value?: number) => [formatNumber(value ?? 0), 'Hits']}
                labelFormatter={(_, payload) => payload?.[0]?.payload?.fullName || ''}
              />
              <Bar dataKey="hits" fill="#3b82f6" radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Table */}
      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">#</th>
                <th className="px-4 py-2">Value</th>
                <th className="px-4 py-2">Hits</th>
                <th className="px-4 py-2">Node</th>
              </tr>
            </thead>
            <tbody>
              {data.map((d, i) => (
                <tr key={i} className="border-b border-border/50 hover:bg-muted/50">
                  <td className="px-4 py-2 text-xs text-muted-foreground">{i + 1}</td>
                  <td className="px-4 py-2 font-mono text-xs break-all" title={d.what}>{displayValue(d.what)}</td>
                  <td className="px-4 py-2 text-xs">{formatNumber(d.hits)}</td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{d.node}</td>
                </tr>
              ))}
              {data.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-muted-foreground">No data</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export default function StatsPage() {
  const { table } = useParams<{ table: string }>();
  const navigate = useNavigate();

  const renderContent = () => {
    switch (table) {
      case 'timeline':
        return <TimelineView />;
      case 'blocked':
        return <BlockedView />;
      case 'geo':
        return <GeoView />;
      default:
        return <TopNView table={table || 'hosts'} />;
    }
  };

  return (
    <div className="space-y-4 md:space-y-6">
      <h1 className="text-xl font-bold">{tableTitles[table || ''] || 'Stats'}</h1>

      {/* Tab bar */}
      <div className="flex gap-1 overflow-x-auto pb-1 -mx-4 px-4 md:mx-0 md:px-0 scrollbar-none">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => navigate(`/stats/${t.key}`)}
            className={`shrink-0 px-4 py-2 text-sm rounded-lg border transition-colors ${
              table === t.key
                ? 'bg-primary/10 text-primary border-primary/30'
                : 'bg-card border-border text-muted-foreground hover:text-foreground hover:bg-muted'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {renderContent()}
    </div>
  );
}
