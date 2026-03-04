import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '@/lib/api';
import { formatNumber } from '@/lib/utils';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

const tableTitles: Record<string, string> = {
  hosts: 'Top Hosts',
  processes: 'Top Processes',
  addresses: 'Top Addresses',
  ports: 'Top Ports',
  users: 'Top Users',
};

export default function StatsPage() {
  const { table } = useParams<{ table: string }>();
  const [data, setData] = useState<any[]>([]);

  useEffect(() => {
    if (!table) return;
    api.getStatsByTable(table, 100).then((d) => setData(d || [])).catch(console.error);
    const interval = setInterval(() => {
      api.getStatsByTable(table, 100).then((d) => setData(d || [])).catch(console.error);
    }, 10000);
    return () => clearInterval(interval);
  }, [table]);

  const chartData = data.slice(0, 20).map((d) => ({
    name: d.what?.length > 30 ? d.what.substring(0, 30) + '...' : d.what,
    hits: d.hits,
    fullName: d.what,
  }));

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">{tableTitles[table || ''] || 'Stats'}</h1>

      {/* Chart */}
      {chartData.length > 0 && (
        <div className="bg-card border border-border rounded-xl p-4">
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 150 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
              <XAxis type="number" tick={{ fontSize: 10, fill: '#a1a1aa' }} />
              <YAxis type="category" dataKey="name" tick={{ fontSize: 10, fill: '#a1a1aa' }} width={140} />
              <Tooltip
                contentStyle={{ background: '#111118', border: '1px solid #27272a', borderRadius: 8 }}
                formatter={(value: number) => [formatNumber(value), 'Hits']}
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
                  <td className="px-4 py-2 font-mono text-xs">{d.what}</td>
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
