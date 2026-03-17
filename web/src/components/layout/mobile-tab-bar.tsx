import { useState } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Network,
  Shield,
  Server,
  Bell,
  MoreHorizontal,
  BarChart3,
  Flame,
  ShieldBan,
  Settings,
  X,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAppStore } from '@/stores/app-store';

const primaryTabs = [
  { to: '/', icon: LayoutDashboard, label: 'Home', end: true },
  { to: '/connections', icon: Network, label: 'Traffic', end: false },
  { to: '/rules', icon: Shield, label: 'Rules', end: false },
  { to: '/nodes', icon: Server, label: 'Nodes', end: false },
  { to: '/alerts', icon: Bell, label: 'Alerts', end: false, badge: true },
];

const moreTabs = [
  { to: '/stats/timeline', icon: BarChart3, label: 'Statistics' },
  { to: '/firewall', icon: Flame, label: 'Firewall' },
  { to: '/blocklists', icon: ShieldBan, label: 'Blocklists' },
  { to: '/settings', icon: Settings, label: 'Settings' },
];

export function MobileTabBar() {
  const [moreOpen, setMoreOpen] = useState(false);
  const { prompts } = useAppStore();
  const location = useLocation();

  const isMoreActive = moreTabs.some((t) => location.pathname.startsWith(t.to.split('/').slice(0, 2).join('/')));

  return (
    <>
      {/* More overflow sheet */}
      {moreOpen && (
        <div
          className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
          onClick={() => setMoreOpen(false)}
        >
          <div
            className="absolute bottom-0 left-0 right-0 bg-card border-t border-border rounded-t-2xl pb-[env(safe-area-inset-bottom,0px)]"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-5 pt-4 pb-2">
              <span className="text-sm font-medium text-muted-foreground">More</span>
              <button
                onClick={() => setMoreOpen(false)}
                className="p-1.5 rounded-lg hover:bg-muted text-muted-foreground"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="px-3 pb-4 space-y-1">
              {moreTabs.map((tab) => (
                <NavLink
                  key={tab.to}
                  to={tab.to}
                  onClick={() => setMoreOpen(false)}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-3 px-4 py-3 rounded-xl text-sm transition-colors',
                      isActive
                        ? 'bg-primary/10 text-primary'
                        : 'text-foreground hover:bg-muted'
                    )
                  }
                >
                  <tab.icon className="h-5 w-5" />
                  <span>{tab.label}</span>
                </NavLink>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Tab bar */}
      <nav className="fixed bottom-0 left-0 right-0 z-40 bg-card/95 backdrop-blur-md border-t border-border md:hidden pb-[env(safe-area-inset-bottom,0px)]">
        <div className="flex items-stretch h-16">
          {primaryTabs.map((tab) => (
            <NavLink
              key={tab.to}
              to={tab.to}
              end={tab.end}
              className={({ isActive }) =>
                cn(
                  'flex-1 flex flex-col items-center justify-center gap-0.5 text-[10px] transition-colors relative',
                  isActive ? 'text-primary' : 'text-muted-foreground'
                )
              }
            >
              <tab.icon className="h-5 w-5" />
              <span>{tab.label}</span>
              {tab.badge && prompts.length > 0 && (
                <span className="absolute top-2 right-1/2 translate-x-3 -translate-y-0.5 min-w-[18px] h-[18px] flex items-center justify-center text-[10px] font-bold bg-destructive text-white rounded-full px-1">
                  {prompts.length}
                </span>
              )}
            </NavLink>
          ))}
          {/* More button */}
          <button
            onClick={() => setMoreOpen(true)}
            className={cn(
              'flex-1 flex flex-col items-center justify-center gap-0.5 text-[10px] transition-colors',
              isMoreActive ? 'text-primary' : 'text-muted-foreground'
            )}
          >
            <MoreHorizontal className="h-5 w-5" />
            <span>More</span>
          </button>
        </div>
      </nav>
    </>
  );
}
