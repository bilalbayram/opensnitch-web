import { useAppStore } from '@/stores/app-store';
import { cn } from '@/lib/utils';
import { Wifi, WifiOff, LogOut } from 'lucide-react';
import { api } from '@/lib/api';
import { useNavigate } from 'react-router-dom';

export function Header() {
  const { user, wsConnected, setUser } = useAppStore();
  const navigate = useNavigate();

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch { /* ignore */ }
    setUser(null, null);
    navigate('/login');
  };

  return (
    <header className="h-12 border-b border-border bg-card flex items-center justify-between px-4">
      <div className="flex items-center gap-2">
        <div className={cn('flex items-center gap-1.5 text-xs', wsConnected ? 'text-success' : 'text-muted-foreground')}>
          {wsConnected ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
          {wsConnected ? 'Live' : 'Disconnected'}
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className="text-xs text-muted-foreground">{user}</span>
        <button onClick={handleLogout} className="text-muted-foreground hover:text-foreground transition-colors">
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </header>
  );
}
