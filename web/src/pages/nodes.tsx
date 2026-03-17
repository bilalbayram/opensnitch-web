import { useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { formatUptime } from '@/lib/utils';
import { Server, Play, Pause, Shield, ShieldOff, ShieldCheck, ChevronDown, ChevronUp, Trash2, Plus } from 'lucide-react';
import { ResponsiveDataView } from '@/components/ui/responsive-data-view';

const modeOptions = [
  { value: 'ask', label: 'Ask', description: 'Prompt for every unknown connection' },
  { value: 'silent_allow', label: 'Silent Allow', description: 'Allow all connections without prompting' },
  { value: 'silent_deny', label: 'Silent Deny', description: 'Deny all connections without prompting' },
];

export default function NodesPage() {
  const [nodes, setNodes] = useState<any[]>([]);
  const [status, setStatus] = useState<Record<string, string>>({});
  const pendingRef = useRef(0);
  const [trustExpanded, setTrustExpanded] = useState<Record<string, boolean>>({});
  const [trustData, setTrustData] = useState<Record<string, any[]>>({});
  const [newTrustPath, setNewTrustPath] = useState<Record<string, string>>({});
  const [newTrustLevel, setNewTrustLevel] = useState<Record<string, string>>({});

  const fetchNodes = (force?: boolean) => {
    api.getNodes().then((data) => {
      if (force || pendingRef.current === 0) setNodes(data);
    }).catch(console.error);
  };

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(fetchNodes, 5000);
    return () => clearInterval(interval);
  }, []);

  const showStatus = (addr: string, msg: string) => {
    setStatus((prev) => ({ ...prev, [addr]: msg }));
    setTimeout(() => setStatus((prev) => {
      const next = { ...prev };
      delete next[addr];
      return next;
    }), 2000);
  };

  const handleAction = async (addr: string, action: string) => {
    pendingRef.current++;
    try {
      switch (action) {
        case 'enable-interception': await api.enableInterception(addr); break;
        case 'disable-interception': await api.disableInterception(addr); break;
        case 'enable-firewall': await api.enableFirewall(addr); break;
        case 'disable-firewall': await api.disableFirewall(addr); break;
      }
      showStatus(addr, 'Sent!');
    } catch (e) {
      console.error('Action failed:', e);
      showStatus(addr, 'Failed');
    } finally {
      pendingRef.current--;
      fetchNodes(true);
    }
  };

  const handleModeChange = async (addr: string, mode: string) => {
    const prev = nodes.map((n) => ({ ...n }));
    setNodes((cur) => cur.map((n) => n.addr === addr ? { ...n, mode } : n));
    pendingRef.current++;
    try {
      await api.setNodeMode(addr, mode);
      showStatus(addr, 'Mode updated');
    } catch (e) {
      console.error('Mode change failed:', e);
      setNodes(prev);
      showStatus(addr, 'Mode change failed');
    } finally {
      pendingRef.current--;
      fetchNodes(true);
    }
  };

  const fetchTrust = (addr: string) => {
    api.getProcessTrust(addr).then((data) => {
      setTrustData((prev) => ({ ...prev, [addr]: data }));
    }).catch(console.error);
  };

  const toggleTrustExpand = (addr: string) => {
    const expanding = !trustExpanded[addr];
    setTrustExpanded((prev) => ({ ...prev, [addr]: expanding }));
    if (expanding && !trustData[addr]) {
      fetchTrust(addr);
    }
  };

  const handleAddTrust = async (addr: string) => {
    const path = newTrustPath[addr]?.trim();
    const level = newTrustLevel[addr] || 'trusted';
    if (!path) return;
    try {
      await api.addProcessTrust(addr, path, level);
      setNewTrustPath((prev) => ({ ...prev, [addr]: '' }));
      setNewTrustLevel((prev) => ({ ...prev, [addr]: '' }));
      fetchTrust(addr);
    } catch (e: any) {
      showStatus(addr, e.message || 'Failed to add');
    }
  };

  const handleUpdateTrust = async (addr: string, id: number, level: string) => {
    try {
      await api.updateProcessTrust(addr, id, level);
      fetchTrust(addr);
    } catch (e: any) {
      showStatus(addr, e.message || 'Failed to update');
    }
  };

  const handleDeleteTrust = async (addr: string, id: number) => {
    try {
      await api.deleteProcessTrust(addr, id);
      fetchTrust(addr);
    } catch (e: any) {
      showStatus(addr, e.message || 'Failed to delete');
    }
  };

  const trustLevelOptions = ['trusted', 'untrusted', 'default'] as const;
  const trustLevelColors: Record<string, string> = {
    trusted: 'bg-success/10 text-success border-success/30',
    untrusted: 'bg-destructive/10 text-destructive border-destructive/30',
    default: 'bg-primary/10 text-primary border-primary/30',
  };

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">Nodes</h1>

      <div className="grid gap-4">
        {nodes.map((node) => (
          <div key={node.addr} className="bg-card border border-border rounded-xl p-4 md:p-5">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className={`p-2 rounded-lg ${node.online ? 'bg-success/10' : 'bg-muted'}`}>
                  <Server className={`h-5 w-5 ${node.online ? 'text-success' : 'text-muted-foreground'}`} />
                </div>
                <div>
                  <div className="font-medium">{node.hostname || node.addr}</div>
                  <div className="text-xs text-muted-foreground">{node.addr}</div>
                </div>
              </div>
              <span className={`text-xs px-2 py-1 rounded-full ${
                node.online
                  ? 'bg-success/10 text-success'
                  : 'bg-muted text-muted-foreground'
              }`}>
                {node.online ? 'Online' : 'Offline'}
              </span>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mt-4 text-sm">
              <div>
                <div className="text-xs text-muted-foreground">Version</div>
                <div>{node.daemon_version || '-'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Uptime</div>
                <div>{node.daemon_uptime ? formatUptime(node.daemon_uptime) : '-'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Connections</div>
                <div>{node.cons || 0}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Rules</div>
                <div>{node.daemon_rules || 0}</div>
              </div>
            </div>

            {/* Mode selector */}
            <div className="mt-4 flex flex-wrap items-center gap-2 md:gap-3">
              <span className="text-xs text-muted-foreground">Mode:</span>
              <div className="flex flex-wrap gap-1">
                {modeOptions.map((opt) => (
                  <button
                    key={opt.value}
                    onClick={() => handleModeChange(node.addr, opt.value)}
                    title={opt.description}
                    className={`text-xs px-3 py-2 md:py-1.5 rounded-lg border transition-colors ${
                      node.mode === opt.value
                        ? opt.value === 'ask'
                          ? 'bg-primary/10 text-primary border-primary/30'
                          : opt.value === 'silent_allow'
                            ? 'bg-success/10 text-success border-success/30'
                            : 'bg-destructive/10 text-destructive border-destructive/30'
                        : 'bg-muted border-border hover:bg-muted/80'
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
              {status[node.addr] && (
                <span className={`text-xs px-2 py-0.5 rounded-full ${
                  status[node.addr].includes('fail') || status[node.addr] === 'Failed'
                    ? 'bg-destructive/10 text-destructive'
                    : 'bg-success/10 text-success'
                }`}>
                  {status[node.addr]}
                </span>
              )}
            </div>

            {node.online && (
              <div className="flex flex-wrap gap-2 mt-4">
                <button
                  onClick={() => handleAction(node.addr, 'enable-interception')}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Play className="h-3 w-3" /> <span className="hidden sm:inline">Enable</span> Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'disable-interception')}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Pause className="h-3 w-3" /> <span className="hidden sm:inline">Disable</span> Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'enable-firewall')}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Shield className="h-3 w-3" /> Enable FW
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'disable-firewall')}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <ShieldOff className="h-3 w-3" /> Disable FW
                </button>
              </div>
            )}

            {/* Trust List */}
            <div className="mt-4 border-t border-border pt-4">
              <button
                onClick={() => toggleTrustExpand(node.addr)}
                className="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors w-full py-1"
              >
                <ShieldCheck className="h-4 w-4" />
                Trust List ({trustData[node.addr]?.length || 0} entries)
                {trustExpanded[node.addr] ? <ChevronUp className="h-4 w-4 ml-auto" /> : <ChevronDown className="h-4 w-4 ml-auto" />}
              </button>

              {trustExpanded[node.addr] && (
                <div className="mt-3 space-y-3">
                  {/* Add new entry — stack on mobile */}
                  <div className="flex flex-col sm:flex-row gap-2">
                    <input
                      type="text"
                      placeholder="/usr/bin/..."
                      value={newTrustPath[node.addr] || ''}
                      onChange={(e) => setNewTrustPath((prev) => ({ ...prev, [node.addr]: e.target.value }))}
                      onKeyDown={(e) => e.key === 'Enter' && handleAddTrust(node.addr)}
                      className="flex-1 text-xs px-3 py-2 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                    />
                    <div className="flex gap-2">
                      <select
                        value={newTrustLevel[node.addr] || 'trusted'}
                        onChange={(e) => setNewTrustLevel((prev) => ({ ...prev, [node.addr]: e.target.value }))}
                        className="text-xs px-2 py-2 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                      >
                        {trustLevelOptions.map((lvl) => (
                          <option key={lvl} value={lvl}>{lvl}</option>
                        ))}
                      </select>
                      <button
                        onClick={() => handleAddTrust(node.addr)}
                        className="flex items-center gap-1 text-xs px-3 py-2 rounded-lg bg-primary/10 text-primary border border-primary/30 hover:bg-primary/20"
                      >
                        <Plus className="h-3 w-3" /> Add
                      </button>
                    </div>
                  </div>

                  {/* Trust entries */}
                  {trustData[node.addr]?.length > 0 && (
                    <ResponsiveDataView
                      data={trustData[node.addr] || []}
                      columns={4}
                      emptyMessage="No trust entries"
                      tableHead={
                        <tr className="bg-muted/50">
                          <th className="text-left px-3 py-2 text-xs font-medium">Process Path</th>
                          <th className="text-left px-3 py-2 text-xs font-medium w-20">Scope</th>
                          <th className="text-left px-3 py-2 text-xs font-medium w-52">Trust Level</th>
                          <th className="w-10"></th>
                        </tr>
                      }
                      renderRow={(entry: any) => (
                        <tr key={entry.id} className="border-t border-border">
                          <td className="px-3 py-2 font-mono text-xs">{entry.process_path}</td>
                          <td className="px-3 py-2">
                            <span className={`text-xs px-1.5 py-0.5 rounded ${
                              entry.node === '*'
                                ? 'bg-muted text-muted-foreground'
                                : 'bg-primary/10 text-primary'
                            }`}>
                              {entry.node === '*' ? 'Global' : 'This node'}
                            </span>
                          </td>
                          <td className="px-3 py-2">
                            <div className="flex gap-1">
                              {trustLevelOptions.map((lvl) => (
                                <button
                                  key={lvl}
                                  onClick={() => handleUpdateTrust(node.addr, entry.id, lvl)}
                                  className={`text-xs px-2 py-1 rounded-md border transition-colors ${
                                    entry.trust_level === lvl
                                      ? trustLevelColors[lvl]
                                      : 'bg-muted border-border hover:bg-muted/80'
                                  }`}
                                >
                                  {lvl}
                                </button>
                              ))}
                            </div>
                          </td>
                          <td className="px-3 py-2">
                            <button
                              onClick={() => handleDeleteTrust(node.addr, entry.id)}
                              className="text-muted-foreground hover:text-destructive transition-colors p-1"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </button>
                          </td>
                        </tr>
                      )}
                      renderCard={(entry: any) => (
                        <div key={entry.id} className="bg-muted/30 border border-border rounded-xl p-3 space-y-2">
                          <div className="flex items-start justify-between gap-2">
                            <div className="font-mono text-xs break-all flex-1">{entry.process_path}</div>
                            <button
                              onClick={() => handleDeleteTrust(node.addr, entry.id)}
                              className="text-muted-foreground hover:text-destructive transition-colors p-1.5 shrink-0"
                            >
                              <Trash2 className="h-4 w-4" />
                            </button>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className={`text-xs px-1.5 py-0.5 rounded ${
                              entry.node === '*'
                                ? 'bg-muted text-muted-foreground'
                                : 'bg-primary/10 text-primary'
                            }`}>
                              {entry.node === '*' ? 'Global' : 'This node'}
                            </span>
                          </div>
                          <div className="flex flex-wrap gap-1.5">
                            {trustLevelOptions.map((lvl) => (
                              <button
                                key={lvl}
                                onClick={() => handleUpdateTrust(node.addr, entry.id, lvl)}
                                className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                                  entry.trust_level === lvl
                                    ? trustLevelColors[lvl]
                                    : 'bg-muted border-border hover:bg-muted/80'
                                }`}
                              >
                                {lvl}
                              </button>
                            ))}
                          </div>
                        </div>
                      )}
                    />
                  )}
                </div>
              )}
            </div>
          </div>
        ))}
        {nodes.length === 0 && (
          <div className="bg-card border border-border rounded-xl p-8 text-center text-muted-foreground">
            No nodes found. Configure an OpenSnitch daemon to connect to this server.
          </div>
        )}
      </div>
    </div>
  );
}
