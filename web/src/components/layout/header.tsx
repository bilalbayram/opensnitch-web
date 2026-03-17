import { useAppStore } from '@/stores/app-store';
import { cn } from '@/lib/utils';
import { Wifi, WifiOff, LogOut, Shield, ArrowUpCircle } from 'lucide-react';
import { api } from '@/lib/api';
import { useNavigate, NavLink } from 'react-router-dom';

export function Header() {
  const { user, wsConnected, setUser, updateAvailable, latestVersion } = useAppStore();
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
          <Shield className="h-4 w-4 text-primary" />
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
        {updateAvailable && (
          <NavLink
            to="/settings"
            className="flex items-center gap-1.5 text-xs bg-primary/10 text-primary px-2.5 py-1 rounded-full hover:bg-primary/20 transition-colors"
          >
            <ArrowUpCircle className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">{latestVersion}</span>
          </NavLink>
        )}
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
