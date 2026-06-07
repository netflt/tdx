package server

import (
	"net/http"
)

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>TDX 股票数据 API 测试</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; min-height: 100vh; }
.container { max-width: 1400px; margin: 0 auto; padding: 20px; }
header { display: flex; justify-content: space-between; align-items: center; padding: 16px 0; border-bottom: 1px solid #1e293b; margin-bottom: 20px; }
header h1 { font-size: 20px; color: #38bdf8; }
.status { font-size: 13px; color: #94a3b8; }
.status .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #22c55e; margin-right: 6px; animation: pulse 2s infinite; }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
.grid { display: grid; grid-template-columns: 320px 1fr; gap: 16px; }
@media (max-width: 900px) { .grid { grid-template-columns: 1fr; } }
.sidebar { background: #1e293b; border-radius: 8px; padding: 12px; overflow-y: auto; max-height: calc(100vh - 120px); }
.sidebar h3 { font-size: 13px; color: #64748b; text-transform: uppercase; letter-spacing: 1px; margin: 12px 0 8px; }
.sidebar h3:first-child { margin-top: 0; }
.api-item { display: flex; align-items: center; padding: 8px 10px; border-radius: 6px; cursor: pointer; transition: background 0.15s; margin-bottom: 2px; }
.api-item:hover { background: #334155; }
.api-item.active { background: #0ea5e9; color: #fff; }
.api-item .method { font-size: 11px; font-weight: 700; padding: 2px 6px; border-radius: 3px; margin-right: 8px; min-width: 36px; text-align: center; }
.api-item .method.get { background: #22c55e20; color: #22c55e; }
.api-item .method.post { background: #f59e0b20; color: #f59e0b; }
.api-item .path { font-size: 13px; font-family: 'SF Mono', Menlo, monospace; }
.main { display: flex; flex-direction: column; gap: 16px; }
.card { background: #1e293b; border-radius: 8px; padding: 16px; }
.card h2 { font-size: 15px; color: #38bdf8; margin-bottom: 12px; }
.url-bar { display: flex; gap: 8px; margin-bottom: 12px; }
.url-bar select { background: #0f172a; color: #22c55e; border: 1px solid #334155; border-radius: 6px; padding: 8px 12px; font-size: 13px; font-weight: 700; cursor: pointer; }
.url-bar input { flex: 1; background: #0f172a; color: #e2e8f0; border: 1px solid #334155; border-radius: 6px; padding: 8px 12px; font-size: 13px; font-family: 'SF Mono', Menlo, monospace; }
.url-bar button { background: #0ea5e9; color: #fff; border: none; border-radius: 6px; padding: 8px 20px; font-size: 13px; font-weight: 600; cursor: pointer; transition: background 0.15s; }
.url-bar button:hover { background: #0284c7; }
.params { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px; margin-bottom: 12px; }
.param { display: flex; align-items: center; gap: 6px; }
.param label { font-size: 12px; color: #94a3b8; min-width: 70px; }
.param input { flex: 1; background: #0f172a; color: #e2e8f0; border: 1px solid #334155; border-radius: 4px; padding: 6px 8px; font-size: 12px; font-family: 'SF Mono', Menlo, monospace; }
.param .hint { font-size: 11px; color: #475569; }
.result { background: #0f172a; border-radius: 6px; padding: 12px; max-height: 500px; overflow: auto; }
.result pre { font-size: 12px; font-family: 'SF Mono', Menlo, monospace; white-space: pre-wrap; word-break: break-all; color: #cbd5e1; line-height: 1.5; }
.result .time { font-size: 11px; color: #64748b; margin-bottom: 8px; }
.result .error { color: #f87171; }
.tabs { display: flex; gap: 4px; margin-bottom: 8px; }
.tabs button { background: #334155; color: #94a3b8; border: none; border-radius: 4px; padding: 4px 12px; font-size: 12px; cursor: pointer; }
.tabs button.active { background: #0ea5e9; color: #fff; }
</style>
</head>
<body>
<div class="container">
<header>
  <h1>TDX 股票数据 API</h1>
  <div class="status"><span class="dot"></span>服务运行中</div>
</header>
<div class="grid">
  <div class="sidebar" id="sidebar"></div>
  <div class="main">
    <div class="card">
      <h2>请求</h2>
      <div class="url-bar">
        <select id="method"><option value="GET">GET</option><option value="POST">POST</option></select>
        <input id="url" value="/api/quote?code=000001" placeholder="输入API路径">
        <button onclick="sendRequest()">发送</button>
      </div>
      <div class="params" id="params"></div>
      <div id="postBody" style="display:none">
        <textarea id="bodyInput" style="width:100%;height:80px;background:#0f172a;color:#e2e8f0;border:1px solid #334155;border-radius:6px;padding:8px;font-size:12px;font-family:'SF Mono',Menlo,monospace;" placeholder='{"codes":["000001","600519"]}'></textarea>
      </div>
    </div>
    <div class="card">
      <h2>响应</h2>
      <div class="tabs">
        <button class="active" onclick="switchTab(this,'body')">Body</button>
        <button onclick="switchTab(this,'headers')">Headers</button>
      </div>
      <div class="result" id="result">
        <pre>点击左侧接口或输入路径后点击"发送"</pre>
      </div>
    </div>
  </div>
</div>
</div>
<script>
const apis = [
  {group:'行情',items:[
    {method:'GET',path:'/api/quote',desc:'五档行情',params:[{k:'code',v:'000001',hint:'逗号分隔多只'}]},
    {method:'POST',path:'/api/batch-quote',desc:'批量行情',body:'{"codes":["000001","600519"]}'},
  ]},
  {group:'K线',items:[
    {method:'GET',path:'/api/kline',desc:'K线数据',params:[{k:'code',v:'000001'},{k:'type',v:'day',hint:'minute1/5/15/30/hour/day/week/month'},{k:'count',v:'100'}]},
    {method:'GET',path:'/api/kline-all',desc:'全量K线',params:[{k:'code',v:'000001'},{k:'type',v:'day'},{k:'limit',v:'',hint:'截取最近N条'}]},
    {method:'GET',path:'/api/kline-all/tdx',desc:'通达信全量K线',params:[{k:'code',v:'000001'},{k:'type',v:'day'}]},
    {method:'GET',path:'/api/kline-all/ths',desc:'同花顺前复权K线',params:[{k:'code',v:'000001'},{k:'type',v:'day'}]},
    {method:'GET',path:'/api/kline-history',desc:'历史K线',params:[{k:'code',v:'000001'},{k:'type',v:'day'},{k:'limit',v:'100'}]},
  ]},
  {group:'指数',items:[
    {method:'GET',path:'/api/index',desc:'指数K线',params:[{k:'code',v:'sh000001',hint:'sh000001/sz399001'},{k:'type',v:'day'},{k:'count',v:'100'}]},
    {method:'GET',path:'/api/index/all',desc:'指数全量K线',params:[{k:'code',v:'sh000001'},{k:'type',v:'day'}]},
  ]},
  {group:'分时',items:[
    {method:'GET',path:'/api/minute',desc:'分时数据',params:[{k:'code',v:'000001'},{k:'date',v:'',hint:'YYYYMMDD'}]},
  ]},
  {group:'分时成交',items:[
    {method:'GET',path:'/api/trade',desc:'分时成交',params:[{k:'code',v:'000001'},{k:'date',v:''}]},
    {method:'GET',path:'/api/trade-history',desc:'历史分时成交(全量)',params:[{k:'code',v:'000001'},{k:'date',v:'20260605'}]},
    {method:'GET',path:'/api/minute-trade-all',desc:'全天分时成交',params:[{k:'code',v:'000001'},{k:'date',v:''}]},
    {method:'GET',path:'/api/trade-history/full',desc:'上市至今分时成交',params:[{k:'code',v:'000001'},{k:'before',v:''},{k:'limit',v:''}]},
  ]},
  {group:'代码表',items:[
    {method:'GET',path:'/api/codes',desc:'证券代码列表',params:[{k:'exchange',v:'all',hint:'sh/sz/bj/all'}]},
    {method:'GET',path:'/api/stock-codes',desc:'股票代码列表',params:[{k:'limit',v:''},{k:'prefix',v:'true'}]},
    {method:'GET',path:'/api/etf-codes',desc:'ETF代码列表',params:[{k:'limit',v:''}]},
    {method:'GET',path:'/api/etf',desc:'ETF列表',params:[{k:'exchange',v:'all'},{k:'limit',v:''}]},
    {method:'GET',path:'/api/search',desc:'搜索股票',params:[{k:'keyword',v:'平安'}]},
    {method:'GET',path:'/api/market-count',desc:'市场证券数量'},
  ]},
  {group:'综合',items:[
    {method:'GET',path:'/api/stock-info',desc:'股票综合信息',params:[{k:'code',v:'000001'}]},
  ]},
  {group:'除权除息',items:[
    {method:'GET',path:'/api/gbbq',desc:'除权除息',params:[{k:'code',v:'000001'}]},
  ]},
  {group:'财务',items:[
    {method:'GET',path:'/api/finance',desc:'财务信息',params:[{k:'code',v:'000001'}]},
  ]},
  {group:'板块',items:[
    {method:'GET',path:'/api/blocks',desc:'板块列表',params:[{k:'type',v:'concept'}]},
    {method:'GET',path:'/api/block-members',desc:'板块成分',params:[{k:'name',v:'tdxhy.cfg'}]},
  ]},
  {group:'集合竞价',items:[
    {method:'GET',path:'/api/call-auction',desc:'集合竞价',params:[{k:'code',v:'000001'}]},
  ]},
  {group:'工作日',items:[
    {method:'GET',path:'/api/workday',desc:'交易日信息',params:[{k:'date',v:''},{k:'count',v:'1'}]},
    {method:'GET',path:'/api/workday/range',desc:'交易日范围',params:[{k:'start',v:'20260601'},{k:'end',v:'20260610'}]},
  ]},
  {group:'收益',items:[
    {method:'GET',path:'/api/income',desc:'收益区间',params:[{k:'code',v:'000001'},{k:'start_date',v:'20260101'},{k:'days',v:'5,10,20'}]},
  ]},
  {group:'任务',items:[
    {method:'POST',path:'/api/tasks/pull-kline',desc:'创建K线入库任务',body:'{"codes":["000001"],"tables":["day"]}'},
    {method:'POST',path:'/api/tasks/pull-trade',desc:'创建分时成交入库任务',body:'{"code":"000001","start_year":2024,"end_year":2025}'},
    {method:'GET',path:'/api/tasks',desc:'任务列表'},
  ]},
  {group:'系统',items:[
    {method:'GET',path:'/api/server-status',desc:'服务状态'},
    {method:'GET',path:'/api/docs',desc:'接口文档'},
  ]},
];

let currentApi = null;

function renderSidebar() {
  const sb = document.getElementById('sidebar');
  sb.innerHTML = '';
  apis.forEach(g => {
    let h = '<h3>' + g.group + '</h3>';
    g.items.forEach((a,i) => {
      h += '<div class="api-item" data-group="'+g.group+'" data-idx="'+i+'" onclick="selectApi(this)">';
      h += '<span class="method '+a.method.toLowerCase()+'">'+a.method+'</span>';
      h += '<span class="path">'+a.path+'</span>';
      h += '</div>';
    });
    sb.innerHTML += h;
  });
}

function selectApi(el) {
  document.querySelectorAll('.api-item').forEach(e => e.classList.remove('active'));
  el.classList.add('active');
  const g = el.dataset.group, i = parseInt(el.dataset.idx);
  const group = apis.find(a => a.group === g);
  const api = group.items[i];
  currentApi = api;

  document.getElementById('method').value = api.method;
  document.getElementById('url').value = api.path;

  // params
  const pc = document.getElementById('params');
  pc.innerHTML = '';
  if (api.params) {
    api.params.forEach(p => {
      pc.innerHTML += '<div class="param"><label>'+p.k+'</label><input id="p_'+p.k+'" value="'+(p.v||'')+'" placeholder="'+(p.hint||'')+'"><span class="hint">'+(p.hint||'')+'</span></div>';
    });
  }

  // body
  const bodyDiv = document.getElementById('postBody');
  if (api.body) {
    bodyDiv.style.display = 'block';
    document.getElementById('bodyInput').value = api.body;
  } else {
    bodyDiv.style.display = 'none';
  }

  buildUrl();
}

function buildUrl() {
  if (!currentApi || !currentApi.params) return currentApi ? currentApi.path : '';
  let url = currentApi.path + '?';
  currentApi.params.forEach(p => {
    const el = document.getElementById('p_'+p.k);
    if (el && el.value) url += p.k+'='+encodeURIComponent(el.value)+'&';
  });
  return url.replace(/&$/,'').replace(/\?$/,'');
}

document.getElementById('url').addEventListener('input', function() {
  // parse url to fill params
});

async function sendRequest() {
  const method = document.getElementById('method').value;
  let url = buildUrl() || document.getElementById('url').value;
  const resultDiv = document.getElementById('result');
  const startTime = performance.now();

  try {
    const opts = {method: method, headers: {'Content-Type': 'application/json'}};
    if (method === 'POST') {
      const body = document.getElementById('bodyInput').value;
      if (body) opts.body = body;
    }
    const resp = await fetch(url, opts);
    const elapsed = ((performance.now() - startTime) / 1000).toFixed(3);
    const data = await resp.json();

    let html = '<div class="time">HTTP ' + resp.status + ' | ' + elapsed + 's | ' + JSON.stringify(data).length + ' bytes</div>';
    if (data.code !== 0) {
      html += '<pre class="error">' + JSON.stringify(data, null, 2) + '</pre>';
    } else {
      html += '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
    }
    resultDiv.innerHTML = html;
  } catch(e) {
    const elapsed = ((performance.now() - startTime) / 1000).toFixed(3);
    resultDiv.innerHTML = '<div class="time">' + elapsed + 's</div><pre class="error">请求失败: ' + e.message + '</pre>';
  }
}

function switchTab(btn, tab) {
  document.querySelectorAll('.tabs button').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
}

renderSidebar();
</script>
</body>
</html>`
