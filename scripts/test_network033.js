// Network presentation and mode boundary; pure injected lists, no requests.
const fs=require('fs'),vm=require('vm'),assert=require('assert/strict');
const source=fs.readFileSync('internal/webui/assets/app.js','utf8');
const syntheticVPN='198.51.100.2'; // Documentation address; kind comes from the injected list, never from host adapters.
const ski='0123456789abcdef0123456789abcdef01234567';
const fields={ski:{value:ski},address:{value:'127.0.0.1:4711'}};
const ctx=vm.createContext({real:true,$:id=>fields[id]});
vm.runInContext(source.slice(source.indexOf('function typeLabel('),source.indexOf('function setCapabilities(')),ctx);
assert.equal(ctx.typeLabel('Generic'),'Gerätetyp nicht angegeben');
assert.equal(ctx.typeLabel(''),'Gerätetyp nicht angegeben');
assert.equal(ctx.typeLabel('HeatPump'),'Wärmepumpe');
assert.equal(ctx.typeLabel('VendorSpecific'),'Gerätetyp nicht angegeben');
vm.runInContext(source.match(/function backendSelectionMatches\(s\)\{[^\n]+/)[0],ctx);
const device={ski,address:fields.address.value};
assert.equal(ctx.backendSelectionMatches({device,deviceMode:false}),false,'Practice identity must not imply an onsite connection');
assert.equal(ctx.backendSelectionMatches({device,deviceMode:true}),true);
assert.equal(ctx.backendSelectionMatches({device:{...device,address:'127.0.0.1:4712'},deviceMode:true}),false);
console.log('033: matching identity also requires the onsite backend mode');

function select(){return {options:[],_value:'',replaceChildren(){this.options=[];this._value='';},append(o){this.options.push(o);if(this.options.length===1)this._value=o.value;},get selectedOptions(){return this.options.filter(o=>o.value===this._value);},get value(){return this._value;},set value(v){this._value=v;}};}
function option(){return {value:'',textContent:'',dataset:{},cloneNode(){return {...this,dataset:{...this.dataset}};}};}
fields.interface=select();fields['manual-interface']=select();for(const id of ['interface-tip','interface-single','manual-interface-single'])fields[id]={textContent:'',hidden:false};
ctx.controls=()=>{};
ctx.document={createElement:option};
ctx.api=async()=>[
 {name:'en1',address:'192.0.2.1',kind:'wifi'},
 {name:'en0',address:'169.254.1.2',kind:'ethernet'},
 {name:'utun4',address:syntheticVPN,kind:'vpn'},
 {name:'en7',address:'192.0.2.7',kind:'unknown'},
 {name:'loopback',address:'127.0.0.1',loopbackOnly:true},
];
vm.runInContext(source.slice(source.indexOf('async function loadInterfaces()'),source.indexOf("for (const id of ['mode-simulator'")),ctx);
(async()=>{
 await ctx.loadInterfaces();
 assert.equal(fields.interface.value,'en1');
 assert.deepEqual(fields.interface.options.map(o=>o.textContent),['WLAN · 192.0.2.1','Ethernet · 169.254.1.2','VPN · '+syntheticVPN,'Netzwerk · 192.0.2.7']);
 fields.interface.value='en0';fields['manual-interface'].value='utun4';
 await ctx.loadInterfaces();assert.equal(fields.interface.value,'en0');assert.equal(fields['manual-interface'].value,'utun4');
 console.log('033: hardware types label the supplied order; reload preserves explicit choices');
})().catch(e=>{console.error(e);process.exitCode=1;});
