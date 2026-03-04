import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { actionColor } from '@/lib/utils';
import { Plus, Pencil, Trash2, ToggleLeft, ToggleRight } from 'lucide-react';

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

const defaultForm: RuleForm = {
  name: '', node: '', enabled: true, precedence: false,
  action: 'deny', duration: 'always', operator_type: 'simple',
  operator_sensitive: false, operator_operand: 'process.path',
  operator_data: '', description: '', nolog: false,
};

export default function RulesPage() {
  const [rules, setRules] = useState<any[]>([]);
  const [showEditor, setShowEditor] = useState(false);
  const [form, setForm] = useState<RuleForm>(defaultForm);
  const [editing, setEditing] = useState(false);

  const fetchRules = () => {
    api.getRules().then(setRules).catch(console.error);
  };

  useEffect(() => { fetchRules(); }, []);

  const handleSave = async () => {
    try {
      if (editing) {
        await api.updateRule(form.name, form);
      } else {
        await api.createRule(form);
      }
      setShowEditor(false);
      setForm(defaultForm);
      setEditing(false);
      fetchRules();
    } catch (e) {
      console.error('Failed to save rule:', e);
    }
  };

  const handleEdit = (rule: any) => {
    setForm({
      name: rule.name,
      node: rule.node,
      enabled: rule.enabled,
      precedence: rule.precedence,
      action: rule.action,
      duration: rule.duration,
      operator_type: rule.operator_type,
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

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold">Rules</h1>
        <button
          onClick={() => { setForm(defaultForm); setEditing(false); setShowEditor(true); }}
          className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-lg px-3 py-1.5 text-sm font-medium hover:bg-primary/80"
        >
          <Plus className="h-4 w-4" /> New Rule
        </button>
      </div>

      {/* Rules table */}
      <div className="bg-card border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-xs text-muted-foreground">
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Duration</th>
                <th className="px-4 py-2">Operand</th>
                <th className="px-4 py-2">Data</th>
                <th className="px-4 py-2">Node</th>
                <th className="px-4 py-2">Enabled</th>
                <th className="px-4 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={`${r.node}-${r.name}`} className="border-b border-border/50 hover:bg-muted/50">
                  <td className="px-4 py-2 font-medium">{r.name}</td>
                  <td className={`px-4 py-2 ${actionColor(r.action)}`}>{r.action}</td>
                  <td className="px-4 py-2 text-xs">{r.duration}</td>
                  <td className="px-4 py-2 text-xs">{r.operator_operand}</td>
                  <td className="px-4 py-2 font-mono text-xs max-w-48 truncate" title={r.operator_data}>{r.operator_data}</td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{r.node}</td>
                  <td className="px-4 py-2">
                    <button onClick={() => handleToggle(r.name, r.node, r.enabled)}>
                      {r.enabled
                        ? <ToggleRight className="h-5 w-5 text-success" />
                        : <ToggleLeft className="h-5 w-5 text-muted-foreground" />}
                    </button>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex gap-1">
                      <button onClick={() => handleEdit(r)} className="p-1 hover:bg-muted rounded">
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      <button onClick={() => handleDelete(r.name, r.node)} className="p-1 hover:bg-muted rounded text-destructive">
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">No rules</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Rule Editor Modal */}
      {showEditor && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-card border border-border rounded-xl w-full max-w-lg p-5 space-y-4">
            <h2 className="text-lg font-semibold">{editing ? 'Edit Rule' : 'New Rule'}</h2>

            <div className="grid grid-cols-2 gap-3">
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
                  onChange={(e) => setForm({ ...form, operator_operand: e.target.value })}
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
                onChange={(e) => setForm({ ...form, operator_data: e.target.value })}
                placeholder="e.g. /usr/bin/curl or google.com"
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              />
            </div>

            <div>
              <label className="text-xs text-muted-foreground">Description</label>
              <input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm mt-1"
              />
            </div>

            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setShowEditor(false)}
                className="flex-1 bg-muted border border-border rounded-lg py-2 text-sm hover:bg-muted/80"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="flex-1 bg-primary text-primary-foreground rounded-lg py-2 text-sm font-medium hover:bg-primary/80"
              >
                {editing ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
