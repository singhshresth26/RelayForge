export interface Application {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  created_at: string;
}

export interface Endpoint {
  id: string;
  app_id: string;
  url: string;
  description: string;
  is_active: boolean;
  max_retries: number;
  timeout_ms: number;
  rate_limit_rps: number;
  subscriptions: string[];
  created_at: string;
}

export interface EventItem {
  id: string;
  app_id: string;
  idempotency_key?: string;
  event_type: string;
  payload: Record<string, any>;
  metadata: Record<string, any>;
  created_at: string;
}

export interface Delivery {
  id: string;
  event_id: string;
  endpoint_id: string;
  status: 'PENDING' | 'PROCESSING' | 'DELIVERED' | 'FAILED' | 'RETRYING';
  current_attempt: number;
  max_attempts: number;
  next_retry_at?: string;
  delivered_at?: string;
  created_at: string;
  endpoint_url?: string;
  event_type?: string;
}

export interface DeliveryAttempt {
  id: number;
  delivery_id: string;
  attempt_number: number;
  status_code?: number;
  latency_ms: number;
  response_body?: string;
  error_message?: string;
  executed_at: string;
}

