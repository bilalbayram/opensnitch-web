import { useEffect, useMemo, useState } from "react";
import type { NodeRecord, RuleOperator, RuleRecord } from "@/lib/api";
import { api } from "@/lib/api";
import { actionColor } from "@/lib/utils";
import { Pencil, Plus, Trash2, ToggleLeft, ToggleRight } from "lucide-react";
import { ResponsiveDataView } from "@/components/ui/responsive-data-view";
import { BottomSheet } from "@/components/ui/bottom-sheet";

interface RuleForm {
  name: string;
  node: string;
  enabled: boolean;
  precedence: boolean;
  action: string;
  duration: string;
  operator_type: string;
  operator_sensitive: boolean;
  operator_operand: string;
  operator_data: string;
  description: string;
  nolog: boolean;
}

interface GeneratedRulePreview {
  fingerprint: string;
  process: string;
  destination: string;
  destination_operand: string;
  dst_port: number;
  protocol: string;
  hits: number;
  first_seen: string;
  last_seen: string;
  rule: RuleRecord;
}

interface GeneratedRulesRequest {
  node: string;
  since: string;
  until: string;
  exclude_processes: string[];
}

type RangePreset = "24h" | "48h" | "7d" | "custom";

const defaultForm: RuleForm = {
  name: "",
  node: "",
  enabled: true,
  precedence: false,
  action: "deny",
  duration: "always",
  operator_type: "simple",
  operator_sensitive: false,
  operator_operand: "process.path",
  operator_data: "",
  description: "",
  nolog: false,
};

const operandLabels: Record<string, string> = {
  "process.path": "Process",
  "process.command": "Command",
  "dest.host": "Host",
  "dest.ip": "IP",
  "dest.port": "Port",
  "user.id": "User",
  protocol: "Protocol",
};

const modeLabels: Record<string, string> = {
  ask: "Ask",
  silent_allow: "Silent Allow",
  silent_deny: "Silent Deny",
};

function pad(value: number) {
  return value.toString().padStart(2, "0");
}

function formatDateTimeInput(date: Date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function getPresetRange(preset: Exclude<RangePreset, "custom">) {
  const until = new Date();
  const since = new Date(until);

  if (preset === "24h") {
    since.setHours(since.getHours() - 24);
  } else if (preset === "48h") {
    since.setHours(since.getHours() - 48);
  } else {
    since.setDate(since.getDate() - 7);
  }

  return { since, until };
}

function flattenOperators(operator?: RuleOperator): RuleOperator[] {
  if (!operator) return [];
  if (operator.type === "list" && operator.list?.length) {
    return operator.list.flatMap(flattenOperators);
  }
  return [operator];
}

function formatOperator(operator: RuleOperator) {
  const label =
    operandLabels[operator.operand || ""] || operator.operand || "Match";
  const value =
    operator.operand === "protocol"
      ? operator.data?.toUpperCase() || ""
      : operator.data || "";
  return `${label}: ${value}`;
}

function formatRuleMatch(rule: RuleRecord) {
  const operators = flattenOperators(rule.operator);
  if (operators.length > 0) {
    return operators.map(formatOperator).join(" • ");
  }
  if (!rule.operator_operand && !rule.operator_data) {
    return "-";
  }
  return formatOperator({
    operand: rule.operator_operand,
    data: rule.operator_data,
  });
}

function parseExcludeProcesses(value: string) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export default function RulesPage() {
  const [rules, setRules] = useState<RuleRecord[]>([]);
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [selectedNode, setSelectedNode] = useState("");
  const [showEditor, setShowEditor] = useState(false);
  const [showGenerator, setShowGenerator] = useState(false);
  const [form, setForm] = useState<RuleForm>(defaultForm);
  const [editing, setEditing] = useState(false);
  const [rangePreset, setRangePreset] = useState<RangePreset>("48h");
  const [customSince, setCustomSince] = useState("");
  const [customUntil, setCustomUntil] = useState("");
  const [excludeProcesses, setExcludeProcesses] = useState("");
  const [preview, setPreview] = useState<GeneratedRulePreview[]>([]);
  const [selectedFingerprints, setSelectedFingerprints] = useState<string[]>(
    [],
  );
  const [previewSkippedExisting, setPreviewSkippedExisting] = useState(0);
  const [previewSkippedExcluded, setPreviewSkippedExcluded] = useState(0);
  const [previewRequest, setPreviewRequest] =
    useState<GeneratedRulesRequest | null>(null);
  const [generatorError, setGeneratorError] = useState("");
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [applyingPreview, setApplyingPreview] = useState(false);

  const selectedNodeInfo = useMemo(
    () => nodes.find((node) => node.addr === selectedNode),
    [nodes, selectedNode],
  );

  const fetchNodes = () => {
    api.getNodes().then(setNodes).catch(console.error);
  };

  const fetchRules = () => {
    api
      .getRules(selectedNode || undefined)
      .then(setRules)
      .catch(console.error);
  };

  const resetPreviewState = () => {
    setPreview([]);
    setSelectedFingerprints([]);
    setPreviewSkippedExisting(0);
    setPreviewSkippedExcluded(0);
    setPreviewRequest(null);
  };

  useEffect(() => {
    fetchNodes();
  }, []);

  useEffect(() => {
    api
      .getRules(selectedNode || undefined)
      .then(setRules)
      .catch(console.error);
  }, [selectedNode]);

  useEffect(() => {
    if (!showGenerator) return;
    resetPreviewState();
    setGeneratorError("");
  }, [
    showGenerator,
    selectedNode,
    rangePreset,
    customSince,
    customUntil,
    excludeProcesses,
  ]);

  const openCreateRule = () => {
    setForm({ ...defaultForm, node: selectedNode });
    setEditing(false);
    setShowEditor(true);
  };

  const handleSave = async () => {
    try {
      if (editing) {
        await api.updateRule(form.name, form);
      } else {
        await api.createRule(form);
      }
      setShowEditor(false);
      setForm({ ...defaultForm, node: selectedNode });
      setEditing(false);
      fetchRules();
    } catch (e) {
      console.error("Failed to save rule:", e);
    }
  };

  const handleEdit = (rule: RuleRecord) => {
    if (rule.is_compound || rule.source_kind === "managed") return;
    setForm({
      name: rule.name,
      node: rule.node,
      enabled: rule.enabled,
      precedence: rule.precedence,
      action: rule.action,
      duration: rule.duration,
      operator_type: rule.operator_type || "simple",
      operator_sensitive: rule.operator_sensitive,
      operator_operand: rule.operator_operand,
      operator_data: rule.operator_data,
      description: rule.description,
      nolog: rule.nolog,
    });
    setEditing(true);
    setShowEditor(true);
  };

  const handleDelete = async (name: string, node: string) => {
    if (!confirm(`Delete rule "${name}"?`)) return;
    await api.deleteRule(name, node);
    fetchRules();
  };

  const handleToggle = async (name: string, node: string, enabled: boolean) => {
    if (enabled) {
      await api.disableRule(name, node);
    } else {
      await api.enableRule(name, node);
    }
    fetchRules();
  };

  const openGenerator = () => {
    const range = getPresetRange("48h");
    setRangePreset("48h");
    setCustomSince(formatDateTimeInput(range.since));
    setCustomUntil(formatDateTimeInput(range.until));
    setExcludeProcesses("");
    resetPreviewState();
    setGeneratorError("");
    setShowGenerator(true);
  };

  const setPreset = (preset: RangePreset) => {
    setRangePreset(preset);
    if (preset !== "custom") {
      const range = getPresetRange(preset);
      setCustomSince(formatDateTimeInput(range.since));
      setCustomUntil(formatDateTimeInput(range.until));
    }
  };

  const buildGeneratorRequest = (): GeneratedRulesRequest | null => {
    if (!selectedNode) {
      setGeneratorError("Select a node before generating rules.");
      return null;
    }

    let since: Date;
    let until: Date;

    if (rangePreset === "custom") {
      if (!customSince || !customUntil) {
        setGeneratorError("Custom ranges require both start and end times.");
        return null;
      }
      since = new Date(customSince);
      until = new Date(customUntil);
    } else {
      const range = getPresetRange(rangePreset);
      since = range.since;
      until = range.until;
    }

    if (Number.isNaN(since.getTime()) || Number.isNaN(until.getTime())) {
      setGeneratorError("Invalid date range.");
      return null;
    }

    if (until < since) {
      setGeneratorError("End time must be after start time.");
      return null;
    }

    return {
      node: selectedNode,
      since: since.toISOString(),
      until: until.toISOString(),
      exclude_processes: parseExcludeProcesses(excludeProcesses),
    };
  };

  const handlePreview = async () => {
    const payload = buildGeneratorRequest();
    if (!payload) return;

    setLoadingPreview(true);
    setGeneratorError("");
    try {
      const res = await api.previewGeneratedRules(payload);
      const items = (res.data || []) as unknown as GeneratedRulePreview[];
      setPreview(items);
      setSelectedFingerprints(items.map((item) => item.fingerprint));
      setPreviewSkippedExisting(res.skipped_existing || 0);
      setPreviewSkippedExcluded(res.skipped_excluded || 0);
      setPreviewRequest(payload);
    } catch (e) {
      console.error("Failed to preview generated rules:", e);
      resetPreviewState();
      setGeneratorError(
        e instanceof Error ? e.message : "Failed to preview rules",
      );
    } finally {
      setLoadingPreview(false);
    }
  };

  const handleApply = async () => {
    if (!previewRequest) {
      setGeneratorError("Run preview again before applying rules.");
      return;
    }
    if (selectedFingerprints.length === 0) {
      setGeneratorError("Select at least one rule to apply.");
      return;
    }

    setApplyingPreview(true);
    setGeneratorError("");
    try {
      await api.applyGeneratedRules({
        ...previewRequest,
        fingerprints: selectedFingerprints,
      });
      resetPreviewState();
      setShowGenerator(false);
      fetchRules();
      fetchNodes();
    } catch (e) {
      console.error("Failed to apply generated rules:", e);
      setGeneratorError(
        e instanceof Error ? e.message : "Failed to apply rules",
      );
    } finally {
      setApplyingPreview(false);
    }
  };

  const toggleFingerprint = (fingerprint: string) => {
    setSelectedFingerprints((current) =>
      current.includes(fingerprint)
        ? current.filter((item) => item !== fingerprint)
        : [...current, fingerprint],
    );
  };

  const allSelected =
    preview.length > 0 && selectedFingerprints.length === preview.length;

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">Rules</h1>
          {selectedNodeInfo && (
            <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span
                className={`rounded-full px-2 py-0.5 ${selectedNodeInfo.online ? "bg-success/10 text-success" : "bg-muted text-muted-foreground"}`}
              >
                {selectedNodeInfo.online ? "Online" : "Offline"}
              </span>
              <span className="rounded-full bg-primary/10 px-2 py-0.5 text-primary">
                Mode:{" "}
                {modeLabels[selectedNodeInfo.mode] || selectedNodeInfo.mode}
              </span>
              <span>{selectedNodeInfo.hostname || selectedNodeInfo.addr}</span>
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <select
            value={selectedNode}
            onChange={(e) => setSelectedNode(e.target.value)}
            className="bg-card border border-border rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All nodes</option>
            {nodes.map((node) => (
              <option key={node.addr} value={node.addr}>
                {node.hostname || node.addr}
              </option>
            ))}
          </select>

          <button
            onClick={openGenerator}
            disabled={!selectedNode}
            title={
              selectedNode
                ? "Generate rules from observed traffic"
                : "Select a node first"
            }
            className="rounded-lg border border-border bg-muted px-3 py-2 text-sm font-medium hover:bg-muted/80 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <span className="hidden sm:inline">
              Generate Rules from History
            </span>
            <span className="sm:hidden">Generate</span>
          </button>

          <button
            onClick={openCreateRule}
            className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-lg px-3 py-2 text-sm font-medium hover:bg-primary/80"
          >
            <Plus className="h-4 w-4" />{" "}
            <span className="hidden sm:inline">New Rule</span>
            <span className="sm:hidden">New</span>
          </button>
        </div>
      </div>

      {/* Rules data */}
      <ResponsiveDataView
        data={rules}
        columns={7}
        emptyMessage="No rules"
        tableHead={
          <tr className="border-b border-border text-left text-xs text-muted-foreground">
            <th className="px-4 py-2">Name</th>
            <th className="px-4 py-2">Action</th>
            <th className="px-4 py-2">Duration</th>
            <th className="px-4 py-2">Match</th>
            <th className="px-4 py-2">Node</th>
            <th className="px-4 py-2">Enabled</th>
            <th className="px-4 py-2">Actions</th>
          </tr>
        }
        renderRow={(rule: RuleRecord) => {
          const matchSummary = formatRuleMatch(rule);
          const isManaged = rule.source_kind === "managed";
          const displayName = rule.display_name || rule.name;
          return (
            <tr
              key={`${rule.node}-${rule.name}`}
              className="border-b border-border/50 hover:bg-muted/50"
            >
              <td className="px-4 py-2">
                <div className="font-medium">{displayName}</div>
                {isManaged && (
                  <div className="text-xs text-primary">
                    Managed by{" "}
                    {rule.template_name || `Template #${rule.template_id}`}
                  </div>
                )}
                {!isManaged && rule.is_compound && (
                  <div className="text-xs text-primary">
                    Generated compound rule
                  </div>
                )}
                {isManaged && (
                  <div className="text-xs text-muted-foreground">
                    {rule.name}
                  </div>
                )}
              </td>
              <td className={`px-4 py-2 ${actionColor(rule.action)}`}>
                {rule.action}
              </td>
              <td className="px-4 py-2 text-xs">{rule.duration}</td>
              <td
                className="px-4 py-2 text-xs max-w-md truncate"
                title={matchSummary}
              >
                {matchSummary}
              </td>
              <td className="px-4 py-2 text-xs text-muted-foreground">
                {rule.node || "All nodes"}
              </td>
              <td className="px-4 py-2">
                <button
                  onClick={() =>
                    handleToggle(rule.name, rule.node, rule.enabled)
                  }
                  disabled={isManaged}
                  title={
                    isManaged
                      ? "Managed rules must be changed from the Templates page"
                      : "Toggle rule"
                  }
                  className="disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {rule.enabled ? (
                    <ToggleRight className="h-5 w-5 text-success" />
                  ) : (
                    <ToggleLeft className="h-5 w-5 text-muted-foreground" />
                  )}
                </button>
              </td>
              <td className="px-4 py-2">
                <div className="flex gap-1">
                  <button
                    onClick={() => handleEdit(rule)}
                    disabled={rule.is_compound || isManaged}
                    title={
                      isManaged
                        ? "Managed rules must be edited from the Templates page"
                        : rule.is_compound
                          ? "Compound rule editing is not available yet"
                          : "Edit rule"
                    }
                    className="p-1 hover:bg-muted rounded disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </button>
                  <button
                    onClick={() => handleDelete(rule.name, rule.node)}
                    disabled={isManaged}
                    title={
                      isManaged
                        ? "Managed rules must be deleted from the Templates page"
                        : "Delete rule"
                    }
                    className="p-1 hover:bg-muted rounded text-destructive disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          );
        }}
        renderCard={(rule: RuleRecord) => {
          const matchSummary = formatRuleMatch(rule);
          const isManaged = rule.source_kind === "managed";
          const displayName = rule.display_name || rule.name;
          return (
            <div
              key={`${rule.node}-${rule.name}`}
              className="bg-card border border-border rounded-xl p-3 space-y-2"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                  <div className="font-medium text-sm truncate">
                    {displayName}
                  </div>
                  {isManaged && (
                    <div className="text-xs text-primary">
                      Managed by{" "}
                      {rule.template_name || `Template #${rule.template_id}`}
                    </div>
                  )}
                  {!isManaged && rule.is_compound && (
                    <div className="text-xs text-primary">Compound rule</div>
                  )}
                  {isManaged && (
                    <div className="text-xs text-muted-foreground truncate">
                      {rule.name}
                    </div>
                  )}
                </div>
                <button
                  onClick={() =>
                    handleToggle(rule.name, rule.node, rule.enabled)
                  }
                  disabled={isManaged}
                  className="shrink-0 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {rule.enabled ? (
                    <ToggleRight className="h-6 w-6 text-success" />
                  ) : (
                    <ToggleLeft className="h-6 w-6 text-muted-foreground" />
                  )}
                </button>
              </div>
              <div className="flex items-center gap-2 flex-wrap">
                <span
                  className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
                    rule.action === "allow"
                      ? "bg-success/15 text-success"
                      : rule.action === "deny"
                        ? "bg-destructive/15 text-destructive"
                        : "bg-warning/15 text-warning"
                  }`}
                >
                  {rule.action}
                </span>
                <span className="text-xs text-muted-foreground">
                  {rule.duration}
                </span>
                <span className="text-xs text-muted-foreground">
                  · {rule.node || "All nodes"}
                </span>
              </div>
              <div className="text-xs text-muted-foreground break-all">
                {matchSummary}
              </div>
              <div className="flex justify-end gap-2 pt-1">
                <button
                  onClick={() => handleEdit(rule)}
                  disabled={rule.is_compound || isManaged}
                  className="flex items-center gap-1 text-xs px-3 py-1.5 rounded-lg bg-muted border border-border hover:bg-muted/80 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <Pencil className="h-3 w-3" /> Edit
                </button>
                <button
                  onClick={() => handleDelete(rule.name, rule.node)}
                  disabled={isManaged}
                  className="flex items-center gap-1 text-xs px-3 py-1.5 rounded-lg text-destructive bg-destructive/10 border border-destructive/20 hover:bg-destructive/20 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <Trash2 className="h-3 w-3" /> Delete
                </button>
              </div>
            </div>
          );
        }}
      />

      {/* Rule Editor */}
      <BottomSheet
        open={showEditor}
        onClose={() => setShowEditor(false)}
        title={editing ? "Edit Rule" : "New Rule"}
        stickyFooter={
          <div className="flex gap-3">
            <button
              onClick={() => setShowEditor(false)}
              className="flex-1 bg-muted border border-border rounded-lg py-2.5 text-sm hover:bg-muted/80"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="flex-1 bg-primary text-primary-foreground rounded-lg py-2.5 text-sm font-medium hover:bg-primary/80"
            >
              {editing ? "Save" : "Create"}
            </button>
          </div>
        }
      >
        <div className="px-5 py-4 space-y-4">
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            Target node: {form.node || "All nodes"}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-muted-foreground">Name</label>
              <input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                disabled={editing}
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Action</label>
              <select
                value={form.action}
                onChange={(e) => setForm({ ...form, action: e.target.value })}
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              >
                <option value="allow">Allow</option>
                <option value="deny">Deny</option>
                <option value="reject">Reject</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Duration</label>
              <select
                value={form.duration}
                onChange={(e) => setForm({ ...form, duration: e.target.value })}
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              >
                <option value="once">Once</option>
                <option value="5m">5 minutes</option>
                <option value="15m">15 minutes</option>
                <option value="30m">30 minutes</option>
                <option value="1h">1 hour</option>
                <option value="until restart">Until restart</option>
                <option value="always">Always</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Operand</label>
              <select
                value={form.operator_operand}
                onChange={(e) =>
                  setForm({ ...form, operator_operand: e.target.value })
                }
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              >
                <option value="process.path">Process Path</option>
                <option value="process.command">Process Command</option>
                <option value="dest.host">Dest Host</option>
                <option value="dest.ip">Dest IP</option>
                <option value="dest.port">Dest Port</option>
                <option value="user.id">User ID</option>
                <option value="protocol">Protocol</option>
              </select>
            </div>
          </div>

          <div>
            <label className="text-xs text-muted-foreground">Data</label>
            <input
              value={form.operator_data}
              onChange={(e) =>
                setForm({ ...form, operator_data: e.target.value })
              }
              placeholder="e.g. /usr/bin/curl or google.com"
              className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
            />
          </div>

          <div>
            <label className="text-xs text-muted-foreground">Description</label>
            <input
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
            />
          </div>
        </div>
      </BottomSheet>

      {/* Rule Generator */}
      <BottomSheet
        open={showGenerator}
        onClose={() => setShowGenerator(false)}
        title="Generate Rules from History"
        fullScreen
        stickyFooter={
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
            <div className="text-sm text-muted-foreground">
              {preview.length > 0
                ? `${selectedFingerprints.length} selected${allSelected ? " · all proposed rules selected" : ""}`
                : "Preview rules to continue."}
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => setShowGenerator(false)}
                className="flex-1 sm:flex-none rounded-lg border border-border bg-muted px-4 py-2.5 text-sm hover:bg-muted/80"
              >
                Cancel
              </button>
              <button
                onClick={handleApply}
                disabled={applyingPreview || selectedFingerprints.length === 0}
                className="flex-1 sm:flex-none rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/80 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {applyingPreview
                  ? "Applying..."
                  : `Apply ${selectedFingerprints.length} Rules`}
              </button>
            </div>
          </div>
        }
      >
        <div className="p-5 space-y-4">
          <div className="text-sm text-muted-foreground">
            {selectedNodeInfo?.hostname ||
              selectedNodeInfo?.addr ||
              selectedNode}
          </div>

          <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
            <div className="space-y-4">
              <div>
                <label className="text-xs text-muted-foreground">
                  Time range
                </label>
                <div className="mt-1 flex flex-wrap gap-2">
                  {(["24h", "48h", "7d", "custom"] as RangePreset[]).map(
                    (preset) => (
                      <button
                        key={preset}
                        onClick={() => setPreset(preset)}
                        className={`rounded-lg border px-3 py-2 text-sm transition-colors ${
                          rangePreset === preset
                            ? "bg-primary/10 text-primary border-primary/30"
                            : "bg-muted border-border hover:bg-muted/80"
                        }`}
                      >
                        {preset === "custom" ? "Custom" : `Last ${preset}`}
                      </button>
                    ),
                  )}
                </div>
              </div>

              {rangePreset === "custom" && (
                <div className="grid gap-3 grid-cols-1 sm:grid-cols-2">
                  <div>
                    <label className="text-xs text-muted-foreground">
                      Start
                    </label>
                    <input
                      type="datetime-local"
                      value={customSince}
                      onChange={(e) => setCustomSince(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">End</label>
                    <input
                      type="datetime-local"
                      value={customUntil}
                      onChange={(e) => setCustomUntil(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                    />
                  </div>
                </div>
              )}

              <div>
                <label className="text-xs text-muted-foreground">
                  Exclude processes
                </label>
                <textarea
                  value={excludeProcesses}
                  onChange={(e) => setExcludeProcesses(e.target.value)}
                  placeholder="/usr/bin/bash, /usr/bin/ssh"
                  rows={3}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
                <div className="mt-1 text-xs text-muted-foreground">
                  Separate process paths with commas or new lines.
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-border bg-muted/30 p-4 space-y-3">
              <div className="text-sm font-medium">Preview status</div>
              <div className="text-sm text-muted-foreground">
                {preview.length > 0
                  ? `${selectedFingerprints.length} of ${preview.length} rules selected`
                  : "Run a preview to inspect proposed rules before applying."}
              </div>
              <div className="grid grid-cols-2 gap-3 text-sm">
                <div className="rounded-lg border border-border bg-card px-3 py-2">
                  <div className="text-xs text-muted-foreground">
                    Skipped existing
                  </div>
                  <div className="font-medium">{previewSkippedExisting}</div>
                </div>
                <div className="rounded-lg border border-border bg-card px-3 py-2">
                  <div className="text-xs text-muted-foreground">
                    Skipped excluded
                  </div>
                  <div className="font-medium">{previewSkippedExcluded}</div>
                </div>
              </div>
              {generatorError && (
                <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {generatorError}
                </div>
              )}
              <button
                onClick={handlePreview}
                disabled={loadingPreview}
                className="w-full rounded-lg bg-primary px-3 py-2.5 text-sm font-medium text-primary-foreground hover:bg-primary/80 disabled:opacity-50"
              >
                {loadingPreview ? "Generating Preview..." : "Preview Rules"}
              </button>
            </div>
          </div>

          {/* Preview table */}
          <div>
            <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
              <div className="text-sm font-medium">Proposed Rules</div>
              {preview.length > 0 && (
                <div className="flex gap-2">
                  <button
                    onClick={() =>
                      setSelectedFingerprints(
                        preview.map((item) => item.fingerprint),
                      )
                    }
                    className="rounded-lg border border-border bg-muted px-3 py-1.5 text-xs hover:bg-muted/80"
                  >
                    Select all
                  </button>
                  <button
                    onClick={() => setSelectedFingerprints([])}
                    className="rounded-lg border border-border bg-muted px-3 py-1.5 text-xs hover:bg-muted/80"
                  >
                    Clear
                  </button>
                </div>
              )}
            </div>

            <ResponsiveDataView
              data={preview}
              columns={9}
              emptyMessage="No preview generated yet."
              tableHead={
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="px-4 py-2">Select</th>
                  <th className="px-4 py-2">Process</th>
                  <th className="px-4 py-2">Destination</th>
                  <th className="px-4 py-2">Port</th>
                  <th className="px-4 py-2">Protocol</th>
                  <th className="px-4 py-2">Hits</th>
                  <th className="px-4 py-2">First Seen</th>
                  <th className="px-4 py-2">Last Seen</th>
                  <th className="px-4 py-2">Rule Name</th>
                </tr>
              }
              renderRow={(item: GeneratedRulePreview) => {
                const selected = selectedFingerprints.includes(
                  item.fingerprint,
                );
                return (
                  <tr
                    key={item.fingerprint}
                    className="border-b border-border/50 hover:bg-muted/50"
                  >
                    <td className="px-4 py-2">
                      <input
                        type="checkbox"
                        checked={selected}
                        onChange={() => toggleFingerprint(item.fingerprint)}
                        className="h-4 w-4 rounded border-border bg-muted"
                      />
                    </td>
                    <td
                      className="px-4 py-2 font-mono text-xs max-w-48 truncate"
                      title={item.process}
                    >
                      {item.process}
                    </td>
                    <td className="px-4 py-2 text-xs">{item.destination}</td>
                    <td className="px-4 py-2 text-xs">{item.dst_port}</td>
                    <td className="px-4 py-2 text-xs uppercase">
                      {item.protocol}
                    </td>
                    <td className="px-4 py-2 text-xs">{item.hits}</td>
                    <td className="px-4 py-2 text-xs whitespace-nowrap">
                      {item.first_seen}
                    </td>
                    <td className="px-4 py-2 text-xs whitespace-nowrap">
                      {item.last_seen}
                    </td>
                    <td className="px-4 py-2 text-xs">{item.rule.name}</td>
                  </tr>
                );
              }}
              renderCard={(item: GeneratedRulePreview) => {
                const selected = selectedFingerprints.includes(
                  item.fingerprint,
                );
                return (
                  <div
                    key={item.fingerprint}
                    onClick={() => toggleFingerprint(item.fingerprint)}
                    className={`bg-card border rounded-xl p-3 space-y-1.5 cursor-pointer transition-colors ${
                      selected
                        ? "border-primary/50 bg-primary/5"
                        : "border-border"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <input
                        type="checkbox"
                        checked={selected}
                        onChange={() => toggleFingerprint(item.fingerprint)}
                        className="h-4 w-4 rounded border-border bg-muted"
                      />
                      <span className="text-xs text-muted-foreground">
                        {item.hits} hits
                      </span>
                    </div>
                    <div className="font-mono text-xs break-all">
                      {item.process}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      → {item.destination}:{item.dst_port}{" "}
                      <span className="uppercase">{item.protocol}</span>
                    </div>
                    <div className="text-[10px] text-muted-foreground">
                      {item.rule.name}
                    </div>
                  </div>
                );
              }}
            />
          </div>
        </div>
      </BottomSheet>
    </div>
  );
}
