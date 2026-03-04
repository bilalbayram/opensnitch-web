import { Outlet, Navigate } from 'react-router-dom';
import { Sidebar } from './sidebar';
import { Header } from './header';
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
      <div className="ml-56">
        <Header />
        <main className="p-6">
          <Outlet />
        </main>
      </div>
      <PromptOverlay />
    </div>
  );
}
