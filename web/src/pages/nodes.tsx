import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { formatUptime } from '@/lib/utils';
import { Server, Play, Pause, Shield, ShieldOff } from 'lucide-react';

const modeOptions = [
  { value: 'ask', label: 'Ask', description: 'Prompt for every unknown connection' },
  { value: 'silent_allow', label: 'Silent Allow', description: 'Allow all connections without prompting' },
  { value: 'silent_deny', label: 'Silent Deny', description: 'Deny all connections without prompting' },
];

export default function NodesPage() {
  const [nodes, setNodes] = useState<any[]>([]);
  const [status, setStatus] = useState<Record<string, string>>({});

  const fetchNodes = () => {
    api.getNodes().then(setNodes).catch(console.error);
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
    try {
      switch (action) {
        case 'enable-interception': await api.enableInterception(addr); break;
        case 'disable-interception': await api.disableInterception(addr); break;
        case 'enable-firewall': await api.enableFirewall(addr); break;
        case 'disable-firewall': await api.disableFirewall(addr); break;
      }
      showStatus(addr, 'Sent!');
      fetchNodes();
    } catch (e) {
      console.error('Action failed:', e);
      showStatus(addr, 'Failed');
    }
  };

  const handleModeChange = async (addr: string, mode: string) => {
    const prev = nodes.map((n) => ({ ...n }));
    setNodes((cur) => cur.map((n) => n.addr === addr ? { ...n, mode } : n));
    try {
      await api.setNodeMode(addr, mode);
      showStatus(addr, 'Mode updated');
      fetchNodes();
    } catch (e) {
      console.error('Mode change failed:', e);
      setNodes(prev);
      showStatus(addr, 'Mode change failed');
    }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">Nodes</h1>

      <div className="grid gap-4">
        {nodes.map((node) => (
          <div key={node.addr} className="bg-card border border-border rounded-xl p-5">
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

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 text-sm">
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
            <div className="mt-4 flex items-center gap-3">
              <span className="text-xs text-muted-foreground">Mode:</span>
              <div className="flex gap-1">
                {modeOptions.map((opt) => (
                  <button
                    key={opt.value}
                    onClick={() => handleModeChange(node.addr, opt.value)}
                    title={opt.description}
                    className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
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
              <div className="flex gap-2 mt-4">
                <button
                  onClick={() => handleAction(node.addr, 'enable-interception')}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Play className="h-3 w-3" /> Enable Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'disable-interception')}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Pause className="h-3 w-3" /> Disable Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'enable-firewall')}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Shield className="h-3 w-3" /> Enable FW
                </button>
                <button
                  onClick={() => handleAction(node.addr, 'disable-firewall')}
                  className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <ShieldOff className="h-3 w-3" /> Disable FW
                </button>
              </div>
            )}
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
