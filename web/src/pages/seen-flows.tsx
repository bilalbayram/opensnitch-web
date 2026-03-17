import { useEffect, useState } from 'react';
import { Search, ChevronLeft, ChevronRight } from 'lucide-react';

import { api, type SeenFlowRecord } from '@/lib/api';
import { actionColor } from '@/lib/utils';

export default function SeenFlowsPage() {
  const [flows, setFlows] = useState<SeenFlowRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [node, setNode] = useState('');
  const [nodeInput, setNodeInput] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const limit = 50;

  useEffect(() => {
    const params: Record<string, string> = {
      limit: String(limit),
      offset: String(page * limit),
    };
    if (search) params.search = search;
    if (node) params.node = node;
    if (actionFilter) params.action = actionFilter;

    api.getSeenFlows(params).then((res) => {
      setFlows(res.data || []);
      setTotal(res.total);
    }).catch(console.error);
  }, [page, search, node, actionFilter]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(0);
    setSearch(searchInput.trim());
    setNode(nodeInput.trim());
  };

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Seen Flows</h1>
        <span className="text-sm text-muted-foreground">{total} total</span>
      </div>

      <div className="flex gap-3">
        <form onSubmit={handleSearch} className="flex-1 grid gap-3 md:grid-cols-[minmax(0,1fr),200px]">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search process, destination, protocol..."
              className="w-full bg-card border border-border rounded-lg pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <input
            type="text"
            value={nodeInput}
            onChange={(e) => setNodeInput(e.target.value)}
            placeholder="Filter by node"
            className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </form>
        <select
          value={actionFilter}
          onChange={(e) => {
            setActionFilter(e.target.value);
            setPage(0);
          }}
          className="bg-card border border-border rounded-lg px-3 py-2 text-sm"
        >
          <option value="">All Actions</option>
          <option value="allow">Allow</option>
          <option value="deny">Deny</option>
          <option value="reject">Reject</option>
        </select>
      </div>

      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Node</th>
                <th className="px-4 py-2">Process</th>
                <th className="px-4 py-2">Destination</th>
                <th className="px-4 py-2">Protocol / Port</th>
                <th className="px-4 py-2">Count</th>
                <th className="px-4 py-2">First Seen</th>
                <th className="px-4 py-2">Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {flows.map((flow) => (
                <tr key={flow.id} className="border-b border-border/50 hover:bg-muted/50">
                  <td className={`px-4 py-2 font-medium ${actionColor(flow.action)}`}>{flow.action}</td>
                  <td className="px-4 py-2 text-xs whitespace-nowrap">{flow.node}</td>
                  <td className="px-4 py-2 font-mono text-xs max-w-64 truncate" title={flow.process}>{flow.process}</td>
                  <td className="px-4 py-2 text-xs">
                    <div>{flow.destination}</div>
                    <div className="text-muted-foreground">{flow.destination_operand}</div>
                  </td>
                  <td className="px-4 py-2 text-xs uppercase">{flow.protocol}:{flow.dst_port}</td>
                  <td className="px-4 py-2 text-xs">{flow.count}</td>
                  <td className="px-4 py-2 text-xs whitespace-nowrap">{flow.first_seen}</td>
                  <td className="px-4 py-2 text-xs whitespace-nowrap">{flow.last_seen}</td>
                </tr>
              ))}
              {flows.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">No seen flows found</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-border">
            <span className="text-xs text-muted-foreground">
              Page {page + 1} of {totalPages}
            </span>
            <div className="flex gap-1">
              <button
                onClick={() => setPage((current) => Math.max(0, current - 1))}
                disabled={page === 0}
                className="p-1 rounded hover:bg-muted disabled:opacity-30"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <button
                onClick={() => setPage((current) => Math.min(totalPages - 1, current + 1))}
                disabled={page >= totalPages - 1}
                className="p-1 rounded hover:bg-muted disabled:opacity-30"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
