import { useState } from 'react';
import { useAppStore } from '@/stores/app-store';

export default function SettingsPage() {
  const { user } = useAppStore();
  const [saved, setSaved] = useState(false);

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">Settings</h1>

      <div className="bg-card border border-border rounded-xl p-5 space-y-4">
        <h2 className="font-medium">Account</h2>
        <div className="text-sm text-muted-foreground">Logged in as: <span className="text-foreground">{user}</span></div>
      </div>

      <div className="bg-card border border-border rounded-xl p-5 space-y-4">
        <h2 className="font-medium">About</h2>
        <div className="text-sm text-muted-foreground space-y-1">
          <div>OpenSnitch Web UI</div>
          <div>A browser-based interface for managing OpenSnitch firewalls.</div>
          <div className="mt-2">
            Backend: Go | Frontend: React + TypeScript + Vite + Tailwind
          </div>
        </div>
      </div>
    </div>
  );
}
