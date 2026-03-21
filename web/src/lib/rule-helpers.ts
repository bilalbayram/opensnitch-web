import type { RuleForm } from "@/components/rule-editor-sheet";
import { defaultForm } from "@/components/rule-editor-sheet";

/** Minimal interface satisfied by both ConnectionRecord (api.ts) and ConnectionEvent (app-store.ts) */
export interface ConnectionLike {
  node: string;
  dst_host: string;
  dst_ip: string;
  dst_port: number;
  process: string;
  protocol: string;
  uid: number;
  router_managed?: boolean;
}

export function isDeviceSource(process: string): boolean {
  return process.startsWith("device:");
}

export function formatProcessLabel(process: string): string {
  if (!isDeviceSource(process)) {
    return process;
  }
  return `Source device ${process.slice("device:".length)}`;
}

/** Pick the best operand and data from a connection */
export function smartOperandFromConnection(conn: ConnectionLike): {
  operand: string;
  data: string;
} {
  if (conn.router_managed) {
    return { operand: "dest.ip", data: conn.dst_ip };
  }
  if (conn.dst_host) {
    return { operand: "dest.host", data: conn.dst_host };
  }
  return { operand: "dest.ip", data: conn.dst_ip };
}

/** Generate a slug-style rule name with a short unique suffix */
export function generateRuleName(
  action: string,
  operand: string,
  data: string,
): string {
  const slug = data
    .replace(/[^a-zA-Z0-9.-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "")
    .toLowerCase()
    .slice(0, 30);
  const suffix = Date.now().toString(36).slice(-4);
  const operandShort = operand.split(".").pop() || operand;
  return `${action}-${operandShort}-${slug}-${suffix}`;
}

/** Build a complete RuleForm from a connection */
export function connectionToRuleForm(
  conn: ConnectionLike,
  action: string,
  duration: string,
): RuleForm {
  const { operand, data } = smartOperandFromConnection(conn);
  return {
    ...defaultForm,
    name: generateRuleName(action, operand, data),
    node: conn.node,
    action,
    duration,
    operator_operand: operand,
    operator_data: data,
    description: `Quick rule from ${conn.process?.split("/").pop() || "connection"} → ${data}`,
  };
}
