import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { useAppStore } from '@/stores/app-store';
import { Settings, RefreshCw, Download, CheckCircle, AlertCircle } from 'lucide-react';

interface VersionInfo {
  current_version: string;
  build_time?: string;
  latest_version?: string;
  update_available: boolean;
  last_check?: string;
  checking: boolean;
  downloading: boolean;
  error?: string;
  release?: {
    tag_name: string;
    published_at: string;
    html_url: string;
    body: string;
  };
}

export default function SettingsPage() {
  const [info, setInfo] = useState<VersionInfo | null>(null);
  const [status, setStatus] = useState('');
  const [applying, setApplying] = useState(false);
  const { setUpdateAvailable } = useAppStore();

  const fetchVersion = () => {
    api.getVersion().then((data) => {
      setInfo(data);
      if (data.update_available) {
        setUpdateAvailable(true, data.latest_version ?? null);
      }
    }).catch(console.error);
  };

  useEffect(() => { fetchVersion(); }, []);

  const handleCheck = async () => {
    setStatus('');
    try {
      const result = await api.checkUpdate();
      setInfo(result);
      if (result.update_available) {
        setUpdateAvailable(true, result.latest_version ?? null);
        setStatus('Update available!');
      } else {
        setStatus('You are up to date.');
      }
    } catch (e: any) {
      setStatus(e.message || 'Check failed');
    }
  };

  const handleApply = async () => {
    if (!confirm('Apply update and restart the server? It will be briefly unavailable.')) return;
    setApplying(true);
    setStatus('Downloading and applying update...');
    try {
      await api.applyUpdate();
      setStatus('Update applied! Server is restarting...');
    } catch (e: any) {
      setStatus(e.message || 'Update failed');
      setApplying(false);
    }
  };

  return (
    <div className="p-4 md:p-6 space-y-6 max-w-2xl">
      <div className="flex items-center gap-3">
        <Settings className="h-5 w-5 text-primary" />
        <h1 className="text-lg font-semibold">Settings</h1>
      </div>

      {/* About */}
      <div className="bg-card border border-border rounded-xl p-4 space-y-3">
        <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">About</h2>
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Version</span>
            <span className="font-mono">{info?.current_version ?? '...'}</span>
          </div>
          {info?.build_time && (
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">Built</span>
              <span className="font-mono">{info.build_time}</span>
            </div>
          )}
        </div>
      </div>

      {/* Updates */}
      <div className="bg-card border border-border rounded-xl p-4 space-y-4">
        <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">Updates</h2>

        {info?.update_available && info.release && (
          <div className="bg-primary/5 border border-primary/20 rounded-lg p-3 space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium text-primary">
              <Download className="h-4 w-4" />
              Update available: {info.release.tag_name}
            </div>
            {info.release.body && (
              <p className="text-xs text-muted-foreground line-clamp-4 whitespace-pre-line">
                {info.release.body}
              </p>
            )}
            <button
              onClick={handleApply}
              disabled={applying}
              className="mt-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm font-medium hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {applying ? 'Updating...' : 'Update Now'}
            </button>
          </div>
        )}

        {info && !info.update_available && info.latest_version && (
          <div className="flex items-center gap-2 text-sm text-success">
            <CheckCircle className="h-4 w-4" />
            Up to date
          </div>
        )}

        <div className="flex items-center gap-3">
          <button
            onClick={handleCheck}
            disabled={info?.checking}
            className="flex items-center gap-2 px-3 py-1.5 bg-muted border border-border rounded-lg text-sm hover:bg-muted/80 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${info?.checking ? 'animate-spin' : ''}`} />
            Check for updates
          </button>
          {info?.last_check && (
            <span className="text-xs text-muted-foreground">
              Last checked: {new Date(info.last_check).toLocaleString()}
            </span>
          )}
        </div>

        {status && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            {info?.error ? <AlertCircle className="h-4 w-4 text-destructive" /> : null}
            {status}
          </div>
        )}

        {info?.error && (
          <div className="text-xs text-destructive">{info.error}</div>
        )}
      </div>
    </div>
  );
}
