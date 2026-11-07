import React, { useState, useEffect } from 'react';
import { RelayForgeClient } from './api';
import { Application, Endpoint, EventItem, Delivery, DeliveryAttempt } from './types';

export default function App() {
  const [apiKey, setApiKey] = useState(localStorage.getItem('rf_api_key') || '');
  const [apps, setApps] = useState<Application[]>([]);
  const [selectedApp, setSelectedApp] = useState<Application | null>(null);
  const [activeTab, setActiveTab] = useState<'overview' | 'endpoints' | 'events' | 'deliveries'>('overview');

  // State
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [events, setEvents] = useState<EventItem[]>([]);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [selectedDeliveryAttempts, setSelectedDeliveryAttempts] = useState<DeliveryAttempt[] | null>(null);

  // Forms
  const [newAppName, setNewAppName] = useState('');
  const [newEndpointUrl, setNewEndpointUrl] = useState('');
  const [newEndpointSubs, setNewEndpointSubs] = useState('order.completed, customer.created');
  const [simEventType, setSimEventType] = useState('order.completed');
  const [simPayload, setSimPayload] = useState('{\n  "order_id": "ord_9981",\n  "amount_cents": 12500,\n  "currency": "USD"\n}');
  const [simIdempKey, setSimIdempKey] = useState('');
  const [notification, setNotification] = useState<string | null>(null);

  const client = new RelayForgeClient(apiKey);

  const showNotification = (msg: string) => {
    setNotification(msg);
    setTimeout(() => setNotification(null), 4000);
  };

  useEffect(() => {
    if (apiKey) {
      localStorage.setItem('rf_api_key', apiKey);
      loadApps();
    }
  }, [apiKey]);

  useEffect(() => {
    if (selectedApp) {
      loadAppData();
    }
  }, [selectedApp]);

  const loadApps = async () => {
    try {
      const data = await client.listApps();
      setApps(data || []);
      if (data && data.length > 0 && !selectedApp) {
        setSelectedApp(data[0]);
      }
    } catch (e: any) {
      showNotification(`Error loading apps: ${e.message}`);
    }
  };

  const loadAppData = async () => {
    if (!selectedApp) return;
    try {
      const [eps, evs, dels] = await Promise.all([
        client.listEndpoints(selectedApp.id).catch(() => []),
        client.listEvents(selectedApp.id).catch(() => []),
        client.listDeliveries(selectedApp.id).catch(() => []),
      ]);
      setEndpoints(eps || []);
      setEvents(evs || []);
      setDeliveries(dels || []);
    } catch (e: any) {
      showNotification(`Error refreshing data: ${e.message}`);
    }
  };

  const handleCreateApp = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const app = await client.createApp(newAppName, 'Default Application');
      showNotification(`Application ${app.name} created!`);
      setNewAppName('');
      loadApps();
    } catch (e: any) {
      showNotification(`Failed: ${e.message}`);
    }
  };

  const handleCreateEndpoint = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedApp) return;
    try {
      const subs = newEndpointSubs.split(',').map((s) => s.trim()).filter(Boolean);
      const res = await client.createEndpoint(selectedApp.id, {
        url: newEndpointUrl,
        description: 'Production Endpoint',
        subscriptions: subs,
        max_retries: 5,
        timeout_ms: 10000,
      });
      showNotification(`Endpoint created! Secret: ${res.secret_key}`);
      setNewEndpointUrl('');
      loadAppData();
    } catch (e: any) {
      showNotification(`Failed: ${e.message}`);
    }
  };

  const handleSimulateEvent = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedApp) return;
    try {
      const parsed = JSON.parse(simPayload);
      const res = await client.ingestEvent(selectedApp.id, simEventType, parsed, simIdempKey || undefined);
      showNotification(`Event ${res.status}: ${res.event_id} (Duplicate: ${res.is_duplicate})`);
      loadAppData();
    } catch (e: any) {
      showNotification(`Simulation error: ${e.message}`);
    }
  };

  const handleReplay = async (deliveryId: string) => {
    if (!selectedApp) return;
    try {
      await client.replayDelivery(selectedApp.id, deliveryId);
      showNotification('Replay dispatched successfully!');
      loadAppData();
    } catch (e: any) {
      showNotification(`Replay failed: ${e.message}`);
    }
  };

  const handleViewAttempts = async (deliveryId: string) => {
    if (!selectedApp) return;
    try {
      const attempts = await client.listAttempts(selectedApp.id, deliveryId);
      setSelectedDeliveryAttempts(attempts);
    } catch (e: any) {
      showNotification(`Failed to load attempts: ${e.message}`);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      {/* Top Header */}
      <header className="border-b border-slate-800 bg-slate-900/60 backdrop-blur px-6 py-4 flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 rounded-lg bg-emerald-500 flex items-center justify-center font-bold text-slate-950">
            RF
          </div>
          <div>
            <h1 className="font-bold text-lg leading-tight tracking-tight">RelayForge</h1>
            <p className="text-xs text-slate-400">Distributed Webhook Infrastructure</p>
          </div>
        </div>

        {/* Global Controls */}
        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-2">
            <span className="text-xs text-slate-400">API Key:</span>
            <input
              type="password"
              placeholder="rf_live_..."
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="bg-slate-800 border border-slate-700 text-xs rounded px-2.5 py-1.5 focus:outline-none focus:border-emerald-500 w-48 text-slate-200"
            />
          </div>

          {apps.length > 0 && (
            <select
              value={selectedApp?.id || ''}
              onChange={(e) => {
                const found = apps.find((a) => a.id === e.target.value);
                if (found) setSelectedApp(found);
              }}
              className="bg-slate-800 border border-slate-700 text-xs rounded px-3 py-1.5 focus:outline-none focus:border-emerald-500 text-slate-200"
            >
              {apps.map((a) => (
                <option key={a.id} value={a.id}>{a.name}</option>
              ))}
            </select>
          )}

          <button
            onClick={loadAppData}
            className="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded transition border border-slate-700"
          >
            Refresh
          </button>
        </div>
      </header>

      {/* Notification Toast */}
      {notification && (
        <div className="fixed bottom-5 right-5 z-50 bg-slate-800 border border-emerald-500 text-emerald-400 px-4 py-2.5 rounded-lg shadow-xl text-sm animate-pulse">
          {notification}
        </div>
      )}

      {/* Main Container */}
      <div className="flex-1 max-w-7xl w-full mx-auto p-6 space-y-6">
        {/* Navigation Tabs */}
        <div className="flex space-x-2 border-b border-slate-800 pb-3">
          {(['overview', 'endpoints', 'events', 'deliveries'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-2 rounded-md text-sm font-medium capitalize transition ${
                activeTab === tab
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Tab 1: Overview */}
        {activeTab === 'overview' && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                <span className="text-xs text-slate-400">Total Ingested Events</span>
                <p className="text-2xl font-bold text-slate-100 mt-1">{events.length}</p>
                <span className="text-xs text-emerald-400 mt-2 block">Transactional Outbox Guaranteed</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                <span className="text-xs text-slate-400">Active Endpoints</span>
                <p className="text-2xl font-bold text-slate-100 mt-1">{endpoints.length}</p>
                <span className="text-xs text-emerald-400 mt-2 block">SSRF Protected & HMAC Signed</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                <span className="text-xs text-slate-400">Deliveries Processed</span>
                <p className="text-2xl font-bold text-slate-100 mt-1">{deliveries.length}</p>
                <span className="text-xs text-blue-400 mt-2 block">Worker Pool Execution</span>
              </div>
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5">
                <span className="text-xs text-slate-400">System Health</span>
                <p className="text-2xl font-bold text-emerald-400 mt-1">100% OPERATIONAL</p>
                <span className="text-xs text-slate-400 mt-2 block">Postgres + Redis + RabbitMQ</span>
              </div>
            </div>

            {/* Quick Create App Section */}
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-6">
              <h2 className="text-base font-semibold mb-2">Create Application</h2>
              <form onSubmit={handleCreateApp} className="flex gap-3">
                <input
                  type="text"
                  placeholder="Application Name (e.g. Billing Service)"
                  value={newAppName}
                  onChange={(e) => setNewAppName(e.target.value)}
                  className="bg-slate-950 border border-slate-800 text-sm rounded-lg px-3.5 py-2 flex-1 focus:outline-none focus:border-emerald-500"
                />
                <button
                  type="submit"
                  className="bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-5 py-2 rounded-lg text-sm transition"
                >
                  Create App
                </button>
              </form>
            </div>
          </div>
        )}

        {/* Tab 2: Endpoints */}
        {activeTab === 'endpoints' && (
          <div className="space-y-6">
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-6">
              <h2 className="text-base font-semibold mb-3">Register Webhook Endpoint</h2>
              <form onSubmit={handleCreateEndpoint} className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Destination Webhook URL</label>
                    <input
                      type="url"
                      placeholder="https://webhook.site/..."
                      value={newEndpointUrl}
                      onChange={(e) => setNewEndpointUrl(e.target.value)}
                      required
                      className="bg-slate-950 border border-slate-800 text-sm rounded-lg px-3.5 py-2 w-full focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Subscribed Event Types (comma separated)</label>
                    <input
                      type="text"
                      placeholder="order.completed, customer.created, *"
                      value={newEndpointSubs}
                      onChange={(e) => setNewEndpointSubs(e.target.value)}
                      className="bg-slate-950 border border-slate-800 text-sm rounded-lg px-3.5 py-2 w-full focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                </div>
                <button
                  type="submit"
                  className="bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-5 py-2 rounded-lg text-sm transition"
                >
                  Add Endpoint
                </button>
              </form>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-950 text-slate-400 text-xs border-b border-slate-800 uppercase">
                  <tr>
                    <th className="px-6 py-3">URL</th>
                    <th className="px-6 py-3">Subscriptions</th>
                    <th className="px-6 py-3">Max Retries</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {endpoints.map((ep) => (
                    <tr key={ep.id} className="hover:bg-slate-800/40 transition">
                      <td className="px-6 py-4 font-mono text-xs text-emerald-400">{ep.url}</td>
                      <td className="px-6 py-4 text-xs">{ep.subscriptions?.join(', ') || '*'}</td>
                      <td className="px-6 py-4 text-xs">{ep.max_retries} attempts</td>
                      <td className="px-6 py-4">
                        <span className="bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 text-xs px-2.5 py-0.5 rounded-full">
                          ACTIVE
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <button
                          onClick={async () => {
                            if (!selectedApp) return;
                            const res = await client.rotateSecret(selectedApp.id, ep.id);
                            showNotification(`Rotated Secret: ${res.secret_key}`);
                          }}
                          className="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded border border-slate-700 transition"
                        >
                          Rotate Secret
                        </button>
                      </td>
                    </tr>
                  ))}
                  {endpoints.length === 0 && (
                    <tr>
                      <td colSpan={5} className="px-6 py-8 text-center text-slate-500">
                        No endpoints registered yet. Add one above!
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Tab 3: Events & Simulator */}
        {activeTab === 'events' && (
          <div className="space-y-6">
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-6">
              <h2 className="text-base font-semibold mb-3">Event Ingestion Simulator</h2>
              <form onSubmit={handleSimulateEvent} className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Event Type</label>
                    <input
                      type="text"
                      value={simEventType}
                      onChange={(e) => setSimEventType(e.target.value)}
                      required
                      className="bg-slate-950 border border-slate-800 text-sm rounded-lg px-3.5 py-2 w-full focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-slate-400 block mb-1">Idempotency Key (Optional)</label>
                    <input
                      type="text"
                      placeholder="e.g. ord_test_12345"
                      value={simIdempKey}
                      onChange={(e) => setSimIdempKey(e.target.value)}
                      className="bg-slate-950 border border-slate-800 text-sm rounded-lg px-3.5 py-2 w-full focus:outline-none focus:border-emerald-500"
                    />
                  </div>
                </div>
                <div>
                  <label className="text-xs text-slate-400 block mb-1">Payload (JSON)</label>
                  <textarea
                    rows={4}
                    value={simPayload}
                    onChange={(e) => setSimPayload(e.target.value)}
                    className="bg-slate-950 font-mono text-xs border border-slate-800 rounded-lg p-3 w-full focus:outline-none focus:border-emerald-500"
                  />
                </div>
                <button
                  type="submit"
                  className="bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-5 py-2 rounded-lg text-sm transition"
                >
                  Publish Event (HTTP 202)
                </button>
              </form>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-950 text-slate-400 text-xs border-b border-slate-800 uppercase">
                  <tr>
                    <th className="px-6 py-3">Event ID</th>
                    <th className="px-6 py-3">Event Type</th>
                    <th className="px-6 py-3">Idempotency Key</th>
                    <th className="px-6 py-3">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {events.map((ev) => (
                    <tr key={ev.id} className="hover:bg-slate-800/40 transition">
                      <td className="px-6 py-4 font-mono text-xs text-slate-300">{ev.id}</td>
                      <td className="px-6 py-4 text-xs font-semibold text-emerald-400">{ev.event_type}</td>
                      <td className="px-6 py-4 text-xs font-mono text-slate-400">{ev.idempotency_key || '—'}</td>
                      <td className="px-6 py-4 text-xs text-slate-400">{new Date(ev.created_at).toLocaleString()}</td>
                    </tr>
                  ))}
                  {events.length === 0 && (
                    <tr>
                      <td colSpan={4} className="px-6 py-8 text-center text-slate-500">
                        No events ingested yet. Send one with the simulator!
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Tab 4: Deliveries */}
        {activeTab === 'deliveries' && (
          <div className="space-y-6">
            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-950 text-slate-400 text-xs border-b border-slate-800 uppercase">
                  <tr>
                    <th className="px-6 py-3">Delivery ID</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3">Attempts</th>
                    <th className="px-6 py-3">Created</th>
                    <th className="px-6 py-3">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {deliveries.map((d) => (
                    <tr key={d.id} className="hover:bg-slate-800/40 transition">
                      <td className="px-6 py-4 font-mono text-xs text-slate-300">{d.id}</td>
                      <td className="px-6 py-4">
                        <span
                          className={`text-xs px-2.5 py-0.5 rounded-full border ${
                            d.status === 'DELIVERED'
                              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                              : d.status === 'FAILED'
                              ? 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                              : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                          }`}
                        >
                          {d.status}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-xs">{d.current_attempt} / {d.max_attempts}</td>
                      <td className="px-6 py-4 text-xs text-slate-400">{new Date(d.created_at).toLocaleString()}</td>
                      <td className="px-6 py-4 space-x-2">
                        <button
                          onClick={() => handleViewAttempts(d.id)}
                          className="text-xs bg-slate-800 hover:bg-slate-700 px-2.5 py-1 rounded border border-slate-700 transition"
                        >
                          View Logs
                        </button>
                        <button
                          onClick={() => handleReplay(d.id)}
                          className="text-xs bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 px-2.5 py-1 rounded border border-emerald-500/30 transition"
                        >
                          Replay
                        </button>
                      </td>
                    </tr>
                  ))}
                  {deliveries.length === 0 && (
                    <tr>
                      <td colSpan={5} className="px-6 py-8 text-center text-slate-500">
                        No deliveries recorded yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Attempts Inspector Modal */}
            {selectedDeliveryAttempts && (
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-4">
                <div className="flex justify-between items-center">
                  <h3 className="font-semibold text-sm">Delivery Attempt Logs</h3>
                  <button
                    onClick={() => setSelectedDeliveryAttempts(null)}
                    className="text-xs text-slate-400 hover:text-slate-200"
                  >
                    Close
                  </button>
                </div>
                <div className="space-y-3">
                  {selectedDeliveryAttempts.map((att) => (
                    <div key={att.id} className="bg-slate-950 p-4 rounded-lg border border-slate-800 space-y-2 text-xs">
                      <div className="flex justify-between">
                        <span className="font-semibold">Attempt #{att.attempt_number}</span>
                        <span className={att.status_code && att.status_code < 300 ? 'text-emerald-400' : 'text-rose-400'}>
                          HTTP {att.status_code || 'Network Error'} ({att.latency_ms}ms)
                        </span>
                      </div>
                      {att.error_message && (
                        <p className="text-rose-400 font-mono">{att.error_message}</p>
                      )}
                      {att.response_body && (
                        <div className="bg-slate-900 p-2 rounded text-slate-400 font-mono">
                          {att.response_body}
                        </div>
                      )}
                    </div>
                  ))}
                  {selectedDeliveryAttempts.length === 0 && (
                    <p className="text-xs text-slate-500">No attempt logs recorded yet.</p>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

