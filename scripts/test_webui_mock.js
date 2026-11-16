window.testErrors=[];addEventListener('error',e=>testErrors.push(e.message));addEventListener('unhandledrejection',e=>testErrors.push(String(e.reason)));
try{localStorage.removeItem('stromea-mode');localStorage.setItem('stromea-language','de');localStorage.removeItem('stromea-technical');}catch{}
// Test-only fetch boundary. Every URL is handled locally or rejected.
window.mockAPI = {
  calls: [], devices: [], connectError: '', connectBusy: false, discoveryError: false, failRun: false,
  state: {device: {name: 'Loopback-Simulator', ski: '0123456789abcdef0123456789abcdef01234567'}, localSki: 'abcdef0123456789abcdef0123456789abcdef01', paired: false, deviceMode: false, connected: false, busy: false, kind: '', report: null, error: ''}
};
try{const saved=JSON.parse(sessionStorage.getItem('stromea-test-reload-backend') || 'null');if(saved)mockAPI.state=saved;}catch{}
window.fetch = async (path, options = {}) => {
  const m = window.mockAPI, body = options.body ? JSON.parse(options.body) : null;
  m.calls.push({path, body, method: options.method});
  let status = 200, result;
  switch (path) {
    case '/api/reports':result=[];break;
    case '/api/state': result = JSON.parse(JSON.stringify(m.state)); break;
    case '/api/prepare': if(m.prepareError)return {ok:false,status:409,text:async()=> 'Gerätewerte nicht lesbar.'};result={sample:{at:new Date().toISOString(),watts:-4000},originals:{LPP:{power:900,minimum:7200e9}}};break;
    case '/api/interfaces': result = [{name:'lo0',address:'127.0.0.1'},{name:'en0',address:'192.0.2.10'}]; break;
    case '/api/discover-lan':
      if (m.discoveryError) return {ok:false,status:500,text:async () => 'mDNS socket unavailable'};
      result = m.devices; break;
    case '/api/pair': m.state.paired = true; result = {}; break;
    case '/api/select': m.state = {...m.state, device: {...body.device,ski:body.device.ski.toLowerCase()}, deviceMode: true, paired: true, connected: false, report: null, error: '', kind: ''}; result = {}; break;
    case '/api/connect': m.state = {...m.state, capabilities:m.state.capabilities || {lpc:true,lpp:true,mpc:true}, trust:{device:!m.connectError,simulator:!m.connectError && !m.connectBusy}, kind: 'connect', reconnection:{active:m.connectBusy,attempt:1,deadline:new Date(Date.now()+60000).toISOString()},operation: 'Verbindung wird aufgebaut…', busy: m.connectBusy, connected: !m.connectBusy && !m.connectError, error: m.connectError}; status = 202; break;
    case '/api/simulator': m.state = {...m.state, kind: '', busy: false, connected: false, paired: false, deviceMode: false, error: '', report: null}; result = {}; break;
    case '/api/low-load':m.state.progress.lowLoad=null;result={};break;
    case '/api/discover-stop': m.state.search={active:false};result={};break;
    case '/api/cancel': if(body?.read){m.state={...m.state,kind:'',busy:false};result={};break;} m.state={...m.state,kind:'',busy:false,connected:false,connectionState:'disconnected',trust:{device:false,simulator:false},search:{active:false},reconnection:{active:false},error:'',operation:'Abgebrochen'};result={};break;
    case '/api/run':
      if (m.failRun) return {ok: false, status: 400, text: async () => 'Grenzen müssen mit ausreichendem Abstand über Failsafe liegen'};
      m.lastConfig = body; m.state = {...m.state, busy: true, kind: 'run', operation: 'Prüfung läuft', report: null, progress: {steps: [], current: 'Verbindung'}, error: ''}; status = 202; break;
    default: throw new Error('Unexpected network request blocked: ' + path);
  }
  return {ok: true, status, json: async () => result, text: async () => ''};
};
