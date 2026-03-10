package handlers

import (
	"fmt"
	"net/http"
)

func (h *Handler) IndexPage(w http.ResponseWriter, r *http.Request) {
	if wantsJSON(r) {
		h.Health(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>LocOll — Lab Portal</title>
	<script src="https://unpkg.com/htmx.org@2.0.4"></script>
	<script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
	<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js"></script>
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.css">
	<link rel="stylesheet" href="/static/style.css">
</head>
<body>
	<!-- SSE Listener -->
	<div id="sse-listener" hx-ext="sse" sse-connect="/events" style="display:none">
		<div sse-swap="message" hx-swap="none"></div>
	</div>

	<header class="top-bar">
		<h1 class="logo">&#128300; LocOll</h1>
		<nav x-data="{ tab: 'server' }">
			<button :class="{ active: tab === 'server' }" @click="tab = 'server'">Server</button>
			<button :class="{ active: tab === 'containers' }" @click="tab = 'containers'">Containers</button>
			<button :class="{ active: tab === 'services' }" @click="tab = 'services'">Services</button>
			<button :class="{ active: tab === 'analytics' }" @click="tab = 'analytics'">Analytics</button>
			<button :class="{ active: tab === 'ai' }" @click="tab = 'ai'">AI</button>
		</nav>
	</header>

	<main x-data="{ tab: 'server' }" @keydown.window="
		if ($event.key === '1') tab = 'server';
		if ($event.key === '2') tab = 'containers';
		if ($event.key === '3') tab = 'services';
		if ($event.key === '4') tab = 'analytics';
		if ($event.key === '5') tab = 'ai';
	">
		<!-- Sync nav and main tabs -->
		<template x-effect="
			document.querySelectorAll('nav button').forEach((btn, i) => {
				const tabs = ['server','containers','services','analytics','ai'];
				btn.classList.toggle('active', tabs[i] === tab);
				btn.onclick = () => tab = tabs[i];
			});
		"></template>

		<!-- Server Section -->
		<section x-show="tab === 'server'" x-cloak>
			<h2>Server Metrics</h2>
			<div id="server-metrics"
				hx-get="/fragments/server"
				hx-trigger="load"
				hx-swap="innerHTML">
				<div class="loading">Loading metrics...</div>
			</div>
		</section>

		<!-- Containers Section -->
		<section x-show="tab === 'containers'" x-cloak>
			<h2>Containers</h2>
			<div id="containers-list"
				hx-get="/fragments/containers"
				hx-trigger="load"
				hx-swap="innerHTML">
				<div class="loading">Loading containers...</div>
			</div>
		</section>

		<!-- Services Section -->
		<section x-show="tab === 'services'" x-cloak>
			<h2>Service Health</h2>
			<div id="services-grid"
				hx-get="/fragments/services"
				hx-trigger="load"
				hx-swap="innerHTML">
				<div class="loading">Checking services...</div>
			</div>
		</section>

		<!-- Analytics Section (DuckDB WASM) -->
		<section x-show="tab === 'analytics'" x-cloak x-data="analyticsApp()" x-effect="if (tab === 'analytics' && !initialized) init()">
			<h2>Analytics (DuckDB WASM)</h2>
			<div class="analytics-panel">
				<div x-show="rowCount > 0" class="loaded-info" x-text="'Loaded ' + rowCount + ' rows (last 30 days)'"></div>
				<textarea x-model="query" rows="5" class="sql-input" placeholder="Enter SQL query..."></textarea>
				<button class="btn btn-primary" @click="runQuery()" :disabled="loading">
					<span x-show="!loading">Run Query</span>
					<span x-show="loading">Running...</span>
				</button>
				<div x-show="error" class="error" x-text="error"></div>
				<div x-show="results.length > 0" class="results-table">
					<table>
						<thead>
							<tr>
								<template x-for="col in columns"><th x-text="col"></th></template>
							</tr>
						</thead>
						<tbody>
							<template x-for="row in results">
								<tr>
									<template x-for="col in columns">
										<td x-text="row[col]"></td>
									</template>
								</tr>
							</template>
						</tbody>
					</table>
				</div>
			</div>
		</section>

		<!-- AI Section -->
		<section x-show="tab === 'ai'" x-cloak>
			<h2>AI Analysis</h2>
			<div class="ai-panel" x-data="aiApp()">
				<div class="ai-form">
					<select x-model="containerId" class="select-container">
						<option value="">Select container...</option>
					</select>
					<select x-model="model" class="select-model">
						<option value="mistral:latest">mistral</option>
						<option value="tinyllama:latest">tinyllama</option>
					</select>
					<button class="btn btn-primary"
						hx-post="/ai/analyze"
						hx-target="#ai-result"
						hx-swap="innerHTML"
						hx-indicator="#ai-spinner"
						:disabled="!containerId"
						@click="setFormData($el)">
						Analyze
					</button>
				</div>
				<div id="ai-spinner" class="htmx-indicator">Analyzing...</div>
				<div id="ai-result"></div>
			</div>
		</section>
	</main>

	<!-- Modal Overlay -->
	<div x-data x-show="$store.modal.open"
		x-transition:enter="transition ease-out duration-200"
		x-transition:enter-start="opacity-0"
		x-transition:enter-end="opacity-100"
		x-transition:leave="transition ease-in duration-150"
		x-transition:leave-start="opacity-100"
		x-transition:leave-end="opacity-0"
		@click.self="$store.modal.close()"
		class="modal-backdrop" x-cloak>
		<div class="modal-content" @click.stop>
			<div class="modal-header">
				<span x-text="($store.modal.type === 'logs' ? '\ud83d\udccb Logs: ' : '\u2328\ufe0f Terminal: ') + $store.modal.containerName"></span>
				<button @click="$store.modal.close()" class="modal-close">&times;</button>
			</div>
			<div class="modal-body">
				<template x-if="$store.modal.open && $store.modal.type === 'logs'">
					<div x-data="logsPanel()" x-init="init($store.modal.containerId)" class="logs-stream"></div>
				</template>
				<template x-if="$store.modal.open && $store.modal.type === 'terminal'">
					<div x-data="terminalPanel()" x-init="init($store.modal.containerId)" class="terminal-mount"></div>
				</template>
			</div>
		</div>
	</div>

	<!-- Scripts -->
	<script src="/static/wasm_exec.js"></script>
	<script>
		// Go WASM - lazy load
		(async function() {
			try {
				const go = new Go();
				const result = await WebAssembly.instantiateStreaming(fetch('/static/app.wasm'), go.importObject);
				go.run(result.instance);
				console.log('Go WASM loaded');
			} catch(e) {
				console.warn('Go WASM not available:', e.message);
			}
		})();

		// === Alpine Stores ===
		document.addEventListener('alpine:init', () => {
			Alpine.store('modal', {
				open: false,
				type: null,
				containerId: null,
				containerName: null,
				show(type, id, name) {
					this.type = type;
					this.containerId = id;
					this.containerName = name;
					this.open = true;
				},
				close() {
					this.open = false;
					setTimeout(() => {
						this.type = null;
						this.containerId = null;
						this.containerName = null;
					}, 200);
				}
			});

			Alpine.store('focus', {
				containerId: null,
				containerName: null,
				set(id, name) {
					this.containerId = id;
					this.containerName = name;
				}
			});
		});

		// === Global Hotkeys ===
		document.addEventListener('keydown', (e) => {
			if (e.key === 'Escape' && Alpine.store('modal').open) {
				Alpine.store('modal').close();
				return;
			}
			const tag = document.activeElement?.tagName;
			if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
			if (Alpine.store('modal').open) return;

			const { containerId, containerName } = Alpine.store('focus');
			if (!containerId) return;

			if (e.key === 'l' || e.key === 'L') {
				e.preventDefault();
				Alpine.store('modal').show('logs', containerId, containerName);
			}
			if (e.key === 't' || e.key === 'T') {
				e.preventDefault();
				Alpine.store('modal').show('terminal', containerId, containerName);
			}
		});

		// === Logs Panel (SSE) ===
		function logsPanel() {
			return {
				es: null,
				init(containerId) {
					const el = this.$el;
					this.es = new EventSource('/containers/' + containerId + '/logs');
					this.es.onmessage = (e) => {
						const line = document.createElement('div');
						line.className = 'log-line';
						line.textContent = e.data;
						el.appendChild(line);
						el.scrollTop = el.scrollHeight;
					};
					this.es.onerror = () => {
						const line = document.createElement('div');
						line.className = 'log-error';
						line.textContent = '[Connection closed]';
						el.appendChild(line);
						this.es.close();
					};
				},
				destroy() {
					if (this.es) { this.es.close(); this.es = null; }
				}
			};
		}

		// === Terminal Panel (WebSocket + xterm.js) ===
		function terminalPanel() {
			return {
				term: null,
				ws: null,
				async init(containerId) {
					const { Terminal } = await import('https://cdn.jsdelivr.net/npm/xterm@5.3.0/+esm');
					const { FitAddon } = await import('https://cdn.jsdelivr.net/npm/xterm-addon-fit@0.8.0/+esm');
					const term = new Terminal({ cursorBlink: true, theme: { background: '#1a1a2e' } });
					const fitAddon = new FitAddon();
					term.loadAddon(fitAddon);
					term.open(this.$el);
					fitAddon.fit();

					const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
					const ws = new WebSocket(proto + '//' + location.host + '/terminal/' + containerId);
					ws.binaryType = 'arraybuffer';
					ws.onmessage = (e) => term.write(new Uint8Array(e.data));
					term.onData(data => { if (ws.readyState === WebSocket.OPEN) ws.send(data); });
					ws.onclose = () => term.write('\r\n[Connection closed]');

					this.term = term;
					this.ws = ws;
				},
				destroy() {
					if (this.ws) { this.ws.close(); this.ws = null; }
					if (this.term) { this.term.dispose(); this.term = null; }
				}
			};
		}

		// === AI App ===
		function aiApp() {
			return {
				containerId: '',
				model: 'mistral:latest',
				init() {
					fetch('/api/v1/containers', { headers: { 'Accept': 'application/json' } })
						.then(r => r.json())
						.then(containers => {
							const sel = this.$el.querySelector('.select-container');
							containers.forEach(c => {
								const opt = document.createElement('option');
								opt.value = c.id;
								opt.textContent = c.name;
								sel.appendChild(opt);
							});
						});
				},
				setFormData(btn) {
					const form = btn.closest('.ai-form');
					let input = form.querySelector('input[name=container_id]');
					if (!input) {
						input = document.createElement('input');
						input.type = 'hidden'; input.name = 'container_id';
						form.appendChild(input);
					}
					input.value = this.containerId;
					let mInput = form.querySelector('input[name=model]');
					if (!mInput) {
						mInput = document.createElement('input');
						mInput.type = 'hidden'; mInput.name = 'model';
						form.appendChild(mInput);
					}
					mInput.value = this.model;
				}
			};
		}

		// === Analytics App (DuckDB WASM) ===
		function analyticsApp() {
			return {
				db: null,
				conn: null,
				initialized: false,
				loading: false,
				error: '',
				rowCount: 0,
				query: "SELECT\n  strftime(to_timestamp(ts), '%Y-%m-%d %H:%M') as time,\n  round(cpu_pct, 1) as cpu,\n  round(ram_used_mb / 1024.0, 2) as ram_gb\nFROM metrics\nORDER BY ts DESC\nLIMIT 20",
				columns: [],
				results: [],
				async init() {
					if (this.initialized) return;
					try {
						this.loading = true;
						this.error = '';
						const duckdb = await import('https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.28.0/+esm');
						const JSDELIVR_BUNDLES = duckdb.getJsDelivrBundles();
						const bundle = await duckdb.selectBundle(JSDELIVR_BUNDLES);
						const workerUrl = bundle.mainWorker;
						const workerScript = await fetch(workerUrl).then(r => r.text());
						const blob = new Blob([workerScript], { type: 'application/javascript' });
						const worker = new Worker(URL.createObjectURL(blob));
						const logger = new duckdb.ConsoleLogger();
						this.db = new duckdb.AsyncDuckDB(logger, worker);
						await this.db.instantiate(bundle.mainModule);
						this.conn = await this.db.connect();

						// Load metrics via registerFileText + insertJSONFromPath
						const resp = await fetch('/metrics/history?hours=720&limit=100000');
						const jsonText = await resp.text();
						const data = JSON.parse(jsonText);
						if (data && data.length > 0) {
							await this.db.registerFileText('metrics.json', jsonText);
							await this.conn.insertJSONFromPath('metrics.json', {
								name: 'metrics',
								schema: 'main',
								create: true
							});
							const countResult = await this.conn.query("SELECT count(*) as cnt FROM metrics");
							this.rowCount = countResult.toArray()[0].cnt;
						} else {
							this.error = 'No metrics data available yet.';
						}
						this.initialized = true;
						this.loading = false;
					} catch(e) {
						this.error = 'Failed to load DuckDB: ' + e.message;
						this.loading = false;
					}
				},
				async runQuery() {
					if (!this.conn) await this.init();
					if (!this.conn) return;
					this.error = '';
					this.loading = true;
					try {
						const result = await this.conn.query(this.query);
						this.columns = result.schema.fields.map(f => f.name);
						this.results = result.toArray().map(row => {
							const obj = {};
							this.columns.forEach(col => obj[col] = row[col]);
							return obj;
						});
					} catch(e) {
						this.error = e.message;
						this.results = [];
					}
					this.loading = false;
				}
			};
		}
	</script>
</body>
</html>`)
}
