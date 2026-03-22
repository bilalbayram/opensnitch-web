import { useState, useEffect } from 'react';
import { useAppStore } from '@/stores/app-store';
import { api } from '@/lib/api';
import { Shield, Clock } from 'lucide-react';

const durations = [
  { value: 'once', label: 'Once' },
  { value: '5m', label: '5 min' },
  { value: '15m', label: '15 min' },
  { value: '30m', label: '30 min' },
  { value: '1h', label: '1 hour' },
  { value: 'until restart', label: 'Restart' },
  { value: 'always', label: 'Always' },
];

export function PromptOverlay() {
  const { prompts, removePrompt } = useAppStore();
  const prompt = prompts[0]; // Show first prompt

  const [duration, setDuration] = useState('once');
  const [operand, setOperand] = useState('process.path');
  const [countdown, setCountdown] = useState(120);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!prompt) return;
    setCountdown(120);
    setDuration('once');
    setOperand('process.path');

    const timer = setInterval(() => {
      setCountdown((c) => {
        if (c <= 1) {
          clearInterval(timer);
          return 0;
        }
        return c - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [prompt?.id]);

  if (!prompt) return null;

  const handleReply = async (action: string) => {
    setLoading(true);
    try {
      let data = prompt.process;
      if (operand === 'dest.host') data = prompt.dst_host;
      else if (operand === 'dest.ip') data = prompt.dst_ip;
      else if (operand === 'dest.port') data = String(prompt.dst_port);
      else if (operand === 'user.id') data = String(prompt.uid);

      await api.replyPrompt(prompt.id, {
        action,
        duration,
        name: `web-rule-${Date.now()}`,
        operand,
        data,
        operator: 'simple',
      });
      removePrompt(prompt.id);
    } catch (e) {
      console.error('Failed to reply to prompt:', e);
    } finally {
      setLoading(false);
    }
  };

  const progressPercent = (countdown / 120) * 100;
  const urgent = countdown <= 30;
  const routerManaged = Boolean(prompt.router_managed);

  return (
    <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-end md:items-center justify-center">
      <div className="bg-card border-t md:border border-border md:rounded-xl md:shadow-2xl w-full md:max-w-lg max-h-[100vh] md:max-h-[90vh] flex flex-col rounded-t-2xl md:rounded-xl">
        {/* Drag handle (mobile) */}
        <div className="flex justify-center pt-2 md:hidden">
          <div className="w-10 h-1 rounded-full bg-border" />
        </div>

        {/* Header + Progress */}
        <div className="shrink-0">
          <div className="flex items-center justify-between px-5 py-3 md:py-4">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-warning" />
              <h2 className="font-semibold text-sm md:text-base">Connection Request</h2>
            </div>
            <div className={`flex items-center gap-1.5 text-sm ${urgent ? 'text-destructive' : 'text-muted-foreground'}`}>
              <Clock className="h-4 w-4" />
              <span className="tabular-nums font-medium">{countdown}s</span>
            </div>
          </div>
          <div className="h-1 bg-muted">
            <div
              className={`h-full transition-all duration-1000 ${urgent ? 'bg-destructive' : 'bg-warning'}`}
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        </div>

        {/* Scrollable body */}
        <div className="flex-1 overflow-y-auto overscroll-contain px-5 py-4 space-y-4">
          {/* Process info */}
          <div className="bg-muted rounded-lg p-3 space-y-1.5">
            <div className="text-xs text-muted-foreground">Process</div>
            <div className="font-mono text-sm break-all">{prompt.process}</div>
            {prompt.args?.length > 0 && (
              <div className="text-xs text-muted-foreground break-all">{prompt.args.join(' ')}</div>
            )}
            <div className="text-xs text-muted-foreground">PID: {prompt.pid} | UID: {prompt.uid}</div>
          </div>

          {/* Connection info */}
          <div className="bg-muted rounded-lg p-3 space-y-1.5">
            <div className="text-xs text-muted-foreground">Connection</div>
            <div className="font-mono text-sm break-all">
              {prompt.protocol?.toUpperCase()} {prompt.src_ip}:{prompt.src_port} → {prompt.dst_host || prompt.dst_ip}:{prompt.dst_port}
            </div>
            {prompt.dst_host && prompt.dst_ip && (
              <div className="text-xs text-muted-foreground">IP: {prompt.dst_ip}</div>
            )}
          </div>

          <div className="text-xs text-muted-foreground">Node: {prompt.node_addr}</div>

          {/* Operand selector */}
          <div>
            <label className="text-xs text-muted-foreground block mb-1.5">Apply rule to</label>
            <select
              value={operand}
              onChange={(e) => setOperand(e.target.value)}
              className="w-full bg-muted border border-border rounded-lg px-3 py-2.5 text-sm"
            >
              <option value="process.path">Process: {prompt.process}</option>
              {!routerManaged && prompt.dst_host && <option value="dest.host">Host: {prompt.dst_host}</option>}
              <option value="dest.ip">IP: {prompt.dst_ip}</option>
              <option value="dest.port">Port: {prompt.dst_port}</option>
              <option value="user.id">User: {prompt.uid}</option>
            </select>
            {routerManaged && (
              <div className="mt-2 text-xs text-muted-foreground">
                Router-managed prompts only support process path, destination IP, destination port, and user ID operands.
              </div>
            )}
          </div>

          {/* Duration selector — larger touch targets on mobile */}
          <div>
            <label className="text-xs text-muted-foreground block mb-1.5">Duration</label>
            <div className="flex flex-wrap gap-1.5">
              {durations.map((d) => (
                <button
                  key={d.value}
                  onClick={() => setDuration(d.value)}
                  className={`px-3 py-2 md:px-2.5 md:py-1 text-xs rounded-lg border transition-colors ${
                    duration === d.value
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-muted border-border text-muted-foreground hover:text-foreground'
                  }`}
                >
                  {d.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Sticky action buttons — always visible */}
        <div className="shrink-0 border-t border-border bg-card px-5 py-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))]">
          <div className="flex gap-3">
            <button
              onClick={() => handleReply('deny')}
              disabled={loading}
              className="flex-1 bg-destructive hover:bg-destructive/80 text-white rounded-xl py-3 md:py-2.5 text-sm font-semibold transition-colors disabled:opacity-50 active:scale-[0.98]"
            >
              Deny
            </button>
            <button
              onClick={() => handleReply('reject')}
              disabled={loading}
              className="flex-1 bg-warning hover:bg-warning/80 text-black rounded-xl py-3 md:py-2.5 text-sm font-semibold transition-colors disabled:opacity-50 active:scale-[0.98]"
            >
              Reject
            </button>
            <button
              onClick={() => handleReply('allow')}
              disabled={loading}
              className="flex-1 bg-success hover:bg-success/80 text-white rounded-xl py-3 md:py-2.5 text-sm font-semibold transition-colors disabled:opacity-50 active:scale-[0.98]"
            >
              Allow
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
