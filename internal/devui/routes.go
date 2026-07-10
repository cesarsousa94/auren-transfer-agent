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
	Storage         any             `json:"storage"`
	Hardening       any             `json:"hardening"`
	DevUI           any             `json:"dev_ui"`
	RequestCounters Counters        `json:"request_counters"`
	RecentRequests  []RequestRecord `json:"recent_requests"`
	Summary         Summary         `json:"summary"`
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
		{Name: "devui.settings", Method: http.MethodGet, Pattern: path + "/settings", Handler: htmlHandler(options, "settings")},
		{Name: "devui.api.snapshot", Method: http.MethodGet, Pattern: path + "/api/snapshot", Handler: snapshotHandler(options)},
		{Name: "devui.api.snapshot.trailing", Method: http.MethodGet, Pattern: path + "/api/snapshot/", Handler: snapshotHandler(options)},
		{Name: "devui.api.requests", Method: http.MethodGet, Pattern: path + "/api/requests", Handler: requestsHandler(options)},
		{Name: "devui.api.requests.trailing", Method: http.MethodGet, Pattern: path + "/api/requests/", Handler: requestsHandler(options)},
		{Name: "devui.api.overview", Method: http.MethodGet, Pattern: path + "/api/overview", Handler: overviewHandler(options)},
		{Name: "devui.api.config", Method: http.MethodGet, Pattern: path + "/api/config", Handler: configHandler(options)},
		{Name: "devui.api.fallback", Method: http.MethodGet, Pattern: path + "/api/*", Handler: apiNotFoundHandler(path)},
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
		snapshot := buildSnapshot(options)
		writeJSON(writer, http.StatusOK, snapshot)
	}
}

func requestsHandler(options Options) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		limit := 500
		if options.Recorder == nil {
			writeJSON(writer, http.StatusOK, []RequestRecord{})
			return
		}
		writeJSON(writer, http.StatusOK, options.Recorder.Snapshot(limit))
	}
}

func overviewHandler(options Options) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if options.Recorder == nil {
			writeJSON(writer, http.StatusOK, Summary{GeneratedAt: time.Now().UTC()})
			return
		}
		writeJSON(writer, http.StatusOK, options.Recorder.Summary(500))
	}
}

func configHandler(options Options) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]any{
			"dev_ui": map[string]any{
				"enabled":          options.Config.Enabled,
				"path":             normalizeBasePath(options.Config.Path),
				"retention":        options.Config.Retention,
				"refresh_interval": options.Config.RefreshInterval,
				"capture_bodies":   options.Config.CaptureBodies,
				"body_limit_bytes": options.Config.BodyLimitBytes,
			},
			"pages": []string{"metrics", "requests", "settings"},
			"note":  "As preferências visuais desta tela são salvas no navegador. Configuração persistente do Agent continua no agent.yaml.",
		})
	}
}

func buildSnapshot(options Options) MetricsSnapshot {
	snapshot := MetricsSnapshot{GeneratedAt: time.Now().UTC()}
	if options.Snapshot != nil {
		snapshot = options.Snapshot()
	}
	if options.Recorder != nil {
		snapshot.RequestCounters = options.Recorder.Counters()
		snapshot.RecentRequests = options.Recorder.Snapshot(50)
		snapshot.Summary = options.Recorder.Summary(500)
	}
	snapshot.DevUI = map[string]any{"path": normalizeBasePath(options.Config.Path), "refresh_interval": options.Config.RefreshInterval, "retention": options.Config.Retention, "capture_bodies": options.Config.CaptureBodies, "body_limit_bytes": options.Config.BodyLimitBytes}
	return snapshot
}

func apiNotFoundHandler(basePath string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusNotFound, map[string]any{
			"error":   "dev_ui_api_route_not_found",
			"message": "Endpoint JSON do Dev Console não encontrado.",
			"path":    request.URL.Path,
			"expected": []string{
				normalizeBasePath(basePath) + "/api/snapshot",
				normalizeBasePath(basePath) + "/api/overview",
				normalizeBasePath(basePath) + "/api/requests",
				normalizeBasePath(basePath) + "/api/config",
			},
		})
	}
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
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
<html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Auren Transfer Agent Console</title>
<style>
:root{color-scheme:dark;--bg:#090d14;--panel:#121927;--panel2:#0f1521;--text:#e7edf7;--muted:#8ea0b8;--line:#253247;--ok:#3ddc97;--warn:#ffd166;--err:#ff6b6b;--brand:#8ab4ff;--info:#7dd3fc;--chip:#1b2638}*{box-sizing:border-box}body{margin:0;background:linear-gradient(120deg,#080b12,#111827);font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--text)}header{padding:18px 26px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;align-items:center;position:sticky;top:0;background:rgba(9,13,20,.94);backdrop-filter:blur(10px);z-index:4}h1{font-size:18px;margin:0}.sub{color:var(--muted);font-size:12px;margin-top:4px}nav a{color:var(--muted);text-decoration:none;margin-left:16px;font-size:14px}nav a.active{color:var(--brand);font-weight:800}main{padding:22px 26px}.grid{display:grid;grid-template-columns:repeat(4,minmax(150px,1fr));gap:14px;margin-bottom:16px}.grid3{display:grid;grid-template-columns:2fr 1fr 1fr;gap:14px;margin-bottom:16px}.card{background:linear-gradient(180deg,var(--panel),var(--panel2));border:1px solid var(--line);border-radius:16px;padding:15px;box-shadow:0 14px 40px rgba(0,0,0,.22)}.label{font-size:11px;color:var(--muted);text-transform:uppercase;letter-spacing:.08em}.value{font-size:27px;font-weight:850;margin-top:8px}.small{font-size:13px;color:var(--muted);margin-top:6px}.ok{color:var(--ok)}.warn{color:var(--warn)}.err{color:var(--err)}.info{color:var(--info)}.muted{color:var(--muted)}table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--line);border-radius:16px;overflow:hidden}th,td{padding:9px 11px;border-bottom:1px solid var(--line);font-size:12.5px;text-align:left;vertical-align:top}th{color:var(--muted);font-size:10.5px;text-transform:uppercase;letter-spacing:.08em;background:#0b111d;position:sticky;top:70px;z-index:2}tr:last-child td{border-bottom:0}tr.click{cursor:pointer}tr.click:hover{background:#182235}.pill{display:inline-block;padding:3px 8px;border-radius:999px;background:var(--chip);color:var(--muted);font-size:11.5px;margin:1px}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}.section{margin-top:18px}.pre{white-space:pre-wrap;word-break:break-word;color:var(--muted)}.two{display:grid;grid-template-columns:1fr 1fr;gap:14px}.toolbar{display:flex;gap:9px;margin:0 0 14px 0;flex-wrap:wrap;align-items:center}.toolbar input,.toolbar select,.toolbar button{background:#0b111d;color:var(--text);border:1px solid var(--line);border-radius:10px;padding:10px}.toolbar button{cursor:pointer}.toolbar button.active{border-color:var(--brand);color:var(--brand)}.detail{margin-top:10px;border-left:3px solid var(--brand)}summary{cursor:pointer;color:var(--brand);font-weight:700}code{color:#dbeafe}.bar{height:8px;border:1px solid var(--line);background:#0a0f1a;border-radius:999px;overflow:hidden;margin-top:7px}.bar span{display:block;height:100%;background:linear-gradient(90deg,var(--brand),var(--ok));width:0}.split{display:grid;grid-template-columns:1.1fr .9fr;gap:14px}.sticky-tools{position:sticky;top:71px;background:rgba(9,13,20,.92);padding:10px 0;z-index:3}.badge-row{display:flex;flex-wrap:wrap;gap:7px}.danger{border-color:rgba(255,107,107,.5)}@media(max-width:1100px){.grid{grid-template-columns:repeat(2,minmax(120px,1fr))}.grid3,.split,.two{grid-template-columns:1fr}header{display:block}nav{margin-top:12px}nav a{margin:0 12px 0 0}th{position:static}}@media(max-width:560px){.grid{grid-template-columns:1fr}main{padding:16px}header{padding:16px}}
</style></head><body>
<header><div><h1>Auren Transfer Agent Console</h1><div class="sub">{{.Version}} · console local de desenvolvimento operacional</div></div><nav><a href="{{.BasePath}}/metrics" class="{{if eq .Page "metrics"}}active{{end}}">Métricas</a><a href="{{.BasePath}}/requests" class="{{if eq .Page "requests"}}active{{end}}">Requisições</a><a href="{{.BasePath}}/settings" class="{{if eq .Page "settings"}}active{{end}}">Configurar painel</a><a href="{{.BasePath}}/api/snapshot">JSON</a></nav></header>
<main><div id="app" class="pre">Carregando...</div></main>
<script>
const CONFIG_BASE={{printf "%q" .BasePath}};const PAGE={{printf "%q" .Page}};const DEFAULT_REFRESH={{.RefreshMS}};
function currentBasePath(){const p=window.location.pathname.replace(/\/+$/,'');for(const s of ['/metrics','/requests','/settings']){if(p.endsWith(s))return p.slice(0,-s.length)||CONFIG_BASE}return CONFIG_BASE}
const BASE=currentBasePath();let LAST_REQ=[];let LAST_SNAP=null;let LAST_OVERVIEW=null;let timer=null;
const defaults={refresh:DEFAULT_REFRESH,paused:false,filter:'',dir:'all',kind:'all',function:'all',hideNoise:true,maxRows:150,detailMode:'compact'};
let prefs=Object.assign({},defaults,JSON.parse(localStorage.getItem('auren.devui.prefs')||'{}'));
function savePrefs(){localStorage.setItem('auren.devui.prefs',JSON.stringify(prefs))}
function el(v){return String(v==null?'':v)}function num(v){return Number(v||0).toLocaleString('pt-BR')}function bytes(v){v=Number(v||0);if(v<1024)return v+' B';const u=['KB','MB','GB','TB'];let i=-1;do{v/=1024;i++}while(v>=1024&&i<u.length-1);return v.toFixed(v>=10?1:2)+' '+u[i]}function pct(v){return Number(v||0).toLocaleString('pt-BR',{maximumFractionDigits:1})+'%'}function statusClass(s){s=Number(s||0);return s>=500?'err':(s>=400?'warn':(s===0?'info':'ok'))}function escapeHtml(x){return String(x==null?'':x).replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]))}
async function readJSON(url){const res=await fetch(url,{headers:{'Accept':'application/json'},cache:'no-store'});const text=await res.text();const ct=res.headers.get('content-type')||'';if(!res.ok)throw new Error(url+' retornou HTTP '+res.status+' '+res.statusText+' · corpo: '+text.slice(0,260));if(!ct.includes('application/json'))throw new Error(url+' retornou Content-Type inesperado '+(ct||'vazio')+' · corpo: '+text.slice(0,260));try{return JSON.parse(text)}catch(e){throw new Error(url+' retornou JSON inválido: '+e.message+' · corpo: '+text.slice(0,260))}}
async function load(force=false){if(prefs.paused&&!force)return;try{const [snap,reqs,overview]=await Promise.all([readJSON(BASE+'/api/snapshot'),readJSON(BASE+'/api/requests'),readJSON(BASE+'/api/overview')]);LAST_SNAP=snap;LAST_REQ=reqs;LAST_OVERVIEW=overview;render()}catch(e){document.getElementById('app').innerHTML='<div class="card err"><b>Falha ao carregar console</b><div class="small">'+escapeHtml(e.message)+'</div><div class="small mono">BASE='+escapeHtml(BASE)+'</div></div>'}}
function render(){document.getElementById('app').innerHTML=PAGE==='requests'?renderRequests(LAST_SNAP,LAST_REQ,LAST_OVERVIEW):(PAGE==='settings'?renderSettings(LAST_SNAP):renderMetrics(LAST_SNAP,LAST_REQ,LAST_OVERVIEW))}
function restartTimer(){if(timer)clearInterval(timer);timer=setInterval(()=>load(false),Number(prefs.refresh||DEFAULT_REFRESH));savePrefs()}function card(label,value,small,cls){return '<div class="card"><div class="label">'+label+'</div><div class="value '+(cls||'')+'">'+value+'</div><div class="small">'+small+'</div></div>'}
function toolbar(){return '<div class="sticky-tools toolbar"><button onclick="prefs.paused=!prefs.paused;savePrefs();render();load(true)" class="'+(prefs.paused?'active':'')+'">'+(prefs.paused?'Pausado':'Auto-refresh')+'</button><button onclick="load(true)">Atualizar agora</button><select onchange="prefs.refresh=Number(this.value);restartTimer();render()"><option value="1000" '+sel(prefs.refresh,1000)+'>1s</option><option value="2000" '+sel(prefs.refresh,2000)+'>2s</option><option value="5000" '+sel(prefs.refresh,5000)+'>5s</option><option value="10000" '+sel(prefs.refresh,10000)+'>10s</option></select><label class="small"><input type="checkbox" '+(prefs.hideNoise?'checked':'')+' onchange="prefs.hideNoise=this.checked;savePrefs();render()"> ocultar heartbeat/metrics/control/dev</label></div>'}
function sel(a,b){return Number(a)===Number(b)?'selected':''}
function renderMetrics(s,r,o){s=s||{};o=o||{};const t=s.transfer||{},g=s.gateway||{},mh=s.media_hub||{},q=s.queue||{},c=s.request_counters||{},h=s.hardening||{},st=s.storage||{};const active=t.active_job_details||[];let html=toolbar();html+='<div class="grid">';html+=card('Downloads ativos',num(active.filter(j=>(j.stage||'').includes('download')).length),num(t.active_jobs)+' jobs ativos / '+num(t.max_concurrent_jobs)+' capacidade','');html+=card('Uploads ativos',num(active.filter(j=>(j.stage||'').includes('upload')).length),num(t.completed_jobs)+' concluídos · '+num(t.failed_jobs)+' falhos',Number(t.failed_jobs||0)>0?'warn':'ok');html+=card('Fila / claim',num(q.length)+'/'+num(q.capacity),'driver '+escapeHtml(el(q.driver||'—'))+' · claim '+(t.claim_enabled?'on':'off'),t.claim_enabled?'ok':'warn');html+=card('Requisições',num(c.total),'out '+num(c.outbound)+' · eventos '+num(c.events)+' · erros '+num(c.errors),Number(c.errors||0)>0?'err':'');html+='</div>';html+='<div class="grid">';html+=card('Node',escapeHtml(el(mh.node_uuid||'—').slice(0,8)),escapeHtml(el(mh.base_url||'Media Hub não configurado')),'');html+=card('Storage',escapeHtml(el(st.driver||'—')),escapeHtml(el(st.summary||'')),st.configured||st.s3_configured||st.auren_configured?'ok':'warn');html+=card('Gateway',g.enabled?'ativo':'inativo','sessões '+num(g.active_sessions)+'/'+num(g.max_sessions)+' · egress '+num(g.current_egress_mbps)+' Mbps',g.enabled?'ok':'warn');html+=card('Hardening',escapeHtml(el(h.reason||'—')),'drain '+Boolean(h.drain_enabled)+' · backpressure '+Boolean(h.backpressure_enabled),h.allowed?'ok':'warn');html+='</div>';html+='<div class="split"><div>'+activeJobs(active)+'</div><div>'+overviewCards(o)+'</div></div>';html+='<div class="split"><div>'+summaryTable('Funções usadas',o.functions||[],['name','count','errors','last_status'])+'</div><div>'+summaryTable('Endpoints chamados',o.endpoints||[],['method','path','count','errors','avg_duration_ms'])+'</div></div>';html+='<div class="section"><h2>Eventos recentes por stage</h2>'+summaryTable('',o.events||[],['operation','stage','count','errors','last_message'])+'</div>';return html}
function activeJobs(active){let rows='';for(const j of active){rows+='<div class="card"><div class="label">Job ativo</div><div class="mono">'+escapeHtml(j.uuid||'')+'</div><div class="badge-row"><span class="pill">'+escapeHtml(j.operation||'—')+'</span><span class="pill">'+escapeHtml(j.stage||'—')+'</span><span class="pill">'+escapeHtml(j.destination_driver||'—')+'</span></div><div class="small">'+escapeHtml(j.message||'')+'</div><div class="small mono">source '+escapeHtml(j.source_url||'—')+'</div><div class="small mono">object '+escapeHtml(j.object_path||'—')+'</div><div class="bar"><span style="width:'+Math.min(100,Number(j.percent||0))+'%"></span></div><div class="small">'+bytes(j.current_bytes)+' / '+bytes(j.total_bytes)+' · '+pct(j.percent)+' · '+bytes(j.speed_bps)+'/s</div></div>'}return '<h2>Jobs/downloads/uploads em andamento</h2>'+(rows||'<div class="card muted">Nenhum job ativo agora.</div>')}
function overviewCards(o){return '<h2>Resumo da janela</h2><div class="grid" style="grid-template-columns:1fr 1fr">'+card('Funções',num((o.functions||[]).length),'agrupadas por endpoint/stage','')+card('Endpoints',num((o.endpoints||[]).length),'na janela retida','')+card('Eventos',num((o.events||[]).length),'stages observados','')+card('Ruído ocultável',num(o.noisy_calls),'heartbeat/metrics/control/dev','info')+'</div>'}
function summaryTable(title,rows,cols){let head=cols.map(c=>'<th>'+escapeHtml(c)+'</th>').join('');let body='';for(const row of rows.slice(0,20)){body+='<tr>'+cols.map(c=>'<td class="'+(c.includes('error')&&Number(row[c])>0?'err':'')+' '+(String(row[c]||'').length>45?'mono':'')+'">'+escapeHtml(formatCell(c,row[c]))+'</td>').join('')+'</tr>'}return (title?'<h2>'+title+'</h2>':'')+'<table><thead><tr>'+head+'</tr></thead><tbody>'+body+'</tbody></table>'}
function formatCell(k,v){if(k.includes('duration'))return num(v)+' ms';if(k.includes('bytes'))return bytes(v);return v==null?'':v}
function renderRequests(s,r,o){r=filteredRows(r||[]);o=o||{};let html=toolbar();html+='<div class="grid">';html+=card('Mostrando',num(r.length),'de '+num((LAST_REQ||[]).length)+' registros','');html+=card('Jobs na janela',num((o.jobs||[]).length),'clique/filtre por UUID','');html+=card('Erros',num((o.errors||[]).length),'HTTP >=400 ou erro capturado',(o.errors||[]).length?'err':'ok');html+=card('Calls de fila/claim',num(o.queue_like_calls),'polls e tentativas de claim','info');html+='</div>';html+='<div class="toolbar"><input style="min-width:360px" placeholder="Filtrar por job, URL, endpoint, função, stage, erro..." value="'+escapeHtml(prefs.filter)+'" oninput="prefs.filter=this.value;savePrefs();render()"><select onchange="prefs.dir=this.value;savePrefs();render()"><option value="all">direção: todas</option><option value="inbound" '+selText(prefs.dir,'inbound')+'>inbound</option><option value="outbound" '+selText(prefs.dir,'outbound')+'>outbound</option><option value="event" '+selText(prefs.dir,'event')+'>event</option></select><select onchange="prefs.kind=this.value;savePrefs();render()"><option value="all">tipo: todos</option><option value="http" '+selText(prefs.kind,'http')+'>http</option><option value="transfer" '+selText(prefs.kind,'transfer')+'>transfer</option></select><select onchange="prefs.function=this.value;savePrefs();render()"><option value="all">função: todas</option>'+functionOptions(o.functions||[])+'</select><select onchange="prefs.maxRows=Number(this.value);savePrefs();render()"><option value="50" '+sel(prefs.maxRows,50)+'>50 linhas</option><option value="150" '+sel(prefs.maxRows,150)+'>150 linhas</option><option value="300" '+sel(prefs.maxRows,300)+'>300 linhas</option></select></div>';html+='<div class="split"><div>'+jobsTable(o.jobs||[])+'</div><div>'+summaryTable('Funções chamadas',o.functions||[],['name','count','errors','last_status','last_path'])+'</div></div>';html+='<div class="section"><h2>Timeline detalhada</h2>'+requestsTable(r.slice(0,Number(prefs.maxRows||150)),true)+'</div>';return html}
function selText(a,b){return String(a)===String(b)?'selected':''}function functionOptions(rows){return rows.map(x=>'<option value="'+escapeHtml(x.name)+'" '+selText(prefs.function,x.name)+'>'+escapeHtml(x.name)+' ('+num(x.count)+')</option>').join('')}
function classify(x){const p=(x.path||x.url||'').toLowerCase();if(x.kind==='transfer')return 'transfer.'+(x.stage||'event');if(p.includes('/nodes/heartbeat'))return 'media_hub.node.heartbeat';if(p.includes('/nodes/metrics'))return 'media_hub.node.metrics';if(p.includes('/nodes/events'))return 'media_hub.node.events';if(p.includes('/nodes/register'))return 'media_hub.node.register';if(p.includes('/nodes/config'))return 'media_hub.node.config';if(p.includes('/jobs/claim'))return 'transfer.claim';if(p.includes('/jobs/')&&p.includes('/progress'))return 'transfer.progress';if(p.includes('/jobs/')&&p.includes('/completed'))return 'transfer.completed';if(p.includes('/jobs/')&&p.includes('/failed'))return 'transfer.failed';if(p.includes('/jobs/')&&p.includes('/control'))return 'transfer.control';if(p.includes('/_auren/dev'))return 'dev_console';if(p.includes('s3')||p.includes('amazonaws.com'))return 'storage.s3';return 'http.'+(x.method||'').toLowerCase()}
function noisy(x){const f=classify(x);return ['media_hub.node.heartbeat','media_hub.node.metrics','media_hub.node.events','transfer.control','dev_console'].includes(f)}
function filteredRows(rows){const q=(prefs.filter||'').toLowerCase();return rows.filter(x=>(prefs.dir==='all'||x.direction===prefs.dir)&&(prefs.kind==='all'||x.kind===prefs.kind)&&(prefs.function==='all'||classify(x)===prefs.function)&&(!prefs.hideNoise||!noisy(x))&&(!q||JSON.stringify(x).toLowerCase().includes(q)))}
function jobsTable(jobs){let body='';for(const j of jobs.slice(0,15)){body+='<tr><td class="mono">'+escapeHtml((j.uuid||'').slice(0,8))+'</td><td>'+escapeHtml(j.operation||'')+'</td><td>'+escapeHtml(j.current_stage||'')+'</td><td>'+pct(j.percent)+'</td><td class="'+(j.errors?'err':'')+'">'+num(j.errors)+'</td></tr>'}return '<h2>Jobs vistos na janela</h2><table><thead><tr><th>Job</th><th>Operação</th><th>Stage</th><th>%</th><th>Erros</th></tr></thead><tbody>'+body+'</tbody></table>'}
function requestsTable(rows,details){let body='';for(const x of rows){const title=(x.job_uuid?'<div class="small">job <code>'+escapeHtml(x.job_uuid)+'</code> · '+escapeHtml(x.operation||'')+' · '+escapeHtml(x.stage||'')+'</div>':'')+(x.source_url?'<div class="small">source <code>'+escapeHtml(x.source_url)+'</code></div>':'')+(x.object_path?'<div class="small">object <code>'+escapeHtml(x.object_path)+'</code></div>':'');body+='<tr class="click"><td class="mono">'+new Date(x.finished_at||x.started_at).toLocaleTimeString('pt-BR')+'</td><td><span class="pill">'+escapeHtml(x.direction)+'</span><div class="small">'+escapeHtml(classify(x))+'</div></td><td class="mono">'+escapeHtml(x.method)+'</td><td class="mono">'+escapeHtml(x.path||x.url||x.stage)+' '+title+'</td><td class="'+statusClass(x.status)+'">'+escapeHtml(x.status||'—')+'</td><td>'+num(x.duration_ms)+' ms</td><td>'+bytes(x.bytes)+'</td><td class="err">'+escapeHtml(x.error||'')+'</td></tr>';if(details){body+='<tr><td colspan="8"><details class="detail"><summary>Ver payloads e registro completo #'+escapeHtml(x.id)+'</summary><div class="two"><div class="card"><div class="label">Enviado</div><div class="pre">'+escapeHtml(JSON.stringify({headers:x.headers,request_body:x.request_body,metadata:x.metadata},null,2))+'</div></div><div class="card"><div class="label">Recebido</div><div class="pre">'+escapeHtml(JSON.stringify({response_headers:x.response_headers,response_body:x.response_body},null,2))+'</div></div></div><div class="card"><div class="label">Registro completo sanitizado</div><div class="pre">'+escapeHtml(JSON.stringify(x,null,2))+'</div></div></details></td></tr>'}}return '<table><thead><tr><th>Hora</th><th>Função</th><th>Método</th><th>Endpoint / job / URL</th><th>Status</th><th>Duração</th><th>Bytes</th><th>Erro</th></tr></thead><tbody>'+body+'</tbody></table>'}
function renderSettings(s){s=s||{};const cfg=s.dev_ui||{};return toolbar()+'<div class="grid3"><div class="card"><div class="label">Preferências visuais</div><p class="small">Essas opções ficam no navegador e ajudam no ambiente de desenvolvimento. Elas não alteram o agent.yaml.</p><div class="toolbar"><label>Refresh <select onchange="prefs.refresh=Number(this.value);restartTimer();render()"><option value="1000" '+sel(prefs.refresh,1000)+'>1 segundo</option><option value="2000" '+sel(prefs.refresh,2000)+'>2 segundos</option><option value="5000" '+sel(prefs.refresh,5000)+'>5 segundos</option><option value="10000" '+sel(prefs.refresh,10000)+'>10 segundos</option></select></label><label>Linhas <select onchange="prefs.maxRows=Number(this.value);savePrefs();render()"><option value="50" '+sel(prefs.maxRows,50)+'>50</option><option value="150" '+sel(prefs.maxRows,150)+'>150</option><option value="300" '+sel(prefs.maxRows,300)+'>300</option></select></label><button onclick="prefs=Object.assign({},defaults);savePrefs();restartTimer();render()">Restaurar padrão</button></div></div><div class="card"><div class="label">Config runtime do Dev UI</div><div class="pre">'+escapeHtml(JSON.stringify(cfg,null,2))+'</div></div><div class="card"><div class="label">Segurança</div><div class="small">Ambiente dev: use túnel SSH/VPN/rede privada. Em produção, não exponha esse painel publicamente sem proxy/autenticação.</div></div></div>'}
load(true);restartTimer();
</script></body></html>`
