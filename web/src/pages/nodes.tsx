import { useEffect, useMemo, useRef, useState } from "react";
import type {
  NodeRecord,
  ProvisionStep,
  DiscoveredRouter,
  RouterRecord,
  RouterCapabilities,
} from "@/lib/api";
import { api } from "@/lib/api";
import { formatUptime } from "@/lib/utils";
import {
  Server,
  Play,
  Pause,
  Shield,
  ShieldOff,
  ShieldCheck,
  ChevronDown,
  ChevronUp,
  Trash2,
  Plus,
  Router,
  Check,
  X,
  Loader2,
  Unplug,
  Radar,
  Wifi,
  AlertTriangle,
} from "lucide-react";
import { ResponsiveDataView } from "@/components/ui/responsive-data-view";
import { BottomSheet } from "@/components/ui/bottom-sheet";

const modeOptions = [
  {
    value: "ask",
    label: "Ask",
    description: "Prompt for every unknown connection",
  },
  {
    value: "silent_allow",
    label: "Silent Allow",
    description: "Allow all connections without prompting",
  },
  {
    value: "silent_deny",
    label: "Silent Deny",
    description: "Deny all connections without prompting",
  },
];

type RouterConnectMode = "monitor" | "manage";

interface RouterFormState {
  addr: string;
  ssh_port: number;
  ssh_user: string;
  ssh_pass: string;
  ssh_key: string;
  name: string;
  lan_subnet: string;
  server_url: string;
  mode: RouterConnectMode;
}

const defaultRouterForm: RouterFormState = {
  addr: "",
  ssh_port: 22,
  ssh_user: "root",
  ssh_pass: "",
  ssh_key: "",
  name: "",
  lan_subnet: "",
  server_url: "",
  mode: "monitor",
};

const routerConnectModeOptions = [
  {
    value: "monitor" as const,
    label: "Monitor",
    description: "Best compatibility. Tracks forwarded traffic with the legacy router agent.",
  },
  {
    value: "manage" as const,
    label: "Manage",
    description: "Deploys router-daemon for router-local prompts and runtime controls when supported.",
  },
];

interface TrustEntry {
  id: number;
  node: string;
  process_path: string;
  trust_level: string;
}

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [routers, setRouters] = useState<RouterRecord[]>([]);
  const [status, setStatus] = useState<Record<string, string>>({});
  const pendingRef = useRef(0);
  const [trustExpanded, setTrustExpanded] = useState<Record<string, boolean>>(
    {},
  );
  const [trustData, setTrustData] = useState<Record<string, TrustEntry[]>>({});
  const [newTrustPath, setNewTrustPath] = useState<Record<string, string>>({});
  const [newTrustLevel, setNewTrustLevel] = useState<Record<string, string>>(
    {},
  );
  const [tagDrafts, setTagDrafts] = useState<Record<string, string>>({});

  // Router connection state
  const [showConnectRouter, setShowConnectRouter] = useState(false);
  const [routerForm, setRouterForm] = useState({ ...defaultRouterForm });
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [serverUrlSource, setServerUrlSource] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectSteps, setConnectSteps] = useState<ProvisionStep[] | null>(
    null,
  );
  const [connectError, setConnectError] = useState("");
  const [connectWarning, setConnectWarning] = useState("");

  // Network scan state
  const [scanning, setScanning] = useState(false);
  const [scanResults, setScanResults] = useState<DiscoveredRouter[] | null>(null);
  const [scanSubnet, setScanSubnet] = useState("");

  const [routerPasswords, setRouterPasswords] = useState<Record<string, string>>(
    {},
  );
  const [routerCapabilities, setRouterCapabilities] = useState<
    Record<string, RouterCapabilities | null>
  >({});
  const [routerSteps, setRouterSteps] = useState<
    Record<string, ProvisionStep[] | null>
  >({});
  const [routerBusy, setRouterBusy] = useState<Record<string, string>>({});
  const [deletingNodes, setDeletingNodes] = useState<Record<string, boolean>>({});

  const fetchPageData = (force?: boolean) => {
    Promise.all([api.getNodes(), api.getRouters()])
      .then(([nodeData, routerData]) => {
        if (force || pendingRef.current === 0) {
          setNodes(nodeData);
          setRouters(routerData);
          setTagDrafts((prev) => {
            const next = { ...prev };
            for (const node of nodeData) {
              if (!(node.addr in next)) {
                next[node.addr] = node.tags.join(", ");
              }
            }
            return next;
          });
        }
      })
      .catch(console.error);
  };

  useEffect(() => {
    fetchPageData();
    const interval = setInterval(fetchPageData, 5000);
    return () => clearInterval(interval);
  }, []);

  const showStatus = (addr: string, msg: string) => {
    setStatus((prev) => ({ ...prev, [addr]: msg }));
    setTimeout(
      () =>
        setStatus((prev) => {
          const next = { ...prev };
          delete next[addr];
          return next;
        }),
      2000,
    );
  };

  const handleAction = async (addr: string, action: string) => {
    pendingRef.current++;
    try {
      switch (action) {
        case "enable-interception":
          await api.enableInterception(addr);
          break;
        case "disable-interception":
          await api.disableInterception(addr);
          break;
        case "enable-firewall":
          await api.enableFirewall(addr);
          break;
        case "disable-firewall":
          await api.disableFirewall(addr);
          break;
      }
      showStatus(addr, "Sent!");
    } catch (e) {
      console.error("Action failed:", e);
      showStatus(addr, "Failed");
    } finally {
      pendingRef.current--;
      fetchPageData(true);
    }
  };

  const handleModeChange = async (addr: string, mode: string) => {
    const prev = nodes.map((n) => ({ ...n }));
    setNodes((cur) => cur.map((n) => (n.addr === addr ? { ...n, mode } : n)));
    pendingRef.current++;
    try {
      await api.setNodeMode(addr, mode);
      showStatus(addr, "Mode updated");
    } catch (e) {
      console.error("Mode change failed:", e);
      setNodes(prev);
      showStatus(addr, "Mode change failed");
    } finally {
      pendingRef.current--;
      fetchPageData(true);
    }
  };

  const fetchTrust = (addr: string) => {
    api
      .getProcessTrust(addr)
      .then((data) => {
        setTrustData((prev) => ({
          ...prev,
          [addr]: data as unknown as TrustEntry[],
        }));
      })
      .catch(console.error);
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
    const level = newTrustLevel[addr] || "trusted";
    if (!path) return;
    try {
      await api.addProcessTrust(addr, path, level);
      setNewTrustPath((prev) => ({ ...prev, [addr]: "" }));
      setNewTrustLevel((prev) => ({ ...prev, [addr]: "" }));
      fetchTrust(addr);
    } catch (e: unknown) {
      showStatus(addr, e instanceof Error ? e.message : "Failed to add");
    }
  };

  const handleUpdateTrust = async (addr: string, id: number, level: string) => {
    try {
      await api.updateProcessTrust(addr, id, level);
      fetchTrust(addr);
    } catch (e: unknown) {
      showStatus(addr, e instanceof Error ? e.message : "Failed to update");
    }
  };

  const handleDeleteTrust = async (addr: string, id: number) => {
    try {
      await api.deleteProcessTrust(addr, id);
      fetchTrust(addr);
    } catch (e: unknown) {
      showStatus(addr, e instanceof Error ? e.message : "Failed to delete");
    }
  };

  const handleSaveTags = async (addr: string) => {
    const raw = tagDrafts[addr] || "";
    const tags = raw
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean);

    pendingRef.current++;
    try {
      const res = await api.replaceNodeTags(addr, tags);
      setTagDrafts((prev) => ({ ...prev, [addr]: res.tags.join(", ") }));
      showStatus(
        addr,
        res.template_sync_pending ? "Tags saved, sync pending" : "Tags saved",
      );
    } catch (e) {
      console.error("Tag update failed:", e);
      showStatus(addr, "Tag update failed");
    } finally {
      pendingRef.current--;
      fetchPageData(true);
    }
  };

  const handleDeleteNode = async (node: NodeRecord) => {
    const label = node.hostname || node.addr;
    if (!window.confirm(`Delete ${label} and its stored data? This cannot be undone.`)) {
      return;
    }

    setDeletingNodes((prev) => ({ ...prev, [node.addr]: true }));
    try {
      await api.deleteNode(node.addr);
      showStatus(node.addr, "Node deleted");
      fetchPageData(true);
    } catch (e) {
      console.error("Node delete failed:", e);
      showStatus(node.addr, e instanceof Error ? e.message : "Node delete failed");
    } finally {
      setDeletingNodes((prev) => {
        const next = { ...prev };
        delete next[node.addr];
        return next;
      });
    }
  };

  const handleConnectRouter = async () => {
    setConnecting(true);
    setConnectError("");
    setConnectWarning("");
    setConnectSteps(null);
    try {
      const res = await api.connectRouter(routerForm);
      setConnectSteps(res.steps);
      fetchPageData(true);
      setConnecting(false);
      if (res.warning) {
        setConnectWarning(res.warning);
        return;
      }
      setTimeout(() => {
        setShowConnectRouter(false);
        setConnectSteps(null);
        setRouterForm({ ...defaultRouterForm });
        setShowAdvanced(false);
        setServerUrlSource("");
        setConnectWarning("");
      }, 2000);
    } catch (e: unknown) {
      // Try to parse steps from the error response body
      const err = e as Error & Record<string, unknown>;
      let errorSteps: ProvisionStep[] | null = null;
      if (err.steps) {
        errorSteps = err.steps as ProvisionStep[];
      }
      if (errorSteps) {
        setConnectSteps(errorSteps);
      }
      setConnectWarning("");
      setConnectError(err.message || "Connection failed");
      setConnecting(false);
    }
  };

  const handleDisconnectRouter = async (addr: string) => {
    const sshPass = routerPasswords[addr]?.trim();
    if (!sshPass) {
      showStatus(addr, "SSH password required");
      return;
    }
    try {
      setRouterBusy((prev) => ({ ...prev, [addr]: "disconnecting" }));
      const res = await api.disconnectRouter(addr, sshPass);
      setRouterSteps((prev) => ({ ...prev, [addr]: res.steps }));
      showStatus(addr, "Router disconnected");
      fetchPageData(true);
    } catch (e: unknown) {
      showStatus(addr, e instanceof Error ? e.message : "Disconnect failed");
    } finally {
      setRouterBusy((prev) => {
        const next = { ...prev };
        delete next[addr];
        return next;
      });
    }
  };

  const autoDetectSubnet = (ip: string) => {
    const parts = ip.split(".");
    if (parts.length === 4 && parts.every((p) => /^\d+$/.test(p))) {
      return `${parts[0]}.${parts[1]}.${parts[2]}.0/24`;
    }
    return "";
  };

  const handleScanNetwork = async () => {
    setScanning(true);
    setScanResults(null);
    try {
      const res = await api.scanRouters(scanSubnet || undefined);
      setScanResults(res.devices);
      if (!scanSubnet) setScanSubnet(res.subnet);
    } catch (e: unknown) {
      setConnectError(e instanceof Error ? e.message : "Scan failed");
    } finally {
      setScanning(false);
    }
  };

  const selectDiscoveredRouter = (device: DiscoveredRouter) => {
    setRouterForm((prev) => ({
      ...prev,
      addr: device.ip,
      ssh_port: device.ssh_port,
      lan_subnet: autoDetectSubnet(device.ip),
    }));
    setScanResults(null);
  };

  const trustLevelOptions = ["trusted", "untrusted", "default"] as const;
  const trustLevelColors: Record<string, string> = {
    trusted: "bg-success/10 text-success border-success/30",
    untrusted: "bg-destructive/10 text-destructive border-destructive/30",
    default: "bg-primary/10 text-primary border-primary/30",
  };

  const handleRouterPasswordChange = (addr: string, value: string) => {
    setRouterPasswords((prev) => ({ ...prev, [addr]: value }));
  };

  const withRouterBusy = async (addr: string, mode: string, fn: () => Promise<void>) => {
    setRouterBusy((prev) => ({ ...prev, [addr]: mode }));
    try {
      await fn();
    } finally {
      setRouterBusy((prev) => {
        const next = { ...prev };
        delete next[addr];
        return next;
      });
    }
  };

  const handleCheckCapabilities = async (router: RouterRecord) => {
    const sshPass = routerPasswords[router.addr]?.trim();
    if (!sshPass) {
      showStatus(router.addr, "SSH password required");
      return;
    }

    await withRouterBusy(router.addr, "capabilities", async () => {
      try {
        const res = await api.getRouterCapabilities(router.addr, { ssh_pass: sshPass });
        setRouterCapabilities((prev) => ({ ...prev, [router.addr]: res.capabilities }));
        setRouterSteps((prev) => ({ ...prev, [router.addr]: res.steps }));
        showStatus(router.addr, res.capabilities.eligible ? "Eligible" : "Not eligible");
      } catch (e) {
        const err = e as Error & Record<string, unknown>;
        if (err.capabilities) {
          setRouterCapabilities((prev) => ({
            ...prev,
            [router.addr]: err.capabilities as RouterCapabilities,
          }));
        }
        if (err.steps) {
          setRouterSteps((prev) => ({
            ...prev,
            [router.addr]: err.steps as ProvisionStep[],
          }));
        }
        showStatus(router.addr, err.message || "Capability check failed");
      }
    });
  };

  const handleUpgradeRouter = async (router: RouterRecord) => {
    const sshPass = routerPasswords[router.addr]?.trim();
    if (!sshPass) {
      showStatus(router.addr, "SSH password required");
      return;
    }

    await withRouterBusy(router.addr, "upgrade", async () => {
      try {
        const res = await api.upgradeRouter(router.addr, {
          ssh_pass: sshPass,
          node_name: router.name || router.addr,
          default_action: "deny",
          poll_interval_ms: 1000,
          firewall_backend: "nft",
        });
        setRouterSteps((prev) => ({ ...prev, [router.addr]: res.steps }));
        if (res.capabilities) {
          setRouterCapabilities((prev) => ({ ...prev, [router.addr]: res.capabilities! }));
        }
        showStatus(router.addr, "Router upgraded");
        fetchPageData(true);
      } catch (e) {
        const err = e as Error & Record<string, unknown>;
        if (err.steps) {
          setRouterSteps((prev) => ({ ...prev, [router.addr]: err.steps as ProvisionStep[] }));
        }
        if (err.capabilities) {
          setRouterCapabilities((prev) => ({
            ...prev,
            [router.addr]: err.capabilities as RouterCapabilities,
          }));
        }
        showStatus(router.addr, err.message || "Upgrade failed");
      }
    });
  };

  const handleDowngradeRouter = async (router: RouterRecord) => {
    const sshPass = routerPasswords[router.addr]?.trim();
    if (!sshPass) {
      showStatus(router.addr, "SSH password required");
      return;
    }

    await withRouterBusy(router.addr, "downgrade", async () => {
      try {
        const res = await api.downgradeRouter(router.addr, { ssh_pass: sshPass });
        setRouterSteps((prev) => ({ ...prev, [router.addr]: res.steps }));
        showStatus(router.addr, "Router downgraded");
        fetchPageData(true);
      } catch (e) {
        const err = e as Error & Record<string, unknown>;
        if (err.steps) {
          setRouterSteps((prev) => ({ ...prev, [router.addr]: err.steps as ProvisionStep[] }));
        }
        showStatus(router.addr, err.message || "Downgrade failed");
      }
    });
  };

  const routerRuntimeNodes = useMemo(() => {
    return routers.map((router) => {
      const runtimeAddr =
        router.daemon_mode === "router-daemon" && router.linked_node_addr
          ? router.linked_node_addr
          : router.addr;
      return {
        router,
        runtimeNode: nodes.find((node) => node.addr === runtimeAddr),
      };
    });
  }, [nodes, routers]);

  const routerNodeAddrs = useMemo(() => {
    const result = new Set<string>();
    for (const router of routers) {
      result.add(router.addr);
      if (router.linked_node_addr) {
        result.add(router.linked_node_addr);
      }
    }
    return result;
  }, [routers]);

  const genericNodes = useMemo(
    () => nodes.filter((node) => !routerNodeAddrs.has(node.addr)),
    [nodes, routerNodeAddrs],
  );

  const renderModeControls = (node: NodeRecord) => (
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
                ? opt.value === "ask"
                  ? "bg-primary/10 text-primary border-primary/30"
                  : opt.value === "silent_allow"
                    ? "bg-success/10 text-success border-success/30"
                    : "bg-destructive/10 text-destructive border-destructive/30"
                : "bg-muted border-border hover:bg-muted/80"
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );

  const renderTagsSection = (node: NodeRecord) => (
    <div className="mt-4 rounded-xl border border-border bg-muted/30 p-4">
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="text-muted-foreground">Current tags:</span>
        {node.tags.length > 0 ? (
          node.tags.map((tag) => (
            <span
              key={`${node.addr}-${tag}`}
              className="rounded-full border border-primary/30 bg-primary/10 px-2 py-0.5 text-primary"
            >
              {tag}
            </span>
          ))
        ) : (
          <span className="text-muted-foreground">No tags</span>
        )}
        {node.template_sync_pending && (
          <span className="rounded-full border border-warning/30 bg-warning/10 px-2 py-0.5 text-warning">
            Sync pending
          </span>
        )}
        {!node.template_sync_pending &&
          !node.template_sync_error &&
          node.tags.length > 0 && (
            <span className="rounded-full border border-success/30 bg-success/10 px-2 py-0.5 text-success">
              Synced
            </span>
          )}
      </div>
      {node.template_sync_error && (
        <div className="mt-2 text-xs text-warning">{node.template_sync_error}</div>
      )}
      <div className="mt-3 flex flex-col gap-2 md:flex-row">
        <input
          type="text"
          value={tagDrafts[node.addr] || ""}
          onChange={(e) =>
            setTagDrafts((prev) => ({
              ...prev,
              [node.addr]: e.target.value,
            }))
          }
          onKeyDown={(e) => e.key === "Enter" && handleSaveTags(node.addr)}
          placeholder="server, desktop, iot"
          className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-sm focus:outline-none focus:border-primary"
        />
        <button
          onClick={() => handleSaveTags(node.addr)}
          className="rounded-lg border border-primary/30 bg-primary/10 px-4 py-2 text-sm text-primary hover:bg-primary/20"
        >
          Save Tags
        </button>
      </div>
      <div className="mt-2 text-xs text-muted-foreground">
        Tags are normalized to lowercase slugs and trigger template reconciliation.
      </div>
    </div>
  );

  const renderTrustSection = (node: NodeRecord, title: string, description?: string) => (
    <div className="mt-4 border-t border-border pt-4">
      <button
        onClick={() => toggleTrustExpand(node.addr)}
        className="flex items-center gap-2 text-sm font-medium hover:text-primary transition-colors w-full py-1"
      >
        <ShieldCheck className="h-4 w-4" />
        {title} ({trustData[node.addr]?.length || 0} entries)
        {trustExpanded[node.addr] ? (
          <ChevronUp className="h-4 w-4 ml-auto" />
        ) : (
          <ChevronDown className="h-4 w-4 ml-auto" />
        )}
      </button>
      {description && <div className="mt-1 text-xs text-muted-foreground">{description}</div>}

      {trustExpanded[node.addr] && (
        <div className="mt-3 space-y-3">
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              placeholder="/usr/bin/..."
              value={newTrustPath[node.addr] || ""}
              onChange={(e) =>
                setNewTrustPath((prev) => ({
                  ...prev,
                  [node.addr]: e.target.value,
                }))
              }
              onKeyDown={(e) => e.key === "Enter" && handleAddTrust(node.addr)}
              className="flex-1 text-xs px-3 py-2 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
            />
            <div className="flex gap-2">
              <select
                value={newTrustLevel[node.addr] || "trusted"}
                onChange={(e) =>
                  setNewTrustLevel((prev) => ({
                    ...prev,
                    [node.addr]: e.target.value,
                  }))
                }
                className="text-xs px-2 py-2 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
              >
                {trustLevelOptions.map((lvl) => (
                  <option key={lvl} value={lvl}>
                    {lvl}
                  </option>
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
              renderRow={(entry: TrustEntry) => (
                <tr key={entry.id} className="border-t border-border">
                  <td className="px-3 py-2 font-mono text-xs">{entry.process_path}</td>
                  <td className="px-3 py-2">
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded ${
                        entry.node === "*"
                          ? "bg-muted text-muted-foreground"
                          : "bg-primary/10 text-primary"
                      }`}
                    >
                      {entry.node === "*" ? "Global" : "This node"}
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
                              : "bg-muted border-border hover:bg-muted/80"
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
              renderCard={(entry: TrustEntry) => (
                <div
                  key={entry.id}
                  className="bg-muted/30 border border-border rounded-xl p-3 space-y-2"
                >
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
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded ${
                        entry.node === "*"
                          ? "bg-muted text-muted-foreground"
                          : "bg-primary/10 text-primary"
                      }`}
                    >
                      {entry.node === "*" ? "Global" : "This node"}
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
                            : "bg-muted border-border hover:bg-muted/80"
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
  );

  const renderStatusPill = (key: string) =>
    status[key] ? (
      <div className="mt-2">
        <span
          className={`text-xs px-2 py-0.5 rounded-full ${
            status[key].includes("fail") || status[key] === "Failed"
              ? "bg-destructive/10 text-destructive"
              : "bg-success/10 text-success"
          }`}
        >
          {status[key]}
        </span>
      </div>
    ) : null;

  const renderProvisionSteps = (addr: string) => {
    const steps = routerSteps[addr];
    if (!steps?.length) return null;
    return (
      <div className="mt-4 space-y-2 rounded-xl border border-border bg-muted/30 p-4">
        <div className="text-xs font-medium text-muted-foreground">Last router action</div>
        {steps.map((step, index) => (
          <div key={`${step.step}-${index}`} className="flex items-start gap-2 text-xs">
            {step.status === "done" ? (
              <Check className="mt-0.5 h-3.5 w-3.5 text-success" />
            ) : step.status === "warning" ? (
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 text-amber-500" />
            ) : (
              <X className="mt-0.5 h-3.5 w-3.5 text-destructive" />
            )}
            <div>
              <div className="font-medium capitalize">{step.step}</div>
              <div className="text-muted-foreground">{step.message}</div>
            </div>
          </div>
        ))}
      </div>
    );
  };

  const renderCapabilitySummary = (addr: string) => {
    const capability = routerCapabilities[addr];
    if (!capability) return null;
    return (
      <div className="mt-4 rounded-xl border border-border bg-muted/30 p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium">Capability check</div>
            <div className="text-xs text-muted-foreground">
              {capability.eligible
                ? "Router is eligible for router-daemon v1."
                : capability.ineligible_reason || "Router is not eligible."}
            </div>
          </div>
          <span
            className={`text-xs px-2 py-1 rounded-full ${
              capability.eligible
                ? "bg-success/10 text-success"
                : "bg-destructive/10 text-destructive"
            }`}
          >
            {capability.eligible ? "Eligible" : "Ineligible"}
          </span>
        </div>
        <div className="mt-3 grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
          <div>Arch: {capability.arch || "-"}</div>
          <div>Kernel: {capability.kernel_version || "-"}</div>
          <div>RAM: {capability.ram_mb || 0} MB</div>
          <div>Overlay: {capability.overlay_free_mb || 0} MB</div>
        </div>
      </div>
    );
  };

  const canConnectRouter = Boolean(
    routerForm.addr.trim() && (routerForm.ssh_pass || routerForm.ssh_key.trim()),
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Nodes</h1>
        <button
          onClick={() => setShowConnectRouter(true)}
          className="flex items-center gap-1.5 text-sm px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <Router className="h-4 w-4" /> Connect Router
        </button>
      </div>

      <div className="grid gap-4">
        {routerRuntimeNodes.map(({ router, runtimeNode }) => {
          const busy = routerBusy[router.addr];
          const managed = router.daemon_mode === "router-daemon";
          const linkedNode = router.linked_node;
          const online = linkedNode?.online ?? runtimeNode?.online ?? router.online;
          const nodeControls = managed && runtimeNode && runtimeNode.online;

          return (
            <div
              key={router.addr}
              className="bg-card border border-border rounded-xl p-4 md:p-5"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-center gap-3">
                  <div className={`p-2 rounded-lg ${online ? "bg-success/10" : "bg-muted"}`}>
                    <Router
                      className={`h-5 w-5 ${online ? "text-success" : "text-muted-foreground"}`}
                    />
                  </div>
                  <div>
                    <div className="font-medium">{router.name || router.addr}</div>
                    <div className="text-xs text-muted-foreground">{router.addr}</div>
                  </div>
                </div>
                <div className="flex flex-wrap items-center justify-end gap-1.5">
                  <span className="text-xs px-2 py-1 rounded-full bg-accent/10 text-accent border border-accent/20">
                    Router
                  </span>
                  <span className="text-xs px-2 py-1 rounded-full bg-primary/10 text-primary border border-primary/20">
                    {router.daemon_mode}
                  </span>
                  <span
                    className={`text-xs px-2 py-1 rounded-full ${
                      online ? "bg-success/10 text-success" : "bg-muted text-muted-foreground"
                    }`}
                  >
                    {online ? "Online" : "Offline"}
                  </span>
                </div>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mt-4 text-sm">
                <div>
                  <div className="text-xs text-muted-foreground">Runtime</div>
                  <div>{linkedNode?.daemon_version || runtimeNode?.daemon_version || "-"}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Mode</div>
                  <div>{linkedNode?.mode || runtimeNode?.mode || "-"}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Connections</div>
                  <div>{linkedNode?.cons ?? runtimeNode?.cons ?? 0}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Rules</div>
                  <div>{linkedNode?.daemon_rules ?? runtimeNode?.daemon_rules ?? 0}</div>
                </div>
              </div>

              {managed ? (
                <div className="mt-4 rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300">
                  v1 only prompts for router-local processes. Forwarded device flows are observed and enforced only by explicit device-scoped rules; unknown forwarded traffic is allowed until a rule exists.
                </div>
              ) : (
                <div className="mt-4 rounded-xl border border-border bg-muted/30 px-4 py-3 text-xs text-muted-foreground">
                  Legacy conntrack-agent mode reports forwarded traffic over HTTP ingest. Upgrade to router-daemon for router-local prompts and inline runtime controls.
                </div>
              )}

              {runtimeNode && renderTagsSection(runtimeNode)}

              <div className="mt-4 rounded-xl border border-border bg-muted/30 p-4 space-y-3">
                <div className="text-sm font-medium">Router actions</div>
                <input
                  type="password"
                  value={routerPasswords[router.addr] || ""}
                  onChange={(e) => handleRouterPasswordChange(router.addr, e.target.value)}
                  placeholder="SSH password for upgrade, downgrade, and disconnect"
                  className="w-full rounded-lg border border-border bg-card px-3 py-2 text-sm focus:outline-none focus:border-primary"
                />
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => handleCheckCapabilities(router)}
                    disabled={!!busy}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg bg-muted hover:bg-muted/80 border border-border disabled:opacity-50"
                  >
                    {busy === "capabilities" ? <Loader2 className="h-3 w-3 animate-spin" /> : <ShieldCheck className="h-3 w-3" />}
                    Check capabilities
                  </button>
                  {managed ? (
                    <button
                      onClick={() => handleDowngradeRouter(router)}
                      disabled={!!busy}
                      className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg bg-primary/10 text-primary border border-primary/30 hover:bg-primary/20 disabled:opacity-50"
                    >
                      {busy === "downgrade" ? <Loader2 className="h-3 w-3 animate-spin" /> : <Pause className="h-3 w-3" />}
                      Downgrade
                    </button>
                  ) : (
                    <button
                      onClick={() => handleUpgradeRouter(router)}
                      disabled={!!busy}
                      className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                    >
                      {busy === "upgrade" ? <Loader2 className="h-3 w-3 animate-spin" /> : <Play className="h-3 w-3" />}
                      Upgrade
                    </button>
                  )}
                  <button
                    onClick={() => handleDisconnectRouter(router.addr)}
                    disabled={!!busy}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg bg-destructive/10 text-destructive border border-destructive/30 hover:bg-destructive/20 disabled:opacity-50"
                  >
                    {busy === "disconnecting" ? <Loader2 className="h-3 w-3 animate-spin" /> : <Unplug className="h-3 w-3" />}
                    Disconnect
                  </button>
                </div>
              </div>

              {nodeControls && renderModeControls(runtimeNode)}

              {nodeControls && (
                <div className="flex flex-wrap gap-2 mt-4">
                  <button
                    onClick={() => handleAction(runtimeNode.addr, "enable-interception")}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                  >
                    <Play className="h-3 w-3" /> Enable Interception
                  </button>
                  <button
                    onClick={() => handleAction(runtimeNode.addr, "disable-interception")}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                  >
                    <Pause className="h-3 w-3" /> Disable Interception
                  </button>
                  <button
                    onClick={() => handleAction(runtimeNode.addr, "enable-firewall")}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                  >
                    <Shield className="h-3 w-3" /> Enable FW
                  </button>
                  <button
                    onClick={() => handleAction(runtimeNode.addr, "disable-firewall")}
                    className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                  >
                    <ShieldOff className="h-3 w-3" /> Disable FW
                  </button>
                </div>
              )}

              {managed && runtimeNode && renderTrustSection(
                runtimeNode,
                "Router-local trust list",
                "Applies only to router-local processes. Forwarded device traffic never enters the prompt flow in v1.",
              )}

              {renderStatusPill(router.addr)}
              {renderCapabilitySummary(router.addr)}
              {renderProvisionSteps(router.addr)}
            </div>
          );
        })}

        {genericNodes.map((node) => {
          const deleting = Boolean(deletingNodes[node.addr]);
          const canDelete =
            !node.online && node.source_type !== "router" && !node.router_managed;

          return (
            <div
              key={node.addr}
              className="bg-card border border-border rounded-xl p-4 md:p-5"
            >
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className={`p-2 rounded-lg ${node.online ? "bg-success/10" : "bg-muted"}`}>
                  <Server
                    className={`h-5 w-5 ${node.online ? "text-success" : "text-muted-foreground"}`}
                  />
                </div>
                <div>
                  <div className="font-medium">{node.hostname || node.addr}</div>
                  <div className="text-xs text-muted-foreground">{node.addr}</div>
                </div>
              </div>
              <div className="flex items-center gap-1.5">
                {canDelete && (
                  <button
                    onClick={() => handleDeleteNode(node)}
                    disabled={deleting}
                    className="flex items-center gap-1.5 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive hover:bg-destructive/20 disabled:opacity-50"
                    title="Delete node and its stored data"
                  >
                    {deleting ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Trash2 className="h-3.5 w-3.5" />
                    )}
                    Delete
                  </button>
                )}
                <span
                  className={`text-xs px-2 py-1 rounded-full ${
                    node.online ? "bg-success/10 text-success" : "bg-muted text-muted-foreground"
                  }`}
                >
                  {node.online ? "Online" : "Offline"}
                </span>
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mt-4 text-sm">
              <div>
                <div className="text-xs text-muted-foreground">Version</div>
                <div>{node.daemon_version || "-"}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Uptime</div>
                <div>{node.daemon_uptime ? formatUptime(node.daemon_uptime) : "-"}</div>
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

            {renderModeControls(node)}
            {renderStatusPill(node.addr)}
            {renderTagsSection(node)}

            {node.online && (
              <div className="flex flex-wrap gap-2 mt-4">
                <button
                  onClick={() => handleAction(node.addr, "enable-interception")}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Play className="h-3 w-3" /> Enable Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, "disable-interception")}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Pause className="h-3 w-3" /> Disable Interception
                </button>
                <button
                  onClick={() => handleAction(node.addr, "enable-firewall")}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <Shield className="h-3 w-3" /> Enable FW
                </button>
                <button
                  onClick={() => handleAction(node.addr, "disable-firewall")}
                  className="flex items-center gap-1.5 text-xs px-3 py-2 md:py-1.5 rounded-lg bg-muted hover:bg-muted/80 border border-border"
                >
                  <ShieldOff className="h-3 w-3" /> Disable FW
                </button>
              </div>
            )}

            {renderTrustSection(node, "Trust List")}
          </div>
          );
        })}

        {genericNodes.length === 0 && routerRuntimeNodes.length === 0 && (
          <div className="bg-card border border-border rounded-xl p-8 text-center text-muted-foreground">
            No nodes found. Configure an OpenSnitch daemon to connect to this
            server, or connect a router.
          </div>
        )}
      </div>

      {/* Connect Router BottomSheet */}
      <BottomSheet
        open={showConnectRouter}
        onClose={() => {
          if (!connecting) {
            setShowConnectRouter(false);
            setConnectError("");
            setConnectWarning("");
            setConnectSteps(null);
            setScanResults(null);
          }
        }}
        title="Connect Router"
        stickyFooter={
          !connectSteps && (
            <button
              onClick={handleConnectRouter}
              disabled={connecting || !canConnectRouter}
              className="w-full flex items-center justify-center gap-2 text-sm px-4 py-3 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {connecting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" /> Connecting...
                </>
              ) : (
                <>
                  <Router className="h-4 w-4" /> Connect
                </>
              )}
            </button>
          )
        }
      >
        <div className="p-5 space-y-4">
          {connectSteps ? (
            // Show provisioning steps
            <div className="space-y-3">
              {connectSteps.map((step, i) => (
                <div key={i} className="flex items-center gap-3">
                  {step.status === "done" ? (
                    <div className="p-1 rounded-full bg-success/10">
                      <Check className="h-4 w-4 text-success" />
                    </div>
                  ) : step.status === "warning" ? (
                    <div className="p-1 rounded-full bg-amber-500/10">
                      <AlertTriangle className="h-4 w-4 text-amber-500" />
                    </div>
                  ) : (
                    <div className="p-1 rounded-full bg-destructive/10">
                      <X className="h-4 w-4 text-destructive" />
                    </div>
                  )}
                  <div>
                    <div className="text-sm font-medium capitalize">{step.step}</div>
                    <div className={`text-xs ${step.status === "warning" ? "text-amber-500" : "text-muted-foreground"}`}>{step.message}</div>
                  </div>
                </div>
              ))}
              {connectSteps.every((s) => s.status === "done" || s.status === "warning") && (
                <div className="mt-4 text-center text-sm text-success">
                  Router connected successfully!
                </div>
              )}
              {connectWarning && (
                <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300">
                  {connectWarning}
                </div>
              )}
              {connectError && (
                <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {connectError}
                </div>
              )}
            </div>
          ) : (
            // Show form
            <>
              {/* Network scan */}
              <div className="rounded-xl border border-border bg-muted/30 p-4 space-y-3">
                <div className="flex items-center gap-2">
                  <Radar className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">Discover Routers</span>
                </div>
                <div className="flex gap-2">
                  <input
                    type="text"
                    placeholder="Subnet (auto-detect)"
                    value={scanSubnet}
                    onChange={(e) => setScanSubnet(e.target.value)}
                    className="flex-1 text-sm px-3 py-2 rounded-lg bg-card border border-border focus:outline-none focus:border-primary"
                  />
                  <button
                    onClick={handleScanNetwork}
                    disabled={scanning}
                    className="flex items-center gap-1.5 text-sm px-4 py-2 rounded-lg bg-primary/10 text-primary border border-primary/30 hover:bg-primary/20 disabled:opacity-50 transition-colors"
                  >
                    {scanning ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Radar className="h-3.5 w-3.5" />
                    )}
                    {scanning ? "Scanning..." : "Scan"}
                  </button>
                </div>
                {scanResults !== null && (
                  <div className="space-y-1.5">
                    {scanResults.length === 0 ? (
                      <p className="text-xs text-muted-foreground">No SSH devices found on this subnet.</p>
                    ) : (
                      <>
                        <p className="text-xs text-muted-foreground">{scanResults.length} device(s) found:</p>
                        {scanResults.map((device) => (
                          <button
                            key={device.ip}
                            onClick={() => selectDiscoveredRouter(device)}
                            className={`w-full text-left flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-colors ${
                              device.is_openwrt
                                ? "border-success/30 bg-success/5 hover:bg-success/10"
                                : "border-border bg-card hover:bg-muted/50"
                            }`}
                          >
                            <Wifi className={`h-4 w-4 shrink-0 ${device.is_openwrt ? "text-success" : "text-muted-foreground"}`} />
                            <div className="flex-1 min-w-0">
                              <div className="text-sm font-medium">{device.ip}</div>
                              <div className="text-xs text-muted-foreground truncate">{device.banner}</div>
                            </div>
                            {device.is_openwrt && (
                              <span className="text-xs px-2 py-0.5 rounded-full bg-success/10 text-success border border-success/30 shrink-0">
                                OpenWrt
                              </span>
                            )}
                          </button>
                        ))}
                      </>
                    )}
                  </div>
                )}
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">Router IP Address</label>
                <input
                  type="text"
                  placeholder="192.168.1.1"
                  value={routerForm.addr}
                  onChange={(e) => {
                    const addr = e.target.value;
                    setRouterForm((prev) => ({
                      ...prev,
                      addr,
                      lan_subnet: prev.lan_subnet || autoDetectSubnet(addr),
                    }));
                  }}
                  onBlur={() => {
                    if (routerForm.addr && !routerForm.lan_subnet) {
                      setRouterForm((prev) => ({
                        ...prev,
                        lan_subnet: autoDetectSubnet(prev.addr),
                      }));
                    }
                    if (routerForm.addr) {
                      api.suggestServerURL(routerForm.addr).then((res) => {
                        if (res.server_url) {
                          setRouterForm((prev) => ({
                            ...prev,
                            server_url: prev.server_url || res.server_url,
                          }));
                          setServerUrlSource(res.source);
                        }
                      }).catch(() => {});
                    }
                  }}
                  className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-muted-foreground block mb-1">SSH Port</label>
                  <input
                    type="number"
                    value={routerForm.ssh_port}
                    onChange={(e) =>
                      setRouterForm((prev) => ({ ...prev, ssh_port: parseInt(e.target.value) || 22 }))
                    }
                    className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                  />
                </div>
                <div>
                  <label className="text-xs text-muted-foreground block mb-1">SSH Username</label>
                  <input
                    type="text"
                    value={routerForm.ssh_user}
                    onChange={(e) =>
                      setRouterForm((prev) => ({ ...prev, ssh_user: e.target.value }))
                    }
                    className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                  />
                </div>
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">SSH Password</label>
                <input
                  type="password"
                  placeholder="Router password"
                  value={routerForm.ssh_pass}
                  onChange={(e) =>
                    setRouterForm((prev) => ({ ...prev, ssh_pass: e.target.value }))
                  }
                  className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">Router Name <span className="text-muted-foreground/60">(optional)</span></label>
                <input
                  type="text"
                  placeholder={routerForm.addr || "my-router"}
                  value={routerForm.name}
                  onChange={(e) =>
                    setRouterForm((prev) => ({ ...prev, name: e.target.value }))
                  }
                  className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                />
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-2">Router Mode</label>
                <div className="grid gap-2 sm:grid-cols-2">
                  {routerConnectModeOptions.map((option) => {
                    const selected = routerForm.mode === option.value;
                    return (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() =>
                          setRouterForm((prev) => ({ ...prev, mode: option.value }))
                        }
                        className={`rounded-xl border p-3 text-left transition-colors ${
                          selected
                            ? "border-primary bg-primary/10"
                            : "border-border bg-muted/30 hover:bg-muted/50"
                        }`}
                      >
                        <div className="flex items-center justify-between gap-3">
                          <div className="text-sm font-medium">{option.label}</div>
                          {selected && <Check className="h-4 w-4 text-primary" />}
                        </div>
                        <div className="mt-1 text-xs text-muted-foreground">
                          {option.description}
                        </div>
                      </button>
                    );
                  })}
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  Manage may fail on smaller routers. It needs enough RAM, storage, and a supported OpenWrt target. If that happens, the router stays connected in monitor mode.
                </p>
              </div>

              <div>
                <label className="text-xs text-muted-foreground block mb-1">LAN Subnet <span className="text-muted-foreground/60">(auto-detected)</span></label>
                <input
                  type="text"
                  placeholder="192.168.1.0/24"
                  value={routerForm.lan_subnet}
                  onChange={(e) =>
                    setRouterForm((prev) => ({ ...prev, lan_subnet: e.target.value }))
                  }
                  className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Only outbound traffic from this subnet to the internet will be tracked.
                </p>
              </div>

              <button
                type="button"
                onClick={() => setShowAdvanced((v) => !v)}
                className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                <ChevronDown className={`h-3 w-3 transition-transform ${showAdvanced ? "" : "-rotate-90"}`} />
                Advanced
              </button>

              {showAdvanced && (
                <div className="space-y-3">
                  <div>
                    <label className="text-xs text-muted-foreground block mb-1">
                      SSH Private Key <span className="text-muted-foreground/60">(optional)</span>
                    </label>
                    <textarea
                      placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                      value={routerForm.ssh_key}
                      onChange={(e) =>
                        setRouterForm((prev) => ({ ...prev, ssh_key: e.target.value }))
                      }
                      rows={3}
                      className="w-full text-xs font-mono px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary resize-y"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      Paste an SSH private key for key-based auth. Password is used as fallback.
                    </p>
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground block mb-1">
                      Server URL{" "}
                      {serverUrlSource === "lan_auto" && (
                        <span className="text-emerald-500">(auto-detected LAN)</span>
                      )}
                      {!serverUrlSource && (
                        <span className="text-muted-foreground/60">(auto-detected if empty)</span>
                      )}
                    </label>
                    <input
                      type="text"
                      placeholder="http://192.168.1.50:8080"
                      value={routerForm.server_url}
                      onChange={(e) => {
                        setRouterForm((prev) => ({ ...prev, server_url: e.target.value }));
                        setServerUrlSource(e.target.value ? "user_override" : "");
                      }}
                      className="w-full text-sm px-3 py-2.5 rounded-lg bg-muted border border-border focus:outline-none focus:border-primary"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      URL the router agent will POST connection data to. Leave empty to auto-detect.
                    </p>
                  </div>
                </div>
              )}

              {connectError && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                  {connectError}
                </div>
              )}
              {connectWarning && (
                <div className="text-sm text-amber-700 dark:text-amber-300 bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2">
                  {connectWarning}
                </div>
              )}
            </>
          )}
        </div>
      </BottomSheet>
    </div>
  );
}
