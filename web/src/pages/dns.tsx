import { useEffect, useState } from 'react';
import { api, type DNSDomainRecord, type DNSServerRecord } from '@/lib/api';
import { Search, ChevronLeft, ChevronRight, Trash2, ShieldCheck } from 'lucide-react';

type Tab = 'domains' | 'servers';

interface NodeInfo {
  addr: string;
  hostname: string;
  online: boolean;
}

export default function DNSPage() {
  const [tab, setTab] = useState<Tab>('domains');
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [selectedNode, setSelectedNode] = useState('');

  // Domain resolutions state
  const [domains, setDomains] = useState<DNSDomainRecord[]>([]);
  const [domainsTotal, setDomainsTotal] = useState(0);
  const [domainsPage, setDomainsPage] = useState(0);
  const [domainsSearch, setDomainsSearch] = useState('');

  // DNS servers state
  const [servers, setServers] = useState<DNSServerRecord[]>([]);
  const [serversTotal, setServersTotal] = useState(0);
  const [serversPage, setServersPage] = useState(0);

  // DNS rule creation state
  const [showRuleForm, setShowRuleForm] = useState(false);
  const [ruleNode, setRuleNode] = useState('');
  const [ruleIPs, setRuleIPs] = useState('');
  const [ruleCreating, setRuleCreating] = useState(false);
  const [ruleResult, setRuleResult] = useState<string | null>(null);

  const limit = 50;

  useEffect(() => {
    api.getNodes().then(setNodes).catch(console.error);
  }, []);

  const fetchDomains = () => {
    const params: Record<string, string> = {
      limit: String(limit),
      offset: String(domainsPage * limit),
    };
    if (selectedNode) params.node = selectedNode;
    if (domainsSearch) params.search = domainsSearch;

    api.getDNSDomains(params).then((res) => {
      setDomains(res.data || []);
      setDomainsTotal(res.total);
    }).catch(console.error);
  };

  const fetchServers = () => {
    const params: Record<string, string> = {
      limit: String(limit),
      offset: String(serversPage * limit),
    };
    if (selectedNode) params.node = selectedNode;

    api.getDNSServers(params).then((res) => {
      setServers(res.data || []);
      setServersTotal(res.total);
    }).catch(console.error);
  };

  useEffect(() => {
    if (tab === 'domains') fetchDomains();
    else fetchServers();
  }, [tab, selectedNode, domainsPage, serversPage, domainsSearch]);

  const handleDomainsSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setDomainsPage(0);
    fetchDomains();
  };

  const handlePurge = async () => {
    if (!confirm('Purge all DNS domain mappings? This cannot be undone.')) return;
    try {
      await api.purgeDNSDomains();
      fetchDomains();
    } catch (err: unknown) {
      console.error('Failed to purge DNS domains:', err);
    }
  };

  const handleNodeChange = (value: string) => {
    setSelectedNode(value);
    setDomainsPage(0);
    setServersPage(0);
  };

  const handleCreateRules = async () => {
    const ips = ruleIPs.split(',').map(s => s.trim()).filter(Boolean);
    if (!ruleNode || ips.length === 0) return;

    setRuleCreating(true);
    setRuleResult(null);
    try {
      const res = await api.createDNSServerRules({ node: ruleNode, allowed_ips: ips });
      setRuleResult(`Created ${res.count} DNS rules successfully.`);
      setRuleIPs('');
    } catch (err: unknown) {
      setRuleResult(`Error: ${err instanceof Error ? err.message : "Unknown error"}`);
    } finally {
      setRuleCreating(false);
    }
  };

  const domainsPages = Math.ceil(domainsTotal / limit);
  const serversPages = Math.ceil(serversTotal / limit);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">DNS</h1>
        <div className="flex items-center gap-3">
          <select
            value={selectedNode}
            onChange={(e) => handleNodeChange(e.target.value)}
            className="bg-card border border-border rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All Nodes</option>
            {nodes.map((n) => (
              <option key={n.addr} value={n.addr}>
                {n.hostname || n.addr}
              </option>
            ))}
          </select>
          {tab === 'domains' && (
            <>
              <span className="text-sm text-muted-foreground">{domainsTotal} domains</span>
              <button onClick={handlePurge} className="text-muted-foreground hover:text-destructive transition-colors" title="Purge all">
                <Trash2 className="h-4 w-4" />
              </button>
            </>
          )}
          {tab === 'servers' && (
            <span className="text-sm text-muted-foreground">{serversTotal} entries</span>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-muted rounded-lg p-1 w-fit">
        <button
          onClick={() => setTab('domains')}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
            tab === 'domains' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          Domain Resolutions
        </button>
        <button
          onClick={() => setTab('servers')}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
            tab === 'servers' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          DNS Servers
        </button>
      </div>

      {/* Domain Resolutions Tab */}
      {tab === 'domains' && (
        <>
          <form onSubmit={handleDomainsSearch} className="relative max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              value={domainsSearch}
              onChange={(e) => setDomainsSearch(e.target.value)}
              placeholder="Search domain or IP..."
              className="w-full bg-card border border-border rounded-lg pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </form>

          <div className="bg-card border border-border rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs text-muted-foreground">
                    <th className="px-4 py-2">Domain</th>
                    <th className="px-4 py-2">IP</th>
                    <th className="px-4 py-2">Node</th>
                    <th className="px-4 py-2">Hits</th>
                    <th className="px-4 py-2">First Seen</th>
                    <th className="px-4 py-2">Last Seen</th>
                  </tr>
                </thead>
                <tbody>
                  {domains.map((d) => (
                    <tr key={d.id} className="border-b border-border/50 hover:bg-muted/50">
                      <td className="px-4 py-2 font-medium">{d.domain}</td>
                      <td className="px-4 py-2 font-mono text-xs">{d.ip}</td>
                      <td className="px-4 py-2 text-xs">{d.node}</td>
                      <td className="px-4 py-2 text-xs">{d.hit_count}</td>
                      <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{d.first_seen}</td>
                      <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{d.last_seen}</td>
                    </tr>
                  ))}
                  {domains.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-4 py-8 text-center text-muted-foreground">
                        No domain resolutions recorded yet
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {domainsPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-border">
                <span className="text-xs text-muted-foreground">
                  Page {domainsPage + 1} of {domainsPages}
                </span>
                <div className="flex gap-1">
                  <button
                    onClick={() => setDomainsPage((p) => Math.max(0, p - 1))}
                    disabled={domainsPage === 0}
                    className="p-1 rounded hover:bg-muted disabled:opacity-30"
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => setDomainsPage((p) => Math.min(domainsPages - 1, p + 1))}
                    disabled={domainsPage >= domainsPages - 1}
                    className="p-1 rounded hover:bg-muted disabled:opacity-30"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </button>
                </div>
              </div>
            )}
          </div>
        </>
      )}

      {/* DNS Servers Tab */}
      {tab === 'servers' && (
        <div className="bg-card border border-border rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-4 py-2">DNS Server</th>
                  <th className="px-4 py-2">Process</th>
                  <th className="px-4 py-2">Protocol</th>
                  <th className="px-4 py-2">Action</th>
                  <th className="px-4 py-2">Hits</th>
                  <th className="px-4 py-2">First Seen</th>
                  <th className="px-4 py-2">Last Seen</th>
                </tr>
              </thead>
              <tbody>
                {servers.map((s, i) => (
                  <tr key={i} className="border-b border-border/50 hover:bg-muted/50">
                    <td className="px-4 py-2 font-mono font-medium">{s.dst_ip}</td>
                    <td className="px-4 py-2 font-mono text-xs max-w-48 truncate" title={s.process}>{s.process}</td>
                    <td className="px-4 py-2 text-xs uppercase">{s.protocol}</td>
                    <td className="px-4 py-2 text-xs">{s.action}</td>
                    <td className="px-4 py-2 text-xs">{s.hits}</td>
                    <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{s.first_seen}</td>
                    <td className="px-4 py-2 text-xs text-muted-foreground whitespace-nowrap">{s.last_seen}</td>
                  </tr>
                ))}
                {servers.length === 0 && (
                  <tr>
                    <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                      No DNS server queries recorded yet
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {serversPages > 1 && (
            <div className="flex items-center justify-between px-4 py-3 border-t border-border">
              <span className="text-xs text-muted-foreground">
                Page {serversPage + 1} of {serversPages}
              </span>
              <div className="flex gap-1">
                <button
                  onClick={() => setServersPage((p) => Math.max(0, p - 1))}
                  disabled={serversPage === 0}
                  className="p-1 rounded hover:bg-muted disabled:opacity-30"
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <button
                  onClick={() => setServersPage((p) => Math.min(serversPages - 1, p + 1))}
                  disabled={serversPage >= serversPages - 1}
                  className="p-1 rounded hover:bg-muted disabled:opacity-30"
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* DNS Server Rules */}
      <div className="bg-card border border-border rounded-xl">
        <button
          onClick={() => setShowRuleForm(!showRuleForm)}
          className="w-full flex items-center gap-2 px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors rounded-xl"
        >
          <ShieldCheck className="h-4 w-4 text-primary" />
          Restrict DNS Servers
          <span className="ml-auto text-xs text-muted-foreground">{showRuleForm ? '▲' : '▼'}</span>
        </button>

        {showRuleForm && (
          <div className="px-4 pb-4 space-y-3 border-t border-border pt-3">
            <p className="text-xs text-muted-foreground">
              Create rules to restrict which DNS servers a node can use. All DNS traffic to non-allowed servers will be denied.
            </p>
            <div className="flex gap-3 flex-wrap">
              <select
                value={ruleNode}
                onChange={(e) => setRuleNode(e.target.value)}
                className="bg-background border border-border rounded-lg px-3 py-2 text-sm"
              >
                <option value="">Select Node...</option>
                {nodes.map((n) => (
                  <option key={n.addr} value={n.addr}>
                    {n.hostname || n.addr}
                  </option>
                ))}
              </select>
              <input
                type="text"
                value={ruleIPs}
                onChange={(e) => setRuleIPs(e.target.value)}
                placeholder="Allowed IPs: 1.1.1.1, 8.8.8.8"
                className="flex-1 min-w-64 bg-background border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
              />
              <button
                onClick={handleCreateRules}
                disabled={ruleCreating || !ruleNode || !ruleIPs.trim()}
                className="bg-primary text-primary-foreground px-4 py-2 rounded-lg text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {ruleCreating ? 'Creating...' : 'Apply'}
              </button>
            </div>
            {ruleResult && (
              <p className={`text-sm ${ruleResult.startsWith('Error') ? 'text-destructive' : 'text-success'}`}>
                {ruleResult}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
