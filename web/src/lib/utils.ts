import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toString();
}

export function actionColor(action: string): string {
  switch (action?.toLowerCase()) {
    case 'allow': return 'text-success';
    case 'deny': return 'text-destructive';
    case 'reject': return 'text-warning';
    default: return 'text-muted-foreground';
  }
}

export function priorityLabel(p: number): string {
  switch (p) {
    case 0: return 'Low';
    case 1: return 'Medium';
    case 2: return 'High';
    default: return 'Unknown';
  }
}

export function alertTypeLabel(t: number): string {
  switch (t) {
    case 0: return 'Error';
    case 1: return 'Warning';
    case 2: return 'Info';
    default: return 'Unknown';
  }
}
