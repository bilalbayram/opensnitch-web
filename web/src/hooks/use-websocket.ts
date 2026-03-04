import { useEffect } from 'react';
import { wsClient } from '@/lib/ws';
import { useAppStore } from '@/stores/app-store';

export function useWebSocket() {
  const { token, setWSConnected, updateStats, addPrompt, removePrompt, addConnection, addNodeOnline, removeNodeOnline } = useAppStore();

  useEffect(() => {
    if (!token) return;

    wsClient.connect();

    const unsubs = [
      wsClient.on('stats_update', (e) => updateStats(e.payload)),
      wsClient.on('connection_event', (e) => addConnection(e.payload)),
      wsClient.on('prompt_request', (e) => addPrompt(e.payload)),
      wsClient.on('prompt_timeout', (e) => removePrompt(e.payload.id)),
      wsClient.on('node_connected', (e) => addNodeOnline(e.payload.addr)),
      wsClient.on('node_disconnected', (e) => removeNodeOnline(e.payload.addr)),
    ];

    const checkInterval = setInterval(() => {
      setWSConnected(wsClient.connected);
    }, 1000);

    return () => {
      unsubs.forEach((u) => u());
      clearInterval(checkInterval);
      wsClient.disconnect();
    };
  }, [token]);
}
