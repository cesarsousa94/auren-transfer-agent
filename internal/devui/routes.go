package devui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/auren/auren-transfer-agent/internal/runtime"
	"github.com/auren/auren-transfer-agent/internal/server"
)

// MetricsSnapshot is the compact operational view rendered by the local console.
type MetricsSnapshot struct {
	GeneratedAt     time.Time       `json:"generated_at"`
	Runtime         any             `json:"runtime"`
	MediaHub        any             `json:"media_hub"`
	Transfer        any             `json:"transfer"`
	Gateway         any             `json:"gateway"`
	Queue           any             `json:"queue"`
	Download        any             `json:"download"`
	Hardening       any             `json:"hardening"`
	RequestCounters Counters        `json:"request_counters"`
	RecentRequests  []RequestRecord `json:"recent_requests"`
}

// SnapshotFunc builds the current metrics snapshot.
type SnapshotFunc func() MetricsSnapshot

// Options configures local development console routes.
type Options struct {
	Config   Config
	Recorder *Recorder
	Snapshot SnapshotFunc
}

// Routes returns the local console HTML and JSON routes.
func Routes(options Options) []server.RouteDefinition {
	path := normalizeBasePath(options.Config.Path)
	return []server.RouteDefinition{
		{Name: "devui.index", Method: http.MethodGet, Pattern: path, Handler: redirectTo(path + "/metrics")},
		{Name: "devui.metrics", Method: http.MethodGet, Pattern: path + "/metrics", Handler: htmlHandler(options, "metrics")},
		{Name: "devui.requests", Method: http.MethodGet, Pattern: path + "/requests", Handler: htmlHandler(options, "requests")},
		{Name: "devui.api.snapshot", Method: http.MethodGet, Pattern: path + "/api/snapshot", Handler: snapshotHandler(options)},
		{Name: "devui.api.requests", Method: http.MethodGet, Pattern: path + "/api/requests", Handler: requestsHandler(options)},
	}
}

func redirectTo(target string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target, http.StatusFound)
	}
}

func htmlHandler(options Options, page string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		base := normalizeBasePath(options.Config.Path)
		refreshMS := refreshMilliseconds(options.Config.RefreshInterval)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consoleTemplate.Execute(writer, map[string]any{"BasePath": base, "Page": page, "RefreshMS": refreshMS, "Version": runtime.Version})
	}
}

func snapshotHandler(options Options) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		snapshot := MetricsSnapshot{GeneratedAt: time.Now().UTC()}
		if options.Snapshot != nil {
			snapshot = options.Snapshot()
		}
		if options.Recorder != nil {
			snapshot.RequestCounters = options.Recorder.Counters()
			snapshot.RecentRequests = options.Recorder.Snapshot(25)
		}
		writeJSON(writer, snapshot)
	}
}

func requestsHandler(options Options) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limit := 200
		if options.Recorder == nil {
			writeJSON(writer, []RequestRecord{})
			return
		}
		writeJSON(writer, options.Recorder.Snapshot(limit))
	}
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func normalizeBasePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultPath
	}
	path = "/" + strings.Trim(path, "/")
	return path
}

func refreshMilliseconds(value string) int64 {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 2000
	}
	return duration.Milliseconds()
}

var consoleTemplate = template.Must(template.New("console").Parse(consoleHTML))

const consoleHTML = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Auren Transfer Agent Console</title>
  <style>
    :root{color-scheme:dark;--bg:#090d14;--panel:#121927;--panel2:#0f1521;--text:#e7edf7;--muted:#8ea0b8;--line:#253247;--ok:#3ddc97;--warn:#ffd166;--err:#ff6b6b;--brand:#8ab4ff}
    body{margin:0;background:linear-gradient(120deg,#080b12,#111827);font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--text)}
    header{padding:22px 28px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;align-items:center;position:sticky;top:0;background:rgba(9,13,20,.92);backdrop-filter:blur(10px);z-index:2}
    h1{font-size:18px;margin:0}.sub{color:var(--muted);font-size:12px;margin-top:4px}nav a{color:var(--muted);text-decoration:none;margin-left:16px;font-size:14px}nav a.active{color:var(--brand);font-weight:700}
    main{padding:24px 28px}.grid{display:grid;grid-template-columns:repeat(4,minmax(150px,1fr));gap:14px;margin-bottom:18px}.card{background:linear-gradient(180deg,var(--panel),var(--panel2));border:1px solid var(--line);border-radius:16px;padding:16px;box-shadow:0 14px 40px rgba(0,0,0,.22)}.label{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}.value{font-size:28px;font-weight:800;margin-top:8px}.small{font-size:13px;color:var(--muted);margin-top:6px}.ok{color:var(--ok)}.warn{color:var(--warn)}.err{color:var(--err)}
    table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--line);border-radius:16px;overflow:hidden}th,td{padding:10px 12px;border-bottom:1px solid var(--line);font-size:13px;text-align:left;vertical-align:top}th{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.08em;background:#0b111d}tr:last-child td{border-bottom:0}.pill{display:inline-block;padding:3px 8px;border-radius:999px;background:#1b2638;color:var(--muted);font-size:12px}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}.section{margin-top:18px}.pre{white-space:pre-wrap;word-break:break-word;color:var(--muted)}@media(max-width:900px){.grid{grid-template-columns:repeat(2,minmax(120px,1fr))}header{display:block}nav{margin-top:12px}nav a{margin:0 12px 0 0}}@media(max-width:560px){.grid{grid-template-columns:1fr}main{padding:16px}header{padding:16px}}
  </style>
</head>
<body>
<header><div><h1>Auren Transfer Agent Console</h1><div class="sub">{{.Version}} · atualização automática local para desenvolvimento</div></div><nav><a href="{{.BasePath}}/metrics" class="{{if eq .Page "metrics"}}active{{end}}">Métricas</a><a href="{{.BasePath}}/requests" class="{{if eq .Page "requests"}}active{{end}}">Requisições</a><a href="{{.BasePath}}/api/snapshot">JSON</a></nav></header>
<main><div id="app" class="pre">Carregando...</div></main>
<script>
const BASE={{printf "%q" .BasePath}};const PAGE={{printf "%q" .Page}};const REFRESH={{.RefreshMS}};
function el(v){return String(v==null?'':v)}
function num(v){return Number(v||0).toLocaleString('pt-BR')}
function statusClass(s){s=Number(s||0);return s>=500?'err':(s>=400?'warn':'ok')}
function escapeHtml(x){return String(x==null?'':x).replace(/[&<>"']/g,function(m){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]})}
async function load(){try{const snapReq=await fetch(BASE+'/api/snapshot');const reqReq=await fetch(BASE+'/api/requests');const snap=await snapReq.json();const reqs=await reqReq.json();document.getElementById('app').innerHTML=PAGE==='requests'?renderRequests(snap,reqs):renderMetrics(snap,reqs)}catch(e){document.getElementById('app').innerHTML='<div class="card err">Falha ao carregar console: '+escapeHtml(e.message)+'</div>'}}
function card(label,value,small,cls){return '<div class="card"><div class="label">'+label+'</div><div class="value '+(cls||'')+'">'+value+'</div><div class="small">'+small+'</div></div>'}
function renderMetrics(s,r){const t=s.transfer||{},g=s.gateway||{},mh=s.media_hub||{},q=s.queue||{},c=s.request_counters||{},h=s.hardening||{};let html='<div class="grid">';html+=card('Node',escapeHtml(el(mh.node_uuid||'—').slice(0,8)),escapeHtml(el(mh.base_url||'Media Hub não configurado')),'');html+=card('Jobs ativos',num(t.active_jobs)+'/'+num(t.max_concurrent_jobs),'completed '+num(t.completed_jobs)+' · failed '+num(t.failed_jobs),'');html+=card('Sessões gateway',num(g.active_sessions)+'/'+num(g.max_sessions),'egress '+num(g.current_egress_mbps)+' Mbps','');html+=card('Requisições',num(c.total),'in '+num(c.inbound)+' · out '+num(c.outbound)+' · erros '+num(c.errors),'');html+='</div><div class="grid">';html+=card('Fila local',num(q.length)+'/'+num(q.capacity),'driver '+escapeHtml(el(q.driver||'—')),'');html+=card('Transferência',t.claim_enabled?'claim on':'claim off','work '+escapeHtml(el(t.work_dir||'—')),t.claim_enabled?'ok':'warn');html+=card('Gateway',g.enabled?'ativo':'inativo','proxy '+Boolean(g.proxy_enabled)+' · redirect '+Boolean(g.redirect_enabled),g.enabled?'ok':'warn');html+=card('Hardening',escapeHtml(el(h.reason||'—')),'drain '+Boolean(h.drain_enabled)+' · disk '+Boolean(h.disk_guard_enabled),h.allowed?'ok':'warn');html+='</div><div class="section"><h2>Últimas requisições</h2>'+requestsTable(r.slice(0,12))+'</div><div class="section"><h2>Snapshot JSON</h2><div class="card pre">'+escapeHtml(JSON.stringify(s,null,2))+'</div></div>';return html}
function renderRequests(s,r){const c=s.request_counters||{};let html='<div class="grid">';html+=card('Total',num(c.total),'','');html+=card('Inbound',num(c.inbound),'','');html+=card('Outbound',num(c.outbound),'','');html+=card('Erros',num(c.errors),'',Number(c.errors||0)>0?'err':'ok');html+='</div>'+requestsTable(r);return html}
function requestsTable(rows){let body='';for(const x of rows){body+='<tr><td class="mono">'+new Date(x.finished_at).toLocaleTimeString('pt-BR')+'</td><td><span class="pill">'+escapeHtml(x.direction)+'</span></td><td class="mono">'+escapeHtml(x.method)+'</td><td class="mono">'+escapeHtml(x.path)+'</td><td class="'+statusClass(x.status)+'">'+escapeHtml(x.status||'—')+'</td><td>'+num(x.duration_ms)+' ms</td><td>'+num(x.bytes)+'</td><td class="err">'+escapeHtml(x.error||'')+'</td></tr>'}return '<table><thead><tr><th>Hora</th><th>Dir</th><th>Método</th><th>Path</th><th>Status</th><th>Duração</th><th>Bytes</th><th>Erro</th></tr></thead><tbody>'+body+'</tbody></table>'}
load();setInterval(load,REFRESH);
</script>
</body></html>`
