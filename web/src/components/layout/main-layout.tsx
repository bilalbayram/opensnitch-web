import { Outlet, Navigate } from 'react-router-dom';
import { Sidebar } from './sidebar';
import { Header } from './header';
import { MobileTabBar } from './mobile-tab-bar';
import { PromptOverlay } from '@/components/prompt/dialog';
import { useWebSocket } from '@/hooks/use-websocket';
import { useAppStore } from '@/stores/app-store';

export function MainLayout() {
  const { token } = useAppStore();
  useWebSocket();

  if (!token) {
    return <Navigate to="/login" replace />;
  }

  return (
    <div className="min-h-screen">
      <Sidebar />
      <div className="ml-0 md:ml-16 lg:ml-56 transition-[margin] duration-200">
        <Header />
        <main className="p-4 md:p-6 pb-24 md:pb-6">
          <Outlet />
        </main>
      </div>
      <MobileTabBar />
      <PromptOverlay />
    </div>
  );
}
