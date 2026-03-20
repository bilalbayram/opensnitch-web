import { useEffect, useState } from 'react';
import { api, type DNSDomainRecord, type DNSServerRecord, type DNSPolicyConfig } from '@/lib/api';
import { Search, ChevronLeft, ChevronRight, Trash2, ShieldCheck, Shield, ShieldOff, CheckCircle2, XCircle, Lock } from 'lucide-react';

type Tab = 'domains' | 'servers' | 'policy';

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

  // DNS Policy state
  const [policyNode, setPolicyNode] = useState('');
  const [policyResolvers, setPolicyResolvers] = useState('1.1.1.1, 8.8.8.8');
  const [policyBlockDoT, setPolicyBlockDoT] = useState(true);
  const [policyBlockDoHIPs, setPolicyBlockDoHIPs] = useState(true);
  const [policyBlockDoHHostnames, setPolicyBlockDoHHostnames] = useState(true);
  const [policyLoading, setPolicyLoading] = useState(false);
  const [policyError, setPolicyError] = useState<string | null>(null);
  const [activePolicy, setActivePolicy] = useState<DNSPolicyConfig | null>(null);
  const [policyRuleCount, setPolicyRuleCount] = useState(0);
  const [dohProviderCount, setDohProviderCount] = useState(0);
  const [dohHostnameCount, setDohHostnameCount] = useState(0);

  const limit = 50;

  useEffect(() => {
    api.getNodes().then(setNodes).catch(console.error);
    api.getDNSPolicyProviders().then((p) => {
      setDohProviderCount(p.doh_ips.length);
      setDohHostnameCount(p.doh_hostnames.length);
    }).catch(console.error);
  }, []);

  // Fetch policy status when node changes on the policy tab
  const fetchPolicyStatus = (node: string) => {
    if (!node) {
      setActivePolicy(null);
      setPolicyRuleCount(0);
      return;
    }
    api.getDNSPolicy(node).then((res) => {
      setActivePolicy(res.policy);
      setPolicyRuleCount(res.rule_count);
    }).catch(console.error);
  };

  useEffect(() => {
    if (tab === 'policy') fetchPolicyStatus(policyNode);
  }, [tab, policyNode]);

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
    else if (tab === 'servers') fetchServers();
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

  const handleEnablePolicy = async () => {
    const resolvers = policyResolvers.split(',').map(s => s.trim()).filter(Boolean);
    if (!policyNode || resolvers.length === 0) return;

    setPolicyLoading(true);
    setPolicyError(null);
    try {
      const res = await api.setDNSPolicy({
        node: policyNode,
        enabled: true,
        allowed_resolvers: resolvers,
        block_dot: policyBlockDoT,
        block_doh_ips: policyBlockDoHIPs,
        block_doh_hostnames: policyBlockDoHHostnames,
      });
      setActivePolicy(res.policy);
      setPolicyRuleCount(res.rule_count);
    } catch (err: unknown) {
      setPolicyError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setPolicyLoading(false);
    }
  };

  const handleDisablePolicy = async () => {
    if (!policyNode) return;
    if (!confirm('Disable DNS policy? All policy rules will be removed.')) return;

    setPolicyLoading(true);
    setPolicyError(null);
    try {
      await api.setDNSPolicy({ node: policyNode, enabled: false });
      setActivePolicy(null);
      setPolicyRuleCount(0);
    } catch (err: unknown) {
      setPolicyError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setPolicyLoading(false);
    }
  };

  // Check if any node has an active policy (for the restriction form guard)
  const isPolicyActiveForRuleNode = activePolicy?.enabled && policyNode === ruleNode;
  const selectedPolicyNode = nodes.find((n) => n.addr === policyNode);
  const policyNodeOnline = selectedPolicyNode?.online ?? false;

  const domainsPages = Math.ceil(domainsTotal / limit);
  const serversPages = Math.ceil(serversTotal / limit);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">DNS</h1>
        <div className="flex items-center gap-3">
          {tab !== 'policy' && (
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
          )}
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
        <button
          onClick={() => setTab('policy')}
          className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
            tab === 'policy' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          Policy
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

      {/* DNS Policy Tab */}
      {tab === 'policy' && (
        <div className="space-y-4">
          {/* Node selector for policy */}
          <div className="flex items-center gap-3">
            <select
              value={policyNode}
              onChange={(e) => setPolicyNode(e.target.value)}
              className="bg-card border border-border rounded-lg px-3 py-2 text-sm"
            >
              <option value="">Select Node...</option>
              {nodes.map((n) => (
                <option key={n.addr} value={n.addr}>
                  {n.hostname || n.addr}
                </option>
              ))}
            </select>
          </div>

          {!policyNode && (
            <div className="bg-card border border-border rounded-xl p-8 text-center">
              <Shield className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
              <h3 className="font-medium mb-1">Enforce DNS Policy</h3>
              <p className="text-sm text-muted-foreground max-w-md mx-auto">
                Select a node to configure DNS policy enforcement. This locks DNS to approved resolvers and blocks bypass vectors (DoH, DoT).
              </p>
            </div>
          )}

          {policyNode && !policyNodeOnline && (
            <div className="bg-orange-500/5 border border-orange-500/20 rounded-xl p-4 text-sm text-orange-600">
              This node is offline. DNS policy changes are only applied while the node is connected.
            </div>
          )}

          {/* Policy active — status view */}
          {policyNode && activePolicy?.enabled && (
            <div className="bg-card border border-success/30 rounded-xl p-5 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-success/10">
                  <Shield className="h-5 w-5 text-success" />
                </div>
                <div>
                  <h3 className="font-medium">DNS Policy Active</h3>
                  <p className="text-xs text-muted-foreground">
                    {policyRuleCount} rules enforced{activePolicy.enabled_at ? ` since ${activePolicy.enabled_at}` : ''}
                  </p>
                </div>
              </div>

              <div className="grid gap-2">
                <div className="flex items-center gap-2 text-sm">
                  <Lock className="h-3.5 w-3.5 text-success" />
                  <span>DNS locked to: <span className="font-mono text-xs">{activePolicy.allowed_resolvers.join(', ')}</span></span>
                </div>
                {activePolicy.block_dot && (
                  <div className="flex items-center gap-2 text-sm">
                    <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    <span>DNS-over-TLS blocked (port 853)</span>
                  </div>
                )}
                {activePolicy.block_doh_ips && (
                  <div className="flex items-center gap-2 text-sm">
                    <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    <span>{dohProviderCount} DoH provider IPs blocked on port 443</span>
                  </div>
                )}
                {activePolicy.block_doh_hostnames && (
                  <div className="flex items-center gap-2 text-sm">
                    <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                    <span>{dohHostnameCount} DoH hostnames blocked on port 443</span>
                  </div>
                )}
                {!activePolicy.block_dot && (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <XCircle className="h-3.5 w-3.5" />
                    <span>DNS-over-TLS not blocked</span>
                  </div>
                )}
                {!activePolicy.block_doh_ips && (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <XCircle className="h-3.5 w-3.5" />
                    <span>DoH provider IPs not blocked</span>
                  </div>
                )}
                {!activePolicy.block_doh_hostnames && (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <XCircle className="h-3.5 w-3.5" />
                    <span>DoH hostnames not blocked</span>
                  </div>
                )}
              </div>

              <button
                onClick={handleDisablePolicy}
                disabled={policyLoading || !policyNodeOnline}
                className="flex items-center gap-2 text-sm px-4 py-2 rounded-lg border border-destructive/30 text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
              >
                <ShieldOff className="h-4 w-4" />
                {policyLoading ? 'Disabling...' : 'Disable Policy'}
              </button>

              {policyError && (
                <p className="text-sm text-destructive">{policyError}</p>
              )}
            </div>
          )}

          {/* Policy not active — configuration form */}
          {policyNode && !activePolicy?.enabled && (
            <div className="bg-card border border-border rounded-xl p-5 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-muted">
                  <Shield className="h-5 w-5 text-muted-foreground" />
                </div>
                <div>
                  <h3 className="font-medium">Enforce DNS Policy</h3>
                  <p className="text-xs text-muted-foreground">
                    Lock DNS to approved resolvers and block bypass vectors (DoH, DoT) to make blocklists enforceable.
                  </p>
                </div>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="text-xs font-medium text-muted-foreground block mb-1">Allowed Resolvers (comma-separated IPs)</label>
                  <input
                    type="text"
                    value={policyResolvers}
                    onChange={(e) => setPolicyResolvers(e.target.value)}
                    placeholder="1.1.1.1, 8.8.8.8"
                    className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </div>

                <div className="space-y-2">
                  <label className="text-xs font-medium text-muted-foreground block">Bypass Protection</label>
                  <label className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      checked={policyBlockDoT}
                      onChange={(e) => setPolicyBlockDoT(e.target.checked)}
                      className="rounded border-border"
                    />
                    Block DNS-over-TLS (port 853)
                  </label>
                  <label className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      checked={policyBlockDoHIPs}
                      onChange={(e) => setPolicyBlockDoHIPs(e.target.checked)}
                      className="rounded border-border"
                    />
                    Block DoH provider IPs ({dohProviderCount} known IPs on port 443)
                  </label>
                  <label className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      checked={policyBlockDoHHostnames}
                      onChange={(e) => setPolicyBlockDoHHostnames(e.target.checked)}
                      className="rounded border-border"
                    />
                    Block DoH hostnames ({dohHostnameCount} known endpoints on port 443)
                  </label>
                </div>
              </div>

              <button
                onClick={handleEnablePolicy}
                disabled={policyLoading || !policyResolvers.trim() || !policyNodeOnline}
                className="flex items-center gap-2 bg-primary text-primary-foreground px-4 py-2 rounded-lg text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                <Shield className="h-4 w-4" />
                {policyLoading ? 'Enabling...' : 'Enable Policy'}
              </button>

              {policyError && (
                <p className="text-sm text-destructive">{policyError}</p>
              )}
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
            {isPolicyActiveForRuleNode ? (
              <p className="text-sm text-muted-foreground flex items-center gap-2">
                <Lock className="h-4 w-4 text-primary" />
                DNS restriction is managed by the active DNS policy. Use the Policy tab to manage DNS restrictions.
              </p>
            ) : (
              <>
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
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
