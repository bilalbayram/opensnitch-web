type WSHandler<T = unknown> = (event: WSEvent<T>) => void;

export interface WSEvent<T = unknown> {
  type: string;
  payload: T;
}

class WebSocketClient {
  private ws: WebSocket | null = null;
  private handlers: Map<string, Set<WSHandler<unknown>>> = new Map();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private url: string;

  constructor() {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    this.url = `${proto}//${window.location.host}/api/v1/ws`;
  }

  connect() {
    const token = localStorage.getItem('token');
    if (!token) return;

    try {
      this.ws = new WebSocket(`${this.url}?token=${token}`);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      console.log('[ws] Connected');
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      }
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSEvent = JSON.parse(event.data);
        this.emit(msg.type, msg);
      } catch (e) {
        console.error('[ws] Failed to parse message:', e);
      }
    };

    this.ws.onclose = () => {
      console.log('[ws] Disconnected');
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, 3000);
  }

  on<T = unknown>(event: string, handler: WSHandler<T>) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler as WSHandler<unknown>);
    return () => {
      this.handlers.get(event)?.delete(handler as WSHandler<unknown>);
    };
  }

  private emit(event: string, data: WSEvent<unknown>) {
    // Call specific handlers
    this.handlers.get(event)?.forEach((h) => h(data));
    // Call wildcard handlers
    this.handlers.get('*')?.forEach((h) => h(data));
  }

  send(type: string, payload: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload }));
    }
  }

  get connected() {
    return this.ws?.readyState === WebSocket.OPEN;
  }
}

export const wsClient = new WebSocketClient();
