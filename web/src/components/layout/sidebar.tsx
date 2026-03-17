import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Network,
  Shield,
  ShieldBan,
  Server,
  Globe,
  Cpu,
  MapPin,
  Hash,
  Users,
  Flame,
  Bell,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAppStore } from '@/stores/app-store';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/connections', icon: Network, label: 'Connections' },
  { to: '/seen-flows', icon: Network, label: 'Seen Flows' },
  { to: '/rules', icon: Shield, label: 'Rules' },
  { to: '/blocklists', icon: ShieldBan, label: 'Blocklists' },
  { to: '/nodes', icon: Server, label: 'Nodes' },
  { to: '/stats/hosts', icon: Globe, label: 'Hosts' },
  { to: '/stats/processes', icon: Cpu, label: 'Processes' },
  { to: '/stats/addresses', icon: MapPin, label: 'Addresses' },
  { to: '/stats/ports', icon: Hash, label: 'Ports' },
  { to: '/stats/users', icon: Users, label: 'Users' },
  { to: '/firewall', icon: Flame, label: 'Firewall' },
  { to: '/alerts', icon: Bell, label: 'Alerts' },
];

export function Sidebar() {
  const { nodesOnline, prompts } = useAppStore();

  return (
    <aside className="fixed left-0 top-0 bottom-0 w-56 bg-sidebar border-r border-border flex flex-col z-40">
      <div className="px-4 py-5 border-b border-border">
        <div className="flex items-center gap-2">
          <Shield className="h-6 w-6 text-primary" />
          <span className="font-bold text-lg text-foreground">OpenSnitch</span>
        </div>
        <div className="text-xs text-muted-foreground mt-1">Web UI</div>
      </div>

      <nav className="flex-1 py-2 overflow-y-auto">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 px-4 py-2.5 text-sm transition-colors',
                isActive
                  ? 'bg-sidebar-active text-foreground border-r-2 border-primary'
                  : 'text-sidebar-foreground hover:bg-sidebar-active/50 hover:text-foreground'
              )
            }
          >
            <item.icon className="h-4 w-4" />
            <span>{item.label}</span>
            {item.label === 'Nodes' && nodesOnline.size > 0 && (
              <span className="ml-auto text-xs bg-success/20 text-success px-1.5 py-0.5 rounded-full">
                {nodesOnline.size}
              </span>
            )}
            {item.label === 'Alerts' && prompts.length > 0 && (
              <span className="ml-auto text-xs bg-destructive/20 text-destructive px-1.5 py-0.5 rounded-full">
                {prompts.length}
              </span>
            )}
          </NavLink>
        ))}
      </nav>

      <div className="px-4 py-3 border-t border-border text-xs text-muted-foreground">
        {nodesOnline.size} node{nodesOnline.size !== 1 ? 's' : ''} online
      </div>
    </aside>
  );
}
