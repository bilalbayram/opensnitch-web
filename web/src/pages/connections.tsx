import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { actionColor, truncateMiddle } from '@/lib/utils';
import { Search, ChevronLeft, ChevronRight, Trash2 } from 'lucide-react';
import { ResponsiveDataView } from '@/components/ui/responsive-data-view';

export default function ConnectionsPage() {
  const [connections, setConnections] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const limit = 50;

  const fetchConnections = () => {
    const params: Record<string, string> = {
      limit: String(limit),
      offset: String(page * limit),
    };
    if (search) params.search = search;
    if (actionFilter) params.action = actionFilter;

    api.getConnections(params).then((res) => {
      setConnections(res.data || []);
      setTotal(res.total);
    }).catch(console.error);
  };

  useEffect(() => { fetchConnections(); }, [page, actionFilter]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(0);
    fetchConnections();
  };

  const handlePurge = async () => {
    if (!confirm('Purge all connections? This cannot be undone.')) return;
    await api.purgeConnections();
    fetchConnections();
  };

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Connections</h1>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground hidden sm:inline">{total} total</span>
          <button onClick={handlePurge} className="text-muted-foreground hover:text-destructive transition-colors p-1" title="Purge all">
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Filters — stack on mobile */}
      <div className="flex flex-col sm:flex-row gap-3">
        <form onSubmit={handleSearch} className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search host, process, IP, rule..."
            className="w-full bg-card border border-border rounded-lg pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </form>
        <select
          value={actionFilter}
          onChange={(e) => { setActionFilter(e.target.value); setPage(0); }}
          className="bg-card border border-border rounded-lg px-3 py-2 text-sm"
        >
          <option value="">All Actions</option>
          <option value="allow">Allow</option>
          <option value="deny">Deny</option>
          <option value="reject">Reject</option>
        </select>
      </div>

      {/* Data */}
      <ResponsiveDataView
        data={connections}
        columns={8}
        emptyMessage="No connections found"
        tableHead={
          <tr className="border-b border-border text-left text-xs text-muted-foreground">
            <th className="px-4 py-2">Time</th>
            <th className="px-4 py-2">Node</th>
            <th className="px-4 py-2">Action</th>
            <th className="px-4 py-2">Protocol</th>
            <th className="px-4 py-2">Source</th>
            <th className="px-4 py-2">Destination</th>
            <th className="px-4 py-2">Process</th>
            <th className="px-4 py-2">Rule</th>
          </tr>
        }
        renderRow={(c: any) => (
          <tr key={c.id} className="border-b border-border/50 hover:bg-muted/50">
            <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{c.time}</td>
            <td className="px-4 py-2 text-xs">{c.node}</td>
            <td className={`px-4 py-2 font-medium ${actionColor(c.action)}`}>{c.action}</td>
            <td className="px-4 py-2 text-xs uppercase">{c.protocol}</td>
            <td className="px-4 py-2 text-xs">{c.src_ip}:{c.src_port}</td>
            <td className="px-4 py-2 text-xs">{c.dst_host || c.dst_ip}:{c.dst_port}</td>
            <td className="px-4 py-2 font-mono text-xs max-w-48 truncate" title={c.process}>{c.process}</td>
            <td className="px-4 py-2 text-xs text-muted-foreground">{c.rule}</td>
          </tr>
        )}
        renderCard={(c: any) => (
          <div key={c.id} className="bg-card border border-border rounded-xl p-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
                  c.action === 'allow' ? 'bg-success/15 text-success'
                    : c.action === 'deny' ? 'bg-destructive/15 text-destructive'
                    : 'bg-warning/15 text-warning'
                }`}>
                  {c.action}
                </span>
                <span className="text-xs text-muted-foreground uppercase">{c.protocol}</span>
              </div>
              <span className="text-[10px] text-muted-foreground">{c.time}</span>
            </div>
            <div className="font-mono text-xs break-all text-foreground/90">
              {truncateMiddle(c.process || '', 60)}
            </div>
            <div className="text-xs text-muted-foreground">
              → {c.dst_host || c.dst_ip}:{c.dst_port}
            </div>
            {c.rule && (
              <div className="text-[10px] text-muted-foreground/70">Rule: {c.rule}</div>
            )}
          </div>
        )}
      />

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between bg-card border border-border rounded-xl px-4 py-3">
          <span className="text-xs text-muted-foreground">
            Page {page + 1} of {totalPages}
          </span>
          <div className="flex gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="p-2 rounded-lg hover:bg-muted disabled:opacity-30"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className="p-2 rounded-lg hover:bg-muted disabled:opacity-30"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
