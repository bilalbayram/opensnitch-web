import { NavLink, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Network,
  Shield,
  ShieldBan,
  Server,
  BarChart3,
  Waypoints,
  Flame,
  Bell,
  Settings,
  Layers,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { AppLogo } from "@/components/app-logo";
import { cn } from "@/lib/utils";
import { useAppStore } from "@/stores/app-store";
import { api } from "@/lib/api";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "Dashboard" },
  { to: "/connections", icon: Network, label: "Connections" },
  { to: "/seen-flows", icon: Network, label: "Seen Flows" },
  { to: "/dns", icon: Waypoints, label: "DNS" },
  { to: "/rules", icon: Shield, label: "Rules" },
  { to: "/templates", icon: Layers, label: "Templates" },
  { to: "/blocklists", icon: ShieldBan, label: "Blocklists" },
  { to: "/nodes", icon: Server, label: "Nodes" },
  {
    to: "/stats/timeline",
    icon: BarChart3,
    label: "Stats",
    matchPrefix: "/stats",
  },
  { to: "/firewall", icon: Flame, label: "Firewall" },
  { to: "/alerts", icon: Bell, label: "Alerts" },
  { to: "/settings", icon: Settings, label: "Settings" },
];

export function Sidebar() {
  const { nodesOnline, prompts } = useAppStore();
  const { pathname } = useLocation();

  const { data: nodes } = useQuery({
    queryKey: ["nodes"],
    queryFn: api.getNodes,
    refetchInterval: 10_000,
  });
  const onlineCount = nodes?.filter((n) => n.online).length ?? nodesOnline.size;

  return (
    <aside className="fixed left-0 top-0 bottom-0 hidden md:flex flex-col z-40 bg-sidebar border-r border-border w-16 lg:w-56 transition-[width] duration-200">
      {/* Brand */}
      <div className="px-3 lg:px-4 py-5 border-b border-border">
        <div className="flex items-center gap-2">
          <AppLogo className="h-6 w-6" />
          <span className="font-bold text-lg text-foreground hidden lg:block">
            OpenSnitch
          </span>
        </div>
        <div className="text-xs text-muted-foreground mt-1 hidden lg:block">
          Web UI
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 py-2 overflow-y-auto">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) => {
              const active =
                isActive ||
                (item.matchPrefix && pathname.startsWith(item.matchPrefix));
              return cn(
                "flex items-center gap-3 px-3 lg:px-4 py-2.5 text-sm transition-colors relative group",
                active
                  ? "bg-sidebar-active text-foreground border-r-2 border-primary"
                  : "text-sidebar-foreground hover:bg-sidebar-active/50 hover:text-foreground",
              );
            }}
          >
            <item.icon className="h-4 w-4 shrink-0 mx-auto lg:mx-0" />
            <span className="hidden lg:inline">{item.label}</span>

            {/* Tooltip for collapsed sidebar */}
            <span className="absolute left-full ml-2 px-2 py-1 rounded-md bg-card border border-border text-xs text-foreground whitespace-nowrap opacity-0 pointer-events-none group-hover:opacity-100 lg:hidden z-50 transition-opacity">
              {item.label}
            </span>

            {/* Badges */}
            {item.label === "Nodes" && onlineCount > 0 && (
              <span className="ml-auto text-xs bg-success/20 text-success px-1.5 py-0.5 rounded-full hidden lg:inline">
                {onlineCount}
              </span>
            )}
            {item.label === "Alerts" && prompts.length > 0 && (
              <>
                <span className="ml-auto text-xs bg-destructive/20 text-destructive px-1.5 py-0.5 rounded-full hidden lg:inline">
                  {prompts.length}
                </span>
                {/* Dot indicator for collapsed mode */}
                <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-destructive rounded-full lg:hidden" />
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      <div className="px-3 lg:px-4 py-3 border-t border-border text-xs text-muted-foreground">
        <span className="hidden lg:inline">
          {onlineCount} node{onlineCount !== 1 ? "s" : ""} online
        </span>
        <span className="lg:hidden text-center block text-[10px]">
          {onlineCount}
        </span>
      </div>
    </aside>
  );
}
