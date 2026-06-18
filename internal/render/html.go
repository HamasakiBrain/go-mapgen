package render

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/HamasakiBrain/go-mapgen/internal/model"
)

// HTMLBytes renders the self-contained interactive dashboard (D3 from CDN).
func HTMLBytes(pm *model.ProjectMap) ([]byte, error) {
	data, err := json.Marshal(pm)
	if err != nil {
		return nil, err
	}
	safe := strings.ReplaceAll(string(data), "</", "<\\/")
	return []byte(strings.Replace(dashboardTemplate, "/*__DATA__*/", safe, 1)), nil
}

// HTML writes the dashboard to a file.
func HTML(pm *model.ProjectMap, path string) error {
	b, err := HTMLBytes(pm)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

const dashboardTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MapGen Dashboard</title>
<script src="https://cdn.jsdelivr.net/npm/d3@7/dist/d3.min.js"></script>
<style>
:root{
  --bg:#0f172a; --panel:#1e293b; --panel2:#273449; --line:#334155;
  --text:#e2e8f0; --muted:#94a3b8; --accent:#60a5fa; --green:#34d399;
  --yellow:#fbbf24; --red:#f87171; --purple:#a78bfa;
}
*{box-sizing:border-box}
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  background:var(--bg);color:var(--text);font-size:14px}
.wrap{max-width:1400px;margin:0 auto;padding:24px}
header{display:flex;align-items:baseline;gap:16px;flex-wrap:wrap;margin-bottom:8px}
header h1{font-size:24px;margin:0}
.badge{padding:2px 10px;border-radius:999px;font-size:12px;font-weight:600;background:var(--panel2)}
.badge.engine{background:#064e3b;color:var(--green)}
.sub{color:var(--muted);margin-bottom:24px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:14px;margin-bottom:24px}
.card{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:16px}
.card .v{font-size:26px;font-weight:700}
.card .l{color:var(--muted);font-size:12px;margin-top:4px}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:18px;margin-bottom:24px}
@media(max-width:900px){.grid2{grid-template-columns:1fr}}
.panel{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:18px}
.panel h2{font-size:15px;margin:0 0 14px;color:var(--muted);text-transform:uppercase;letter-spacing:.5px}
.bar{display:flex;align-items:center;gap:10px;margin:6px 0}
.bar .name{width:90px;color:var(--muted);font-size:12px}
.bar .track{flex:1;height:8px;background:var(--panel2);border-radius:4px;overflow:hidden}
.bar .fill{height:100%;background:var(--accent)}
.bar .num{width:36px;text-align:right;color:var(--muted);font-size:12px}
.metric{display:flex;justify-content:space-between;padding:7px 0;border-bottom:1px solid var(--line)}
.metric:last-child{border:0}
.metric .mv{font-weight:600}
#graph{width:100%;height:460px;background:var(--panel);border:1px solid var(--line);border-radius:12px;margin-bottom:24px}
.legend{display:flex;gap:18px;margin:-12px 0 24px;color:var(--muted);font-size:12px}
.legend span{display:inline-flex;align-items:center;gap:6px}
.dot{width:10px;height:10px;border-radius:50%;display:inline-block}
input[type=search]{width:100%;padding:10px 12px;border-radius:8px;border:1px solid var(--line);
  background:var(--panel2);color:var(--text);margin-bottom:12px;font-size:14px}
.filters{display:flex;gap:14px;margin-bottom:12px;color:var(--muted);font-size:13px;flex-wrap:wrap}
.filters label{display:flex;align-items:center;gap:6px;cursor:pointer}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);font-size:13px}
th{color:var(--muted);cursor:pointer;user-select:none;position:sticky;top:0;background:var(--panel)}
td.id{font-family:ui-monospace,monospace;font-size:12px}
.tag{padding:1px 7px;border-radius:6px;font-size:11px;font-weight:600}
.tag.exp{background:#064e3b;color:var(--green)}
.tag.crit{background:#7f1d1d;color:var(--red)}
.tag.test{background:#1e3a8a;color:#93c5fd}
.tablewrap{max-height:520px;overflow:auto;border:1px solid var(--line);border-radius:12px}
.muted{color:var(--muted)}
.sev-error{color:var(--red)} .sev-warning{color:var(--yellow)}
.section-title{font-size:15px;color:var(--muted);text-transform:uppercase;letter-spacing:.5px;margin:8px 0 12px}
.tip{position:absolute;pointer-events:none;background:#0b1220;border:1px solid var(--line);
  padding:6px 10px;border-radius:6px;font-size:12px;opacity:0;transition:opacity .1s}
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>🗺️ <span id="proj"></span></h1>
    <span class="badge engine" id="engine"></span>
    <span class="badge" id="arch"></span>
    <span class="badge" id="mod"></span>
  </header>
  <div class="sub" id="gen"></div>

  <div class="cards" id="cards"></div>

  <div class="grid2">
    <div class="panel"><h2>Метрики качества</h2><div id="metrics"></div></div>
    <div class="panel"><h2>Бизнес-области</h2><div id="areas"></div></div>
  </div>

  <div class="section-title">Граф зависимостей пакетов</div>
  <div class="legend">
    <span><i class="dot" style="background:#3b82f6"></i>import</span>
    <span><i class="dot" style="background:#10b981"></i>call</span>
    <span><i class="dot" style="background:#fca5a5"></i>сложный пакет</span>
    <span class="muted">размер = число функций · тяни ноды мышью</span>
  </div>
  <svg id="graph"></svg>

  <div class="grid2">
    <div class="panel"><h2>Проблемы</h2><div id="issues"></div></div>
    <div class="panel"><h2>HTTP-маршруты</h2><div id="routes"></div></div>
  </div>

  <div class="section-title">Функции (<span id="fcount"></span>)</div>
  <input type="search" id="q" placeholder="Поиск по имени, пакету, файлу…">
  <div class="filters">
    <label><input type="checkbox" id="fExp">только экспортируемые</label>
    <label><input type="checkbox" id="fCrit">только критические</label>
    <label><input type="checkbox" id="fNoTest">скрыть тесты</label>
  </div>
  <div class="tablewrap">
    <table id="ftable">
      <thead><tr>
        <th data-k="ID">Функция</th><th data-k="Package">Пакет</th>
        <th data-k="Cyclomatic">Cyclo</th><th data-k="Cognitive">Cog</th>
        <th data-k="LinesOfCode">LOC</th><th data-k="fanin">Fan-in</th>
        <th data-k="fanout">Fan-out</th><th>Флаги</th>
      </tr></thead>
      <tbody id="fbody"></tbody>
    </table>
  </div>
</div>
<div class="tip" id="tip"></div>

<script>
const DATA = /*__DATA__*/;
const $ = s => document.querySelector(s);
const esc = s => (s==null?"":String(s)).replace(/[&<>]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;"}[c]));

// Header
$("#proj").textContent = DATA.project_name || "project";
$("#engine").textContent = "движок: " + DATA.engine;
$("#arch").textContent = DATA.summary.architecture;
$("#mod").textContent = DATA.module || "";
$("#gen").textContent = "Сгенерировано " + new Date(DATA.generated_at).toLocaleString() + " · Go " + DATA.go_version;

// Cards
const s = DATA.summary, q = DATA.quality;
const cards = [
  ["Пакеты", s.total_packages], ["Функции", s.total_functions],
  ["Интерфейсы", s.total_interfaces], ["Типы", s.total_types],
  ["Маршруты", s.total_routes], ["Строк кода", s.total_lines],
  ["Покрытие", q.test_coverage.toFixed(0)+"%"],
  ["Поддерживаемость", q.maintainability.toFixed(0)],
];
$("#cards").innerHTML = cards.map(c=>'<div class="card"><div class="v">'+esc(c[1])+'</div><div class="l">'+esc(c[0])+'</div></div>').join("");

// Metrics
const m = DATA.metrics;
const metrics = [
  ["Ср. цикломатическая", q.avg_cyclomatic.toFixed(2)],
  ["Ср. когнитивная", q.avg_cognitive.toFixed(2)],
  ["Документированность", m.documentation_score.toFixed(0)+"%"],
  ["Связанность (coupling)", m.coupling.toFixed(2)],
  ["Сцепление (cohesion)", m.cohesion.toFixed(2)],
  ["Технический долг", m.technical_debt.toFixed(1)],
  ["Циклы импортов", m.cycles_found],
];
$("#metrics").innerHTML = metrics.map(x=>'<div class="metric"><span class="muted">'+esc(x[0])+'</span><span class="mv">'+esc(x[1])+'</span></div>').join("");

// Areas
const areas = Object.entries(DATA.business.areas||{}).sort((a,b)=>b[1]-a[1]);
const amax = Math.max(1,...areas.map(a=>a[1]));
$("#areas").innerHTML = areas.map(a=>'<div class="bar"><span class="name">'+esc(a[0])+'</span><span class="track"><span class="fill" style="width:'+(a[1]/amax*100)+'%"></span></span><span class="num">'+a[1]+'</span></div>').join("") || '<span class="muted">нет данных</span>';

// Issues
const issues = DATA.quality.issues||[];
$("#issues").innerHTML = issues.length ? issues.slice(0,40).map(i=>'<div class="metric"><span class="sev-'+esc(i.severity)+'">●</span>&nbsp;<span style="flex:1">'+esc(i.message)+'</span><span class="muted">'+esc((i.file||"").split("/").pop())+':'+i.line+'</span></div>').join("") : '<span class="muted">проблем не найдено 🎉</span>';

// Routes
const routes = DATA.routes||[];
$("#routes").innerHTML = routes.length ? routes.map(r=>'<div class="metric"><span class="mv" style="width:60px">'+esc(r.method)+'</span><span style="flex:1;font-family:monospace">'+esc(r.path)+'</span><span class="muted">'+esc(r.handler)+'</span></div>').join("") : '<span class="muted">маршрутов не найдено</span>';

// ---- Function table ----
const funcs = (DATA.functions||[]).map(f=>({
  ...f, fanin:(f.called_by||[]).length, fanout:(f.calls||[]).length
}));
$("#fcount").textContent = funcs.length;
let sortKey="Cyclomatic", sortDir=-1;
function rowsFiltered(){
  const term=$("#q").value.toLowerCase();
  const exp=$("#fExp").checked, crit=$("#fCrit").checked, noTest=$("#fNoTest").checked;
  let r=funcs.filter(f=>{
    if(exp&&!f.exported)return false;
    if(crit&&!f.critical)return false;
    if(noTest&&f.is_test)return false;
    if(!term)return true;
    return (f.id+" "+f.package+" "+f.file).toLowerCase().includes(term);
  });
  const k={ID:"id",Package:"package",Cyclomatic:"cyclomatic",Cognitive:"cognitive",LinesOfCode:"lines_of_code",fanin:"fanin",fanout:"fanout"}[sortKey]||"cyclomatic";
  r.sort((a,b)=>{const x=a[k],y=b[k];return (x<y?-1:x>y?1:0)*sortDir;});
  return r;
}
function renderTable(){
  const r=rowsFiltered().slice(0,500);
  $("#fbody").innerHTML=r.map(f=>{
    const flags=[f.exported?'<span class="tag exp">exp</span>':'',f.critical?'<span class="tag crit">crit</span>':'',f.is_test?'<span class="tag test">test</span>':''].join(" ");
    const short=f.id.replace(/^[^ ]*\//,"");
    return '<tr><td class="id" title="'+esc(f.id)+'">'+esc(short)+'</td><td class="muted">'+esc(f.package_name)+'</td><td>'+f.cyclomatic+'</td><td>'+f.cognitive+'</td><td>'+f.lines_of_code+'</td><td>'+f.fanin+'</td><td>'+f.fanout+'</td><td>'+flags+'</td></tr>';
  }).join("");
}
["#q","#fExp","#fCrit","#fNoTest"].forEach(s=>$(s).addEventListener("input",renderTable));
document.querySelectorAll("#ftable th[data-k]").forEach(th=>th.addEventListener("click",()=>{
  const k=th.dataset.k; if(sortKey===k)sortDir*=-1;else{sortKey=k;sortDir=-1;} renderTable();
}));
renderTable();

// ---- D3 package graph ----
const pkgs=Object.values(DATA.packages||{});
const idset=new Set(pkgs.map(p=>p.import_path));
const nodes=pkgs.map(p=>({id:p.import_path,name:p.name,funcs:(p.functions||[]).length,cx:p.complexity||1}));
const links=(DATA.dependencies||[]).filter(d=>idset.has(d.from)&&idset.has(d.to)).map(d=>({source:d.from,target:d.target||d.to,type:d.type}));
const svg=d3.select("#graph"), W=svg.node().clientWidth, H=460;
svg.attr("viewBox",[0,0,W,H]);
const tip=$("#tip");
const color=c=>c>7?"#fca5a5":c>4?"#fde68a":"#93c5fd";
const sim=d3.forceSimulation(nodes)
  .force("link",d3.forceLink(links).id(d=>d.id).distance(110))
  .force("charge",d3.forceManyBody().strength(-340))
  .force("center",d3.forceCenter(W/2,H/2))
  .force("collide",d3.forceCollide(28));
const link=svg.append("g").selectAll("line").data(links).join("line")
  .attr("stroke",d=>d.type==="call"?"#10b981":"#3b82f6")
  .attr("stroke-opacity",.5).attr("stroke-dasharray",d=>d.type==="call"?"4 3":null);
const node=svg.append("g").selectAll("g").data(nodes).join("g").call(drag(sim));
node.append("circle").attr("r",d=>8+Math.sqrt(d.funcs)*2.5)
  .attr("fill",d=>color(d.cx)).attr("stroke","#0f172a").attr("stroke-width",1.5);
node.append("text").text(d=>d.name).attr("x",0).attr("y",d=>-(10+Math.sqrt(d.funcs)*2.5))
  .attr("text-anchor","middle").attr("fill","#e2e8f0").attr("font-size","11px");
node.on("mousemove",(e,d)=>{tip.style.opacity=1;tip.style.left=(e.pageX+12)+"px";tip.style.top=(e.pageY-10)+"px";
  tip.innerHTML="<b>"+esc(d.name)+"</b><br>"+esc(d.id)+"<br>функций: "+d.funcs+" · сложность: "+d.cx+"/10";})
 .on("mouseleave",()=>tip.style.opacity=0);
sim.on("tick",()=>{
  link.attr("x1",d=>d.source.x).attr("y1",d=>d.source.y).attr("x2",d=>d.target.x).attr("y2",d=>d.target.y);
  node.attr("transform",d=>"translate("+d.x+","+d.y+")");
});
function drag(sim){
  return d3.drag()
    .on("start",(e,d)=>{if(!e.active)sim.alphaTarget(.3).restart();d.fx=d.x;d.fy=d.y;})
    .on("drag",(e,d)=>{d.fx=e.x;d.fy=e.y;})
    .on("end",(e,d)=>{if(!e.active)sim.alphaTarget(0);d.fx=null;d.fy=null;});
}
</script>
</body>
</html>`
