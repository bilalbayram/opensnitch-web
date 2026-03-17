import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { alertTypeLabel, priorityLabel } from '@/lib/utils';
import { Trash2, ChevronLeft, ChevronRight } from 'lucide-react';
import { ResponsiveDataView } from '@/components/ui/responsive-data-view';

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const limit = 50;

  const fetchAlerts = () => {
    api.getAlerts(limit, page * limit).then((res) => {
      setAlerts(res.data || []);
      setTotal(res.total);
    }).catch(console.error);
  };

  useEffect(() => { fetchAlerts(); }, [page]);

  const handleDelete = async (id: number) => {
    await api.deleteAlert(id);
    fetchAlerts();
  };

  const totalPages = Math.ceil(total / limit);

  const typeColor = (t: number) => {
    switch (t) {
      case 0: return 'text-destructive';
      case 1: return 'text-warning';
      case 2: return 'text-primary';
      default: return 'text-muted-foreground';
    }
  };

  const typeBg = (t: number) => {
    switch (t) {
      case 0: return 'bg-destructive/15 text-destructive';
      case 1: return 'bg-warning/15 text-warning';
      case 2: return 'bg-primary/15 text-primary';
      default: return 'bg-muted text-muted-foreground';
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Alerts</h1>
        <span className="text-sm text-muted-foreground">{total} total</span>
      </div>

      <ResponsiveDataView
        data={alerts}
        columns={6}
        emptyMessage="No alerts"
        tableHead={
          <tr className="border-b border-border text-left text-xs text-muted-foreground">
            <th className="px-4 py-2">Time</th>
            <th className="px-4 py-2">Node</th>
            <th className="px-4 py-2">Type</th>
            <th className="px-4 py-2">Priority</th>
            <th className="px-4 py-2">Body</th>
            <th className="px-4 py-2"></th>
          </tr>
        }
        renderRow={(a: any) => (
          <tr key={a.id} className="border-b border-border/50 hover:bg-muted/50">
            <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{a.time}</td>
            <td className="px-4 py-2 text-xs">{a.node}</td>
            <td className={`px-4 py-2 text-xs font-medium ${typeColor(a.type)}`}>{alertTypeLabel(a.type)}</td>
            <td className="px-4 py-2 text-xs">{priorityLabel(a.priority)}</td>
            <td className="px-4 py-2 text-xs max-w-md truncate">{a.body}</td>
            <td className="px-4 py-2">
              <button onClick={() => handleDelete(a.id)} className="text-muted-foreground hover:text-destructive p-1">
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            </td>
          </tr>
        )}
        renderCard={(a: any) => (
          <div key={a.id} className="bg-card border border-border rounded-xl p-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${typeBg(a.type)}`}>
                  {alertTypeLabel(a.type)}
                </span>
                <span className="text-xs text-muted-foreground">{priorityLabel(a.priority)}</span>
              </div>
              <button onClick={() => handleDelete(a.id)} className="text-muted-foreground hover:text-destructive p-1.5">
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
            <div className="text-xs text-foreground/90">{a.body}</div>
            <div className="text-[10px] text-muted-foreground">{a.node} · {a.time}</div>
          </div>
        )}
      />

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between bg-card border border-border rounded-xl px-4 py-3">
          <span className="text-xs text-muted-foreground">Page {page + 1} of {totalPages}</span>
          <div className="flex gap-1">
            <button onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={page === 0} className="p-2 rounded-lg hover:bg-muted disabled:opacity-30">
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))} disabled={page >= totalPages - 1} className="p-2 rounded-lg hover:bg-muted disabled:opacity-30">
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
