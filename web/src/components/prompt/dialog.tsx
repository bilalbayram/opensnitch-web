import { useState, useEffect } from 'react';
import { useAppStore } from '@/stores/app-store';
import { api } from '@/lib/api';
import { Shield, X, Clock } from 'lucide-react';

const durations = [
  { value: 'once', label: 'Once' },
  { value: '5m', label: '5 minutes' },
  { value: '15m', label: '15 minutes' },
  { value: '30m', label: '30 minutes' },
  { value: '1h', label: '1 hour' },
  { value: 'until restart', label: 'Until restart' },
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

  return (
    <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-card border border-border rounded-xl shadow-2xl w-full max-w-lg">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <Shield className="h-5 w-5 text-warning" />
            <h2 className="font-semibold">Connection Request</h2>
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Clock className="h-4 w-4" />
            <span>{countdown}s</span>
          </div>
        </div>

        {/* Progress bar */}
        <div className="h-1 bg-muted">
          <div
            className="h-full bg-warning transition-all duration-1000"
            style={{ width: `${progressPercent}%` }}
          />
        </div>

        {/* Body */}
        <div className="px-5 py-4 space-y-4">
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
            <div className="font-mono text-sm">
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
              className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm"
            >
              <option value="process.path">Process: {prompt.process}</option>
              {prompt.dst_host && <option value="dest.host">Host: {prompt.dst_host}</option>}
              <option value="dest.ip">IP: {prompt.dst_ip}</option>
              <option value="dest.port">Port: {prompt.dst_port}</option>
              <option value="user.id">User: {prompt.uid}</option>
            </select>
          </div>

          {/* Duration selector */}
          <div>
            <label className="text-xs text-muted-foreground block mb-1.5">Duration</label>
            <div className="flex flex-wrap gap-1.5">
              {durations.map((d) => (
                <button
                  key={d.value}
                  onClick={() => setDuration(d.value)}
                  className={`px-2.5 py-1 text-xs rounded-lg border transition-colors ${
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

        {/* Actions */}
        <div className="flex gap-3 px-5 py-4 border-t border-border">
          <button
            onClick={() => handleReply('deny')}
            disabled={loading}
            className="flex-1 bg-destructive hover:bg-destructive/80 text-white rounded-lg py-2.5 text-sm font-medium transition-colors disabled:opacity-50"
          >
            Deny
          </button>
          <button
            onClick={() => handleReply('reject')}
            disabled={loading}
            className="flex-1 bg-warning hover:bg-warning/80 text-black rounded-lg py-2.5 text-sm font-medium transition-colors disabled:opacity-50"
          >
            Reject
          </button>
          <button
            onClick={() => handleReply('allow')}
            disabled={loading}
            className="flex-1 bg-success hover:bg-success/80 text-white rounded-lg py-2.5 text-sm font-medium transition-colors disabled:opacity-50"
          >
            Allow
          </button>
        </div>
      </div>
    </div>
  );
}
