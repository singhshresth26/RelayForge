const API_BASE = '/api/v1';

export class RelayForgeClient {
  private apiKey: string;

  constructor(apiKey: string) {
    this.apiKey = apiKey;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers || {});
    headers.set('Content-Type', 'application/json');
    if (this.apiKey) {
      headers.set('Authorization', `Bearer ${this.apiKey}`);
    }

    const res = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    const json = await res.json();
    if (!res.ok || !json.success) {
      throw new Error(json.error?.message || `HTTP ${res.status}`);
    }

    return json.data;
  }

  // Apps
  async listApps() {
    return this.request<any[]>('/apps');
  }

  async createApp(name: string, description: string) {
    return this.request<any>('/apps', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    });
  }

  // Endpoints
  async listEndpoints(appId: string) {
    return this.request<any[]>(`/apps/${appId}/endpoints`);
  }

  async createEndpoint(appId: string, data: { url: string; description: string; subscriptions: string[]; max_retries: number; timeout_ms: number }) {
    return this.request<{ endpoint: any; secret_key: string }>(`/apps/${appId}/endpoints`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async rotateSecret(appId: string, endpointId: string) {
    return this.request<{ endpoint: any; secret_key: string }>(`/apps/${appId}/endpoints/${endpointId}/rotate-secret`, {
      method: 'POST',
    });
  }

  // Events
  async ingestEvent(appId: string, eventType: string, payload: any, idempotencyKey?: string) {
    const headers: Record<string, string> = {};
    if (idempotencyKey) {
      headers['Idempotency-Key'] = idempotencyKey;
    }

    return this.request<{ event_id: string; status: string; is_duplicate: boolean }>(`/apps/${appId}/events`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        event_type: eventType,
        payload,
      }),
    });
  }

  async listEvents(appId: string) {
    return this.request<any[]>(`/apps/${appId}/events`);
  }

  // Deliveries
  async listDeliveries(appId: string) {
    return this.request<any[]>(`/apps/${appId}/deliveries`);
  }

  async listAttempts(appId: string, deliveryId: string) {
    return this.request<any[]>(`/apps/${appId}/deliveries/${deliveryId}/attempts`);
  }

  async replayDelivery(appId: string, deliveryId: string) {
    return this.request<any>(`/apps/${appId}/deliveries/${deliveryId}/replay`, {
      method: 'POST',
    });
  }
}

