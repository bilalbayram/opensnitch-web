import { useAppStore } from '@/stores/app-store';
import { cn } from '@/lib/utils';
import { Wifi, WifiOff, LogOut } from 'lucide-react';
import { api } from '@/lib/api';
import { useNavigate } from 'react-router-dom';
import { AppLogo } from '@/components/app-logo';

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
        {/* Mobile brand — shown only when sidebar is hidden */}
        <div className="flex items-center gap-2 md:hidden">
          <AppLogo className="h-4 w-4" />
          <span className="font-bold text-sm">OpenSnitch</span>
        </div>

        <div className={cn(
          'flex items-center gap-1.5 text-xs',
          wsConnected ? 'text-success' : 'text-muted-foreground',
          'md:ml-0 ml-3'
        )}>
          {wsConnected ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
          <span className="hidden sm:inline">{wsConnected ? 'Live' : 'Disconnected'}</span>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className="text-xs text-muted-foreground hidden sm:inline">{user}</span>
        <button
          onClick={handleLogout}
          className="text-muted-foreground hover:text-foreground transition-colors p-1"
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </header>
  );
}
