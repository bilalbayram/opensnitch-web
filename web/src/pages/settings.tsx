import { useEffect, useState } from 'react';
import { api, type VersionInfo } from '@/lib/api';
import { Settings } from 'lucide-react';

export default function SettingsPage() {
  const [info, setInfo] = useState<VersionInfo | null>(null);

  const fetchVersion = () => {
    api.getVersion().then(setInfo).catch(console.error);
  };

  useEffect(() => { fetchVersion(); }, []);

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
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Live Data</span>
            <span className="font-mono">WebSocket + API</span>
          </div>
        </div>
      </div>

      <div className="bg-card border border-border rounded-xl p-4 space-y-4">
        <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">Notes</h2>
        <p className="text-sm text-muted-foreground">
          This page now shows build information only. Update checks and release downloads are not part of the product surface.
        </p>
      </div>
    </div>
  );
}
