import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { Flame, RefreshCw } from 'lucide-react';

export default function FirewallPage() {
  const [fwState, setFwState] = useState<any[]>([]);

  const fetchFirewall = () => {
    api.getFirewall().then(setFwState).catch(console.error);
  };

  useEffect(() => { fetchFirewall(); }, []);

  const handleReload = async (nodeAddr?: string) => {
    try {
      await api.reloadFirewall(nodeAddr);
      fetchFirewall();
    } catch (e) {
      console.error('Reload failed:', e);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Firewall</h1>
        <button
          onClick={() => handleReload()}
          className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-lg px-3 py-1.5 text-sm font-medium hover:bg-primary/80"
        >
          <RefreshCw className="h-4 w-4" /> Reload All
        </button>
      </div>

      {fwState.map((fw, i) => (
        <div key={i} className="bg-card border border-border rounded-xl p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Flame className={`h-5 w-5 ${fw.running ? 'text-success' : 'text-muted-foreground'}`} />
              <span className="font-medium">{fw.node_addr}</span>
              <span className={`text-xs px-2 py-0.5 rounded-full ${fw.running ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'}`}>
                {fw.running ? 'Running' : 'Stopped'}
              </span>
            </div>
            <button
              onClick={() => handleReload(fw.node_addr)}
              className="text-muted-foreground hover:text-foreground"
            >
              <RefreshCw className="h-4 w-4" />
            </button>
          </div>

          {fw.firewall?.SystemRules?.map((sr: any, j: number) => (
            <div key={j} className="mb-3">
              {sr.Chains?.map((chain: any, k: number) => (
                <div key={k} className="bg-muted rounded-lg p-3 mb-2">
                  <div className="text-xs font-medium mb-1">
                    {chain.Name} ({chain.Family} / {chain.Hook} / {chain.Type}) — Policy: {chain.Policy}
                  </div>
                  {chain.Rules?.map((rule: any, l: number) => (
                    <div key={l} className="text-xs text-muted-foreground font-mono ml-2">
                      [{rule.Position}] {rule.Description || rule.Parameters} → {rule.Target}
                    </div>
                  ))}
                  {(!chain.Rules || chain.Rules.length === 0) && (
                    <div className="text-xs text-muted-foreground ml-2">No rules</div>
                  )}
                </div>
              ))}
            </div>
          ))}

          {(!fw.firewall?.SystemRules || fw.firewall.SystemRules.length === 0) && (
            <div className="text-sm text-muted-foreground">No firewall rules data</div>
          )}
        </div>
      ))}

      {fwState.length === 0 && (
        <div className="bg-card border border-border rounded-xl p-8 text-center text-muted-foreground">
          No firewall data. Connect a daemon first.
        </div>
      )}
    </div>
  );
}
