import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from '@/components/layout/main-layout';
import LoginPage from '@/pages/login';
import DashboardPage from '@/pages/dashboard';
import ConnectionsPage from '@/pages/connections';
import SeenFlowsPage from '@/pages/seen-flows';
import RulesPage from '@/pages/rules';
import TemplatesPage from '@/pages/templates';
import NodesPage from '@/pages/nodes';
import StatsPage from '@/pages/stats';
import FirewallPage from '@/pages/firewall';
import AlertsPage from '@/pages/alerts';
import BlocklistsPage from '@/pages/blocklists';
import SettingsPage from '@/pages/settings';
import DNSPage from '@/pages/dns';


const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<MainLayout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/connections" element={<ConnectionsPage />} />
            <Route path="/seen-flows" element={<SeenFlowsPage />} />
            <Route path="/dns" element={<DNSPage />} />
            <Route path="/rules" element={<RulesPage />} />
            <Route path="/templates" element={<TemplatesPage />} />
            <Route path="/blocklists" element={<BlocklistsPage />} />
            <Route path="/nodes" element={<NodesPage />} />
            <Route path="/stats" element={<Navigate to="/stats/hosts" replace />} />
            <Route path="/stats/:table" element={<StatsPage />} />
            <Route path="/firewall" element={<FirewallPage />} />
            <Route path="/alerts" element={<AlertsPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
