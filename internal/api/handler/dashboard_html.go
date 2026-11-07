package handler

import (
	"net/http"
)

// DashboardHTML serves an embedded, zero-dependency, full-featured developer console
const DashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>RelayForge Dashboard | Developer Console</title>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen font-sans">
  <div class="border-b border-slate-800 bg-slate-900/80 backdrop-blur px-6 py-4 flex items-center justify-between sticky top-0 z-50">
    <div class="flex items-center space-x-3">
      <div class="w-8 h-8 rounded-lg bg-emerald-500 text-slate-950 font-bold flex items-center justify-center">RF</div>
      <div>
        <h1 class="font-bold text-lg leading-tight tracking-tight">RelayForge</h1>
        <p class="text-xs text-slate-400">Distributed Webhook Infrastructure</p>
      </div>
    </div>
    <div class="flex items-center space-x-4">
      <input id="apiKeyInput" type="password" placeholder="Enter API Key (rf_live_...)" class="bg-slate-800 border border-slate-700 text-xs rounded px-3 py-1.5 focus:outline-none focus:border-emerald-500 w-56 text-slate-200" />
      <button onclick="saveApiKey()" class="text-xs bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-3 py-1.5 rounded transition">Connect</button>
      <select id="appSelect" onchange="onAppChange()" class="bg-slate-800 border border-slate-700 text-xs rounded px-3 py-1.5 text-slate-200">
        <option value="">No Apps Loaded</option>
      </select>
    </div>
  </div>

  <div id="toast" class="hidden fixed bottom-5 right-5 z-50 bg-slate-800 border border-emerald-500 text-emerald-400 px-4 py-2.5 rounded-lg text-sm shadow-xl"></div>

  <div class="max-w-7xl mx-auto p-6 space-y-6">
    <!-- Stat Cards -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <span class="text-xs text-slate-400">Transactional Outbox</span>
        <p id="statEvents" class="text-2xl font-bold text-slate-100 mt-1">0</p>
        <span class="text-xs text-emerald-400 mt-2 block">ACID Atomicity Guaranteed</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <span class="text-xs text-slate-400">Active Endpoints</span>
        <p id="statEndpoints" class="text-2xl font-bold text-slate-100 mt-1">0</p>
        <span class="text-xs text-emerald-400 mt-2 block">SSRF Guard & HMAC Signed</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <span class="text-xs text-slate-400">Deliveries Processed</span>
        <p id="statDeliveries" class="text-2xl font-bold text-slate-100 mt-1">0</p>
        <span class="text-xs text-blue-400 mt-2 block">Worker Pool Execution</span>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
        <span class="text-xs text-slate-400">System Health</span>
        <p class="text-2xl font-bold text-emerald-400 mt-1">OPERATIONAL</p>
        <span class="text-xs text-slate-400 mt-2 block">Postgres + Redis + RabbitMQ</span>
      </div>
    </div>

    <!-- Actions & Simulator -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <div class="bg-slate-900/60 border border-slate-800 rounded-xl p-6 space-y-4">
        <h2 class="text-base font-semibold">Event Ingestion Simulator</h2>
        <div>
          <label class="text-xs text-slate-400 block mb-1">Event Type</label>
          <input id="simType" type="text" value="order.completed" class="bg-slate-950 border border-slate-800 text-sm rounded px-3 py-1.5 w-full text-slate-200" />
        </div>
        <div>
          <label class="text-xs text-slate-400 block mb-1">Idempotency Key</label>
          <input id="simIdemp" type="text" placeholder="idemp_1001" class="bg-slate-950 border border-slate-800 text-sm rounded px-3 py-1.5 w-full text-slate-200" />
        </div>
        <div>
          <label class="text-xs text-slate-400 block mb-1">Payload (JSON)</label>
          <textarea id="simPayload" rows="3" class="bg-slate-950 font-mono text-xs border border-slate-800 rounded p-2.5 w-full text-slate-200">{"order_id": "ord_9981", "amount": 125.00}</textarea>
        </div>
        <button onclick="publishEvent()" class="bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-4 py-2 rounded text-sm transition">Publish Event (HTTP 202)</button>
      </div>

      <div class="bg-slate-900/60 border border-slate-800 rounded-xl p-6 space-y-4">
        <h2 class="text-base font-semibold">Register Webhook Endpoint</h2>
        <div>
          <label class="text-xs text-slate-400 block mb-1">Webhook URL</label>
          <input id="newEpUrl" type="url" placeholder="https://webhook.site/..." class="bg-slate-950 border border-slate-800 text-sm rounded px-3 py-1.5 w-full text-slate-200" />
        </div>
        <div>
          <label class="text-xs text-slate-400 block mb-1">Event Subscriptions (comma separated)</label>
          <input id="newEpSubs" type="text" value="order.completed, *" class="bg-slate-950 border border-slate-800 text-sm rounded px-3 py-1.5 w-full text-slate-200" />
        </div>
        <button onclick="createEndpoint()" class="bg-emerald-600 hover:bg-emerald-500 text-slate-950 font-semibold px-4 py-2 rounded text-sm transition">Add Endpoint</button>
      </div>
    </div>

    <!-- Live Deliveries Table -->
    <div class="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
      <div class="p-4 border-b border-slate-800 flex justify-between items-center">
        <h3 class="font-semibold text-sm">Recent Webhook Deliveries</h3>
        <button onclick="refreshData()" class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded border border-slate-700">Refresh Data</button>
      </div>
      <table class="w-full text-left text-sm">
        <thead class="bg-slate-950 text-slate-400 text-xs border-b border-slate-800 uppercase">
          <tr>
            <th class="px-6 py-3">Delivery ID</th>
            <th class="px-6 py-3">Status</th>
            <th class="px-6 py-3">Attempts</th>
            <th class="px-6 py-3">Created</th>
            <th class="px-6 py-3">Actions</th>
          </tr>
        </thead>
        <tbody id="deliveriesBody" class="divide-y divide-slate-800">
          <tr><td colspan="5" class="px-6 py-6 text-center text-slate-500">Connect with an API Key to view live deliveries.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <script>
    let apiKey = localStorage.getItem('rf_api_key') || '';
    if (apiKey) document.getElementById('apiKeyInput').value = apiKey;

    function toast(msg) {
      const el = document.getElementById('toast');
      el.innerText = msg;
      el.classList.remove('hidden');
      setTimeout(() => el.classList.add('hidden'), 4000);
    }

    function saveApiKey() {
      apiKey = document.getElementById('apiKeyInput').value.trim();
      localStorage.setItem('rf_api_key', apiKey);
      toast('API Key saved!');
      loadApps();
    }

    async function api(path, opts = {}) {
      opts.headers = opts.headers || {};
      opts.headers['Content-Type'] = 'application/json';
      if (apiKey) opts.headers['Authorization'] = 'Bearer ' + apiKey;
      const res = await fetch('/api/v1' + path, opts);
      const json = await res.json();
      if (!res.ok || !json.success) throw new Error(json.error?.message || 'Request failed');
      return json.data;
    }

    async function loadApps() {
      if (!apiKey) return;
      try {
        const apps = await api('/apps');
        const sel = document.getElementById('appSelect');
        sel.innerHTML = '';
        apps.forEach(a => {
          const opt = document.createElement('option');
          opt.value = a.id;
          opt.innerText = a.name;
          sel.appendChild(opt);
        });
        if (apps.length > 0) refreshData();
      } catch (e) {
        toast('Failed to load apps: ' + e.message);
      }
    }

    function onAppChange() { refreshData(); }

    async function refreshData() {
      const appId = document.getElementById('appSelect').value;
      if (!appId) return;
      try {
        const [eps, evs, dels] = await Promise.all([
          api('/apps/' + appId + '/endpoints').catch(() => []),
          api('/apps/' + appId + '/events').catch(() => []),
          api('/apps/' + appId + '/deliveries').catch(() => [])
        ]);

        document.getElementById('statEndpoints').innerText = eps.length;
        document.getElementById('statEvents').innerText = evs.length;
        document.getElementById('statDeliveries').innerText = dels.length;

        const body = document.getElementById('deliveriesBody');
        if (dels.length === 0) {
          body.innerHTML = '<tr><td colspan="5" class="px-6 py-6 text-center text-slate-500">No deliveries recorded yet.</td></tr>';
          return;
        }

        body.innerHTML = dels.map(function(d) {
          var badgeColor = d.status === 'DELIVERED' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'bg-amber-500/10 text-amber-400 border-amber-500/30';
          return '<tr class="hover:bg-slate-800/40">' +
            '<td class="px-6 py-3 font-mono text-xs">' + d.id + '</td>' +
            '<td class="px-6 py-3"><span class="text-xs px-2.5 py-0.5 rounded-full border ' + badgeColor + '">' + d.status + '</span></td>' +
            '<td class="px-6 py-3 text-xs">' + d.current_attempt + ' / ' + d.max_attempts + '</td>' +
            '<td class="px-6 py-3 text-xs text-slate-400">' + new Date(d.created_at).toLocaleTimeString() + '</td>' +
            '<td class="px-6 py-3"><button onclick="replayDelivery(\'' + d.id + '\')" class="text-xs bg-slate-800 hover:bg-slate-700 px-2 py-1 rounded border border-slate-700">Replay</button></td>' +
          '</tr>';
        }).join('');
      } catch (e) {
        toast('Refresh failed: ' + e.message);
      }
    }

    async function publishEvent() {
      const appId = document.getElementById('appSelect').value;
      if (!appId) return toast('Select an application first');
      const eventType = document.getElementById('simType').value.trim();
      const idempKey = document.getElementById('simIdemp').value.trim();
      const payload = JSON.parse(document.getElementById('simPayload').value);

      const headers = {};
      if (idempKey) headers['Idempotency-Key'] = idempKey;

      try {
        const res = await api('/apps/' + appId + '/events', {
          method: 'POST',
          headers,
          body: JSON.stringify({ event_type: eventType, payload })
        });
        toast('Event ' + res.status + ': ' + res.event_id);
        refreshData();
      } catch (e) {
        toast('Ingestion error: ' + e.message);
      }
    }

    async function createEndpoint() {
      const appId = document.getElementById('appSelect').value;
      if (!appId) return toast('Select an application first');
      const url = document.getElementById('newEpUrl').value.trim();
      const subs = document.getElementById('newEpSubs').value.split(',').map(s => s.trim()).filter(Boolean);

      try {
        const res = await api('/apps/' + appId + '/endpoints', {
          method: 'POST',
          body: JSON.stringify({ url, description: 'Console Endpoint', subscriptions: subs, max_retries: 5, timeout_ms: 10000 })
        });
        toast('Endpoint created! Secret: ' + res.secret_key);
        document.getElementById('newEpUrl').value = '';
        refreshData();
      } catch (e) {
        toast('Endpoint error: ' + e.message);
      }
    }

    async function replayDelivery(id) {
      const appId = document.getElementById('appSelect').value;
      try {
        await api('/apps/' + appId + '/deliveries/' + id + '/replay', { method: 'POST' });
        toast('Replay initiated!');
        refreshData();
      } catch (e) {
        toast('Replay error: ' + e.message);
      }
    }

    if (apiKey) loadApps();
  </script>
</body>
</html>`

func ServeDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(DashboardHTML))
}
