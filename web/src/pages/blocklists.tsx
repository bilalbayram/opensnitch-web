import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { formatNumber } from '@/lib/utils';
import { ShieldBan, RefreshCw, Trash2, Plus, X } from 'lucide-react';

interface Blocklist {
  id: number;
  name: string;
  url: string;
  category: string;
  description: string;
  enabled: boolean;
  domain_count: number;
  last_synced: string;
}

const categoryColors: Record<string, string> = {
  ads: 'bg-orange-500/10 text-orange-500',
  malware: 'bg-red-500/10 text-red-500',
  telemetry: 'bg-blue-500/10 text-blue-500',
};

export default function BlocklistsPage() {
  const [lists, setLists] = useState<Blocklist[]>([]);
  const [syncing, setSyncing] = useState<Set<number>>(new Set());
  const [showAdd, setShowAdd] = useState(false);
  const [newName, setNewName] = useState('');
  const [newUrl, setNewUrl] = useState('');
  const [newCategory, setNewCategory] = useState('ads');

  const fetchLists = () => {
    api.getBlocklists().then(setLists).catch(console.error);
  };

  useEffect(() => {
    fetchLists();
  }, []);

  const handleToggle = async (id: number, enabled: boolean) => {
    try {
      if (enabled) {
        await api.disableBlocklist(id);
      } else {
        await api.enableBlocklist(id);
      }
      fetchLists();
    } catch (e) {
      console.error('Toggle failed:', e);
    }
  };

  const handleSync = async (id: number) => {
    setSyncing((prev) => new Set(prev).add(id));
    try {
      await api.syncBlocklist(id);
      fetchLists();
    } catch (e) {
      console.error('Sync failed:', e);
    } finally {
      setSyncing((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteBlocklist(id);
      fetchLists();
    } catch (e) {
      console.error('Delete failed:', e);
    }
  };

  const handleAdd = async () => {
    if (!newName || !newUrl) return;
    try {
      await api.createBlocklist(newName, newUrl, newCategory);
      setShowAdd(false);
      setNewName('');
      setNewUrl('');
      setNewCategory('ads');
      fetchLists();
    } catch (e) {
      console.error('Add failed:', e);
    }
  };

  const totalEnabled = lists.filter((l) => l.enabled).length;
  const totalDomains = lists.filter((l) => l.enabled).reduce((sum, l) => sum + l.domain_count, 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold">Blocklists</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {totalEnabled} active list{totalEnabled !== 1 ? 's' : ''} blocking{' '}
            {formatNumber(totalDomains)} domain{totalDomains !== 1 ? 's' : ''}
          </p>
        </div>
        <button
          onClick={() => setShowAdd(true)}
          className="flex items-center gap-1.5 text-sm px-3 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90"
        >
          <Plus className="h-4 w-4" /> Add Custom
        </button>
      </div>

      {/* Add custom modal */}
      {showAdd && (
        <div className="bg-card border border-border rounded-xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <span className="font-medium text-sm">Add Custom Blocklist</span>
            <button onClick={() => setShowAdd(false)} className="text-muted-foreground hover:text-foreground">
              <X className="h-4 w-4" />
            </button>
          </div>
          <input
            type="text"
            placeholder="Name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="w-full px-3 py-2 text-sm rounded-lg bg-muted border border-border focus:outline-none focus:ring-1 focus:ring-primary"
          />
          <input
            type="url"
            placeholder="URL (hosts file or domain list)"
            value={newUrl}
            onChange={(e) => setNewUrl(e.target.value)}
            className="w-full px-3 py-2 text-sm rounded-lg bg-muted border border-border focus:outline-none focus:ring-1 focus:ring-primary"
          />
          <div className="flex items-center gap-3">
            <select
              value={newCategory}
              onChange={(e) => setNewCategory(e.target.value)}
              className="px-3 py-2 text-sm rounded-lg bg-muted border border-border focus:outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="ads">Ads</option>
              <option value="malware">Malware</option>
              <option value="telemetry">Telemetry</option>
            </select>
            <button
              onClick={handleAdd}
              disabled={!newName || !newUrl}
              className="flex items-center gap-1.5 text-sm px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              Add
            </button>
          </div>
        </div>
      )}

      {/* Blocklist cards */}
      <div className="grid gap-3">
        {lists.map((bl) => (
          <div
            key={bl.id}
            className={`bg-card border rounded-xl p-4 transition-colors ${
              bl.enabled ? 'border-primary/30' : 'border-border'
            }`}
          >
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className={`p-2 rounded-lg ${bl.enabled ? 'bg-primary/10' : 'bg-muted'}`}>
                  <ShieldBan className={`h-4 w-4 ${bl.enabled ? 'text-primary' : 'text-muted-foreground'}`} />
                </div>
                <div>
                  <div className="font-medium text-sm">{bl.name}</div>
                  <div className="text-xs text-muted-foreground truncate max-w-[400px]">{bl.url}</div>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <span className={`text-xs px-2 py-0.5 rounded-full ${categoryColors[bl.category] || 'bg-muted text-muted-foreground'}`}>
                  {bl.category}
                </span>
              </div>
            </div>

            <div className="flex items-center justify-between mt-3">
              <div className="flex items-center gap-4 text-xs text-muted-foreground">
                <span>{formatNumber(bl.domain_count)} domain{bl.domain_count !== 1 ? 's' : ''}</span>
                {bl.last_synced && (
                  <span>Synced: {bl.last_synced}</span>
                )}
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => handleSync(bl.id)}
                  disabled={syncing.has(bl.id)}
                  title="Sync domains from URL"
                  className="flex items-center gap-1 text-xs px-2.5 py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border disabled:opacity-50"
                >
                  <RefreshCw className={`h-3 w-3 ${syncing.has(bl.id) ? 'animate-spin' : ''}`} />
                  {syncing.has(bl.id) ? 'Syncing...' : 'Sync'}
                </button>
                <button
                  onClick={() => handleToggle(bl.id, bl.enabled)}
                  className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                    bl.enabled
                      ? 'bg-primary/10 text-primary border-primary/30'
                      : 'bg-muted border-border hover:bg-muted/80'
                  }`}
                >
                  {bl.enabled ? 'Enabled' : 'Disabled'}
                </button>
                <button
                  onClick={() => handleDelete(bl.id)}
                  title="Delete blocklist"
                  className="text-xs p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        ))}
        {lists.length === 0 && (
          <div className="bg-card border border-border rounded-xl p-8 text-center text-muted-foreground">
            No blocklists configured. Add a custom blocklist or check the database seed.
          </div>
        )}
      </div>
    </div>
  );
}
