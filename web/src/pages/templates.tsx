import { useEffect, useMemo, useState } from 'react';
import type {
  NodeRecord,
  TemplateAttachmentRecord,
  TemplateRecord,
  TemplateRuleRecord,
} from '@/lib/api';
import { api } from '@/lib/api';
import { Pencil, Plus, Server, Shield, Tag, Trash2 } from 'lucide-react';

interface TemplateForm {
  name: string;
  description: string;
}

interface TemplateRuleForm {
  name: string;
  position: number;
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

interface AttachmentForm {
  target_type: 'node' | 'tag';
  target_ref: string;
  priority: number;
}

const defaultTemplateForm: TemplateForm = {
  name: '',
  description: '',
};

const defaultTemplateRuleForm: TemplateRuleForm = {
  name: '',
  position: 0,
  enabled: true,
  precedence: false,
  action: 'allow',
  duration: 'always',
  operator_type: 'simple',
  operator_sensitive: false,
  operator_operand: 'process.path',
  operator_data: '',
  description: '',
  nolog: false,
};

const defaultAttachmentForm: AttachmentForm = {
  target_type: 'node',
  target_ref: '',
  priority: 100,
};

function attachmentLabel(attachment: TemplateAttachmentRecord) {
  return attachment.target_type === 'node'
    ? `Node: ${attachment.target_ref}`
    : `Tag: ${attachment.target_ref}`;
}

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<TemplateRecord[]>([]);
  const [nodes, setNodes] = useState<NodeRecord[]>([]);
  const [error, setError] = useState('');

  const [showTemplateEditor, setShowTemplateEditor] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<TemplateRecord | null>(null);
  const [templateForm, setTemplateForm] = useState<TemplateForm>(defaultTemplateForm);

  const [showRuleEditor, setShowRuleEditor] = useState(false);
  const [activeTemplateId, setActiveTemplateId] = useState<number | null>(null);
  const [editingRule, setEditingRule] = useState<TemplateRuleRecord | null>(null);
  const [ruleForm, setRuleForm] = useState<TemplateRuleForm>(defaultTemplateRuleForm);

  const [showAttachmentEditor, setShowAttachmentEditor] = useState(false);
  const [editingAttachment, setEditingAttachment] = useState<TemplateAttachmentRecord | null>(null);
  const [attachmentForm, setAttachmentForm] = useState<AttachmentForm>(defaultAttachmentForm);

  const knownTags = useMemo(() => (
    Array.from(new Set(nodes.flatMap((node) => node.tags || []))).sort()
  ), [nodes]);

  const fetchData = () => {
    Promise.all([api.getTemplates(), api.getNodes()])
      .then(([templateData, nodeData]) => {
        setTemplates(templateData);
        setNodes(nodeData);
      })
      .catch((err) => {
        console.error('Failed to load templates:', err);
        setError(err instanceof Error ? err.message : 'Failed to load templates');
      });
  };

  useEffect(() => {
    fetchData();
  }, []);

  const openCreateTemplate = () => {
    setEditingTemplate(null);
    setTemplateForm(defaultTemplateForm);
    setShowTemplateEditor(true);
  };

  const openEditTemplate = (template: TemplateRecord) => {
    setEditingTemplate(template);
    setTemplateForm({
      name: template.name,
      description: template.description,
    });
    setShowTemplateEditor(true);
  };

  const saveTemplate = async () => {
    try {
      if (editingTemplate) {
        await api.updateTemplate(editingTemplate.id, templateForm);
      } else {
        await api.createTemplate(templateForm);
      }
      setShowTemplateEditor(false);
      setEditingTemplate(null);
      setTemplateForm(defaultTemplateForm);
      setError('');
      fetchData();
    } catch (err) {
      console.error('Failed to save template:', err);
      setError(err instanceof Error ? err.message : 'Failed to save template');
    }
  };

  const deleteTemplate = async (template: TemplateRecord) => {
    if (!confirm(`Delete template "${template.name}"?`)) return;
    try {
      await api.deleteTemplate(template.id);
      fetchData();
    } catch (err) {
      console.error('Failed to delete template:', err);
      setError(err instanceof Error ? err.message : 'Failed to delete template');
    }
  };

  const openCreateRule = (templateId: number) => {
    setActiveTemplateId(templateId);
    setEditingRule(null);
    setRuleForm(defaultTemplateRuleForm);
    setShowRuleEditor(true);
  };

  const openEditRule = (templateId: number, rule: TemplateRuleRecord) => {
    setActiveTemplateId(templateId);
    setEditingRule(rule);
    setRuleForm({
      name: rule.name,
      position: rule.position,
      enabled: rule.enabled,
      precedence: rule.precedence,
      action: rule.action,
      duration: rule.duration,
      operator_type: rule.operator_type || 'simple',
      operator_sensitive: rule.operator_sensitive,
      operator_operand: rule.operator_operand,
      operator_data: rule.operator_data,
      description: rule.description,
      nolog: rule.nolog,
    });
    setShowRuleEditor(true);
  };

  const saveRule = async () => {
    if (!activeTemplateId) return;
    try {
      if (editingRule) {
        await api.updateTemplateRule(activeTemplateId, editingRule.id, ruleForm);
      } else {
        await api.createTemplateRule(activeTemplateId, ruleForm);
      }
      setShowRuleEditor(false);
      setEditingRule(null);
      setRuleForm(defaultTemplateRuleForm);
      setError('');
      fetchData();
    } catch (err) {
      console.error('Failed to save template rule:', err);
      setError(err instanceof Error ? err.message : 'Failed to save template rule');
    }
  };

  const deleteRule = async (templateId: number, rule: TemplateRuleRecord) => {
    if (!confirm(`Delete template rule "${rule.name}"?`)) return;
    try {
      await api.deleteTemplateRule(templateId, rule.id);
      fetchData();
    } catch (err) {
      console.error('Failed to delete template rule:', err);
      setError(err instanceof Error ? err.message : 'Failed to delete template rule');
    }
  };

  const openCreateAttachment = (templateId: number) => {
    setActiveTemplateId(templateId);
    setEditingAttachment(null);
    setAttachmentForm({
      ...defaultAttachmentForm,
      target_ref: nodes[0]?.addr || '',
    });
    setShowAttachmentEditor(true);
  };

  const openEditAttachment = (templateId: number, attachment: TemplateAttachmentRecord) => {
    setActiveTemplateId(templateId);
    setEditingAttachment(attachment);
    setAttachmentForm({
      target_type: attachment.target_type,
      target_ref: attachment.target_ref,
      priority: attachment.priority,
    });
    setShowAttachmentEditor(true);
  };

  const saveAttachment = async () => {
    if (!activeTemplateId) return;
    const payload = {
      target_type: attachmentForm.target_type,
      target_ref: attachmentForm.target_ref,
      priority: attachmentForm.priority,
    };

    try {
      if (editingAttachment) {
        await api.updateTemplateAttachment(activeTemplateId, editingAttachment.id, payload);
      } else {
        await api.createTemplateAttachment(activeTemplateId, payload);
      }
      setShowAttachmentEditor(false);
      setEditingAttachment(null);
      setAttachmentForm(defaultAttachmentForm);
      setError('');
      fetchData();
    } catch (err) {
      console.error('Failed to save attachment:', err);
      setError(err instanceof Error ? err.message : 'Failed to save attachment');
    }
  };

  const deleteAttachment = async (templateId: number, attachment: TemplateAttachmentRecord) => {
    if (!confirm(`Delete attachment "${attachmentLabel(attachment)}"?`)) return;
    try {
      await api.deleteTemplateAttachment(templateId, attachment.id);
      fetchData();
    } catch (err) {
      console.error('Failed to delete attachment:', err);
      setError(err instanceof Error ? err.message : 'Failed to delete attachment');
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold">Templates</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Create reusable rule sets, attach them to nodes or tags, and let the backend reconcile node state.
          </p>
        </div>
        <button
          onClick={openCreateTemplate}
          className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/80"
        >
          <Plus className="h-4 w-4" /> New Template
        </button>
      </div>

      {error && (
        <div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="grid gap-4">
        {templates.map((template) => (
          <div key={template.id} className="rounded-xl border border-border bg-card p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div className="flex items-center gap-2">
                  <Shield className="h-4 w-4 text-primary" />
                  <h2 className="text-lg font-semibold">{template.name}</h2>
                </div>
                {template.description && (
                  <p className="mt-1 text-sm text-muted-foreground">{template.description}</p>
                )}
                <div className="mt-2 flex flex-wrap gap-2 text-xs">
                  <span className="rounded-full bg-primary/10 px-2 py-0.5 text-primary">
                    {template.rules.length} rules
                  </span>
                  <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                    {template.attachments.length} attachments
                  </span>
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => openEditTemplate(template)}
                  className="rounded-lg border border-border bg-muted px-3 py-1.5 text-sm hover:bg-muted/80"
                >
                  Edit
                </button>
                <button
                  onClick={() => deleteTemplate(template)}
                  className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-1.5 text-sm text-destructive hover:bg-destructive/20"
                >
                  Delete
                </button>
              </div>
            </div>

            <div className="mt-5 grid gap-4 lg:grid-cols-[1.5fr_1fr]">
              <div className="rounded-xl border border-border">
                <div className="flex items-center justify-between border-b border-border px-4 py-3">
                  <div className="text-sm font-medium">Template Rules</div>
                  <button
                    onClick={() => openCreateRule(template.id)}
                    className="rounded-lg border border-primary/30 bg-primary/10 px-3 py-1.5 text-xs text-primary hover:bg-primary/20"
                  >
                    Add Rule
                  </button>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border text-left text-xs text-muted-foreground">
                        <th className="px-4 py-2">Pos</th>
                        <th className="px-4 py-2">Name</th>
                        <th className="px-4 py-2">Action</th>
                        <th className="px-4 py-2">Match</th>
                        <th className="px-4 py-2">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {template.rules.map((rule) => (
                        <tr key={rule.id} className="border-b border-border/50 hover:bg-muted/40">
                          <td className="px-4 py-2 text-xs text-muted-foreground">{rule.position}</td>
                          <td className="px-4 py-2">
                            <div className="font-medium">{rule.name}</div>
                            <div className="text-xs text-muted-foreground">{rule.duration}</div>
                          </td>
                          <td className="px-4 py-2 text-xs">{rule.action}</td>
                          <td className="px-4 py-2 text-xs">
                            {rule.operator_operand}: {rule.operator_data}
                          </td>
                          <td className="px-4 py-2">
                            <div className="flex gap-1">
                              <button
                                onClick={() => openEditRule(template.id, rule)}
                                className="rounded p-1 hover:bg-muted"
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </button>
                              <button
                                onClick={() => deleteRule(template.id, rule)}
                                className="rounded p-1 text-destructive hover:bg-muted"
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                      {template.rules.length === 0 && (
                        <tr>
                          <td colSpan={5} className="px-4 py-8 text-center text-muted-foreground">
                            No template rules yet.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              <div className="rounded-xl border border-border">
                <div className="flex items-center justify-between border-b border-border px-4 py-3">
                  <div className="text-sm font-medium">Attachments</div>
                  <button
                    onClick={() => openCreateAttachment(template.id)}
                    className="rounded-lg border border-primary/30 bg-primary/10 px-3 py-1.5 text-xs text-primary hover:bg-primary/20"
                  >
                    Add Attachment
                  </button>
                </div>

                <div className="space-y-3 p-4">
                  {template.attachments.map((attachment) => (
                    <div key={attachment.id} className="rounded-lg border border-border bg-muted/30 p-3">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="flex items-center gap-2 text-sm font-medium">
                            {attachment.target_type === 'node'
                              ? <Server className="h-4 w-4 text-primary" />
                              : <Tag className="h-4 w-4 text-primary" />}
                            {attachmentLabel(attachment)}
                          </div>
                          <div className="mt-1 text-xs text-muted-foreground">
                            Priority {attachment.priority}
                          </div>
                        </div>
                        <div className="flex gap-1">
                          <button
                            onClick={() => openEditAttachment(template.id, attachment)}
                            className="rounded p-1 hover:bg-muted"
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </button>
                          <button
                            onClick={() => deleteAttachment(template.id, attachment)}
                            className="rounded p-1 text-destructive hover:bg-muted"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>
                  ))}
                  {template.attachments.length === 0 && (
                    <div className="py-8 text-center text-sm text-muted-foreground">
                      No attachments yet.
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        ))}

        {templates.length === 0 && (
          <div className="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">
            No templates yet. Create one and attach it to a node or tag.
          </div>
        )}
      </div>

      {showTemplateEditor && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="w-full max-w-lg rounded-xl border border-border bg-card p-5 space-y-4">
            <h2 className="text-lg font-semibold">
              {editingTemplate ? 'Edit Template' : 'New Template'}
            </h2>
            <div>
              <label className="text-xs text-muted-foreground">Name</label>
              <input
                value={templateForm.name}
                onChange={(e) => setTemplateForm((prev) => ({ ...prev, name: e.target.value }))}
                className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Description</label>
              <textarea
                value={templateForm.description}
                onChange={(e) => setTemplateForm((prev) => ({ ...prev, description: e.target.value }))}
                rows={4}
                className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
              />
            </div>
            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setShowTemplateEditor(false)}
                className="flex-1 rounded-lg border border-border bg-muted py-2 text-sm hover:bg-muted/80"
              >
                Cancel
              </button>
              <button
                onClick={saveTemplate}
                className="flex-1 rounded-lg bg-primary py-2 text-sm font-medium text-primary-foreground hover:bg-primary/80"
              >
                {editingTemplate ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showRuleEditor && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-xl border border-border bg-card p-5 space-y-4">
            <h2 className="text-lg font-semibold">
              {editingRule ? 'Edit Template Rule' : 'New Template Rule'}
            </h2>
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <label className="text-xs text-muted-foreground">Name</label>
                <input
                  value={ruleForm.name}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, name: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Position</label>
                <input
                  type="number"
                  value={ruleForm.position}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, position: Number(e.target.value) }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Action</label>
                <select
                  value={ruleForm.action}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, action: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                >
                  <option value="allow">Allow</option>
                  <option value="deny">Deny</option>
                  <option value="reject">Reject</option>
                </select>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Duration</label>
                <select
                  value={ruleForm.duration}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, duration: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
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
                  value={ruleForm.operator_operand}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, operator_operand: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
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
              <div>
                <label className="text-xs text-muted-foreground">Data</label>
                <input
                  value={ruleForm.operator_data}
                  onChange={(e) => setRuleForm((prev) => ({ ...prev, operator_data: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div>
              <label className="text-xs text-muted-foreground">Description</label>
              <input
                value={ruleForm.description}
                onChange={(e) => setRuleForm((prev) => ({ ...prev, description: e.target.value }))}
                className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
              />
            </div>
            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setShowRuleEditor(false)}
                className="flex-1 rounded-lg border border-border bg-muted py-2 text-sm hover:bg-muted/80"
              >
                Cancel
              </button>
              <button
                onClick={saveRule}
                className="flex-1 rounded-lg bg-primary py-2 text-sm font-medium text-primary-foreground hover:bg-primary/80"
              >
                {editingRule ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showAttachmentEditor && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div className="w-full max-w-lg rounded-xl border border-border bg-card p-5 space-y-4">
            <h2 className="text-lg font-semibold">
              {editingAttachment ? 'Edit Attachment' : 'New Attachment'}
            </h2>
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <label className="text-xs text-muted-foreground">Target Type</label>
                <select
                  value={attachmentForm.target_type}
                  onChange={(e) => setAttachmentForm((prev) => ({
                    ...prev,
                    target_type: e.target.value as 'node' | 'tag',
                    target_ref: e.target.value === 'node' ? nodes[0]?.addr || '' : '',
                  }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                >
                  <option value="node">Node</option>
                  <option value="tag">Tag</option>
                </select>
              </div>
              <div>
                <label className="text-xs text-muted-foreground">Priority</label>
                <input
                  type="number"
                  value={attachmentForm.priority}
                  onChange={(e) => setAttachmentForm((prev) => ({ ...prev, priority: Number(e.target.value) }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
              </div>
            </div>

            {attachmentForm.target_type === 'node' ? (
              <div>
                <label className="text-xs text-muted-foreground">Target Node</label>
                <select
                  value={attachmentForm.target_ref}
                  onChange={(e) => setAttachmentForm((prev) => ({ ...prev, target_ref: e.target.value }))}
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                >
                  {nodes.map((node) => (
                    <option key={node.addr} value={node.addr}>
                      {node.hostname || node.addr}
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <div>
                <label className="text-xs text-muted-foreground">Target Tag</label>
                <input
                  value={attachmentForm.target_ref}
                  onChange={(e) => setAttachmentForm((prev) => ({ ...prev, target_ref: e.target.value }))}
                  placeholder={knownTags[0] || 'server'}
                  list="template-known-tags"
                  className="mt-1 w-full rounded-lg border border-border bg-muted px-3 py-2 text-sm"
                />
                <datalist id="template-known-tags">
                  {knownTags.map((tag) => (
                    <option key={tag} value={tag} />
                  ))}
                </datalist>
              </div>
            )}

            <div className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
              Lower priority numbers win when multiple attachments match the same node.
            </div>

            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setShowAttachmentEditor(false)}
                className="flex-1 rounded-lg border border-border bg-muted py-2 text-sm hover:bg-muted/80"
              >
                Cancel
              </button>
              <button
                onClick={saveAttachment}
                className="flex-1 rounded-lg bg-primary py-2 text-sm font-medium text-primary-foreground hover:bg-primary/80"
              >
                {editingAttachment ? 'Save' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
