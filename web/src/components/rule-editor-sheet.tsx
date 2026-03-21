import { useState, useEffect } from "react";
import { BottomSheet } from "@/components/ui/bottom-sheet";

export interface RuleForm {
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

export const defaultForm: RuleForm = {
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

export const operandLabels: Record<string, string> = {
  "process.path": "Process",
  "process.command": "Command",
  "dest.host": "Host",
  "dest.ip": "IP",
  "dest.port": "Port",
  "user.id": "User",
  protocol: "Protocol",
};

const standardOperandOptions = [
  { value: "process.path", label: "Process Path" },
  { value: "process.command", label: "Process Command" },
  { value: "dest.host", label: "Dest Host" },
  { value: "dest.ip", label: "Dest IP" },
  { value: "dest.port", label: "Dest Port" },
  { value: "user.id", label: "User ID" },
  { value: "protocol", label: "Protocol" },
];

const routerManagedOperandOptions = [
  { value: "process.path", label: "Process Path" },
  { value: "dest.ip", label: "Dest IP" },
  { value: "dest.port", label: "Dest Port" },
  { value: "user.id", label: "User ID" },
  { value: "protocol", label: "Protocol" },
];

interface RuleEditorSheetProps {
  open: boolean;
  onClose: () => void;
  initialValues?: Partial<RuleForm>;
  editing?: boolean;
  onSave: (form: RuleForm) => Promise<void>;
  title?: string;
  routerManaged?: boolean;
}

export function RuleEditorSheet({
  open,
  onClose,
  initialValues,
  editing = false,
  onSave,
  title,
  routerManaged = false,
}: RuleEditorSheetProps) {
  const [form, setForm] = useState<RuleForm>({ ...defaultForm });
  const operandOptions = routerManaged
    ? routerManagedOperandOptions
    : standardOperandOptions;

  useEffect(() => {
    if (open) {
      const next = { ...defaultForm, ...initialValues };
      if (
        routerManaged &&
        !operandOptions.some((option) => option.value === next.operator_operand)
      ) {
        next.operator_operand = "dest.ip";
        next.operator_data = next.operator_operand === "dest.ip" ? next.operator_data : "";
      }
      setForm(next);
    }
  }, [open, initialValues, operandOptions, routerManaged]);

  const handleSave = async () => {
    await onSave(form);
  };

  return (
    <BottomSheet
      open={open}
      onClose={onClose}
      title={title ?? (editing ? "Edit Rule" : "New Rule")}
      stickyFooter={
        <div className="flex gap-3">
          <button
            onClick={onClose}
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
        {routerManaged && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
            Router-managed nodes only support process path, destination IP,
            destination port, protocol, and user ID operands. Forwarded traffic
            in v1 is device and network rule based, with no live prompts.
          </div>
        )}

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
              {operandOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
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
            placeholder={
              routerManaged
                ? "e.g. /usr/bin/curl, 1.1.1.1, 443"
                : "e.g. /usr/bin/curl or google.com"
            }
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
  );
}
