import { useState, useRef, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { Shield, Check, AlertTriangle } from "lucide-react";
import { useIsMobile } from "@/hooks/use-media-query";
import { useAppStore } from "@/stores/app-store";
import { BottomSheet } from "@/components/ui/bottom-sheet";
import { api } from "@/lib/api";
import type { RuleForm } from "@/components/rule-editor-sheet";
import type { ConnectionLike } from "@/lib/rule-helpers";
import {
  smartOperandFromConnection,
  connectionToRuleForm,
} from "@/lib/rule-helpers";

const durations = [
  { value: "once", label: "Once" },
  { value: "5m", label: "5m" },
  { value: "15m", label: "15m" },
  { value: "30m", label: "30m" },
  { value: "1h", label: "1h" },
  { value: "until restart", label: "Restart" },
  { value: "always", label: "Always" },
];

interface QuickRulePopoverProps {
  connection: ConnectionLike;
  onAdvanced?: (prefill: Partial<RuleForm>) => void;
  onCreated?: () => void;
  /** Compact mode for dashboard live feed rows */
  compact?: boolean;
}

function PopoverContent({
  connection,
  onAdvanced,
  onCreated,
  onClose,
}: {
  connection: ConnectionLike;
  onAdvanced?: (prefill: Partial<RuleForm>) => void;
  onCreated?: () => void;
  onClose: () => void;
}) {
  const [duration, setDuration] = useState("always");
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [errorMsg, setErrorMsg] = useState("");
  const nodesOnline = useAppStore((s) => s.nodesOnline);

  const { operand, data } = smartOperandFromConnection(connection);
  const nodeOffline = connection.node && !nodesOnline.has(connection.node);

  const handleAction = async (action: string) => {
    try {
      const form = connectionToRuleForm(connection, action, duration);
      await api.createRule(form);
      setStatus("success");
      onCreated?.();
      setTimeout(onClose, 1000);
    } catch (e) {
      setStatus("error");
      setErrorMsg(e instanceof Error ? e.message : "Failed to create rule");
    }
  };

  const handleAdvanced = () => {
    const form = connectionToRuleForm(connection, "deny", duration);
    onAdvanced?.(form);
    onClose();
  };

  const operandLabel =
    operand === "dest.host" ? "Host" : operand === "dest.ip" ? "IP" : operand;

  return (
    <div className="space-y-3">
      {/* Smart default info */}
      <div className="bg-muted rounded-lg p-2 text-xs font-mono">
        {operandLabel}: <span className="text-foreground">{data}</span>
      </div>

      {nodeOffline && (
        <div className="flex items-center gap-1.5 text-xs text-warning">
          <AlertTriangle className="h-3 w-3" />
          Node may be offline
        </div>
      )}

      {/* Duration pills */}
      <div>
        <label className="text-xs text-muted-foreground block mb-1.5">
          Duration
        </label>
        <div className="flex flex-wrap gap-1">
          {durations.map((d) => (
            <button
              key={d.value}
              onClick={() => setDuration(d.value)}
              className={`px-2 py-1 text-xs rounded-lg border transition-colors ${
                duration === d.value
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-muted border-border text-muted-foreground hover:text-foreground"
              }`}
            >
              {d.label}
            </button>
          ))}
        </div>
      </div>

      {/* Status feedback */}
      {status === "success" && (
        <div className="flex items-center gap-1.5 text-xs text-success">
          <Check className="h-3 w-3" />
          Rule created
        </div>
      )}
      {status === "error" && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-2 py-1.5 text-xs text-destructive">
          {errorMsg}
        </div>
      )}

      {/* Actions */}
      {status === "idle" && (
        <>
          <div className="flex gap-2">
            <button
              onClick={() => handleAction("allow")}
              className="flex-1 bg-success/15 text-success border border-success/30 rounded-lg py-2 text-sm font-medium hover:bg-success/25 transition-colors"
            >
              Allow
            </button>
            <button
              onClick={() => handleAction("deny")}
              className="flex-1 bg-destructive/15 text-destructive border border-destructive/30 rounded-lg py-2 text-sm font-medium hover:bg-destructive/25 transition-colors"
            >
              Deny
            </button>
          </div>

          {onAdvanced && (
            <button
              onClick={handleAdvanced}
              className="w-full text-xs text-muted-foreground hover:text-foreground transition-colors py-1"
            >
              Advanced&hellip;
            </button>
          )}
        </>
      )}
    </div>
  );
}

export function QuickRulePopover({
  connection,
  onAdvanced,
  onCreated,
  compact = false,
}: QuickRulePopoverProps) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const isMobile = useIsMobile();

  const close = useCallback(() => setOpen(false), []);

  // Close on Escape and click-outside (desktop popover only)
  useEffect(() => {
    if (!open || isMobile) return;

    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    const handleMouseDown = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node) &&
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        close();
      }
    };

    document.addEventListener("keydown", handleKey);
    document.addEventListener("mousedown", handleMouseDown);
    return () => {
      document.removeEventListener("keydown", handleKey);
      document.removeEventListener("mousedown", handleMouseDown);
    };
  }, [open, isMobile, close]);

  // Position desktop popover
  const [pos, setPos] = useState({ top: 0, left: 0 });
  useEffect(() => {
    if (!open || isMobile || !triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    const popoverWidth = 280;

    let left = rect.left + rect.width / 2 - popoverWidth / 2;
    // Keep within viewport
    if (left < 8) left = 8;
    if (left + popoverWidth > window.innerWidth - 8)
      left = window.innerWidth - popoverWidth - 8;

    let top = rect.bottom + 6;
    // If near bottom, show above
    if (top + 300 > window.innerHeight) {
      top = rect.top - 6;
    }

    setPos({ top, left });
  }, [open, isMobile]);

  const triggerButton = (
    <button
      ref={triggerRef}
      onClick={(e) => {
        e.stopPropagation();
        setOpen(!open);
      }}
      title="Quick rule"
      className={`rounded-md hover:bg-muted transition-colors text-muted-foreground hover:text-foreground ${
        compact ? "p-0.5" : "p-1"
      }`}
    >
      <Shield className={compact ? "h-3 w-3" : "h-3.5 w-3.5"} />
    </button>
  );

  // Mobile: use BottomSheet
  if (isMobile) {
    return (
      <>
        {triggerButton}
        <BottomSheet
          open={open}
          onClose={close}
          title="Quick Rule"
        >
          <div className="px-5 py-4">
            <PopoverContent
              connection={connection}
              onAdvanced={onAdvanced}
              onCreated={onCreated}
              onClose={close}
            />
          </div>
        </BottomSheet>
      </>
    );
  }

  // Desktop: portal popover
  return (
    <>
      {triggerButton}
      {open &&
        createPortal(
          <div
            ref={popoverRef}
            className="fixed z-50 w-[280px] bg-card border border-border rounded-xl shadow-xl p-3"
            style={{ top: pos.top, left: pos.left }}
          >
            <PopoverContent
              connection={connection}
              onAdvanced={onAdvanced}
              onCreated={onCreated}
              onClose={close}
            />
          </div>,
          document.body,
        )}
    </>
  );
}
