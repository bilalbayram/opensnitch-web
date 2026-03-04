import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MainLayout } from '@/components/layout/main-layout';
import LoginPage from '@/pages/login';
import DashboardPage from '@/pages/dashboard';
import ConnectionsPage from '@/pages/connections';
import RulesPage from '@/pages/rules';
import NodesPage from '@/pages/nodes';
import StatsPage from '@/pages/stats';
import FirewallPage from '@/pages/firewall';
import AlertsPage from '@/pages/alerts';
import BlocklistsPage from '@/pages/blocklists';


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
            <Route path="/rules" element={<RulesPage />} />
            <Route path="/blocklists" element={<BlocklistsPage />} />
            <Route path="/nodes" element={<NodesPage />} />
            <Route path="/stats/:table" element={<StatsPage />} />
            <Route path="/firewall" element={<FirewallPage />} />
            <Route path="/alerts" element={<AlertsPage />} />

          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
