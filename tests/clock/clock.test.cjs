const {test} = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname,'../../internal/clock/scripts/clock.js'),'utf8');
function clock(instant, overrides = {}, intl = Intl, hidden = false) {
  let now = Date.parse(instant), next = 0, tasks = new Map();
  const events = {}, windowEvents = {};
  const root = {dataset:{dynamic:'true',seconds:'true',locale:'zh',mode:'custom',zone:'Asia/Shanghai',greetings:'morning;noon;afternoon;night',greetingFallback:'fallback',...overrides},hasAttribute(name){return name === 'data-greetings' && this.dataset.greetings !== undefined;}};
  const nodes = {'clock':root,'clock-date':{textContent:''},'clock-day':{textContent:''},'clock-time':{textContent:''},'plugin-greetings':{textContent:'fixed help title'}};
  const document = {hidden,getElementById:id=>nodes[id],addEventListener:(name,fn)=>events[name]=fn};
  class FakeDate extends Date {constructor(...args){super(...(args.length?args:[now]));}static now(){return now;}}
  const context = vm.createContext({document,window:{addEventListener:(name,fn)=>windowEvents[name]=fn},Date:FakeDate,Intl:intl,setTimeout:(fn,delay)=>{tasks.set(++next,{fn,delay});return next;},clearTimeout:id=>tasks.delete(id)});
  const run = ()=>vm.runInContext(source,context);
  run();
  return {root,nodes,document,tasks,run,setTime:s=>now=Date.parse(s),tick(){assert.equal(tasks.size,1);[...tasks.values()][0].fn();},emit:n=>events[n](),window:n=>windowEvents[n](),text:id=>nodes['clock-'+id].textContent};
}
for (const [instant,zone,time,date,day] of [
  ['2026-12-31T15:59:59Z','Asia/Shanghai','23:59:59','2026年12月31日','星期四'],
  ['2026-12-31T16:00:00Z','Asia/Shanghai','00:00:00','2027年01月01日','星期五'],
  ['2024-02-28T16:00:00Z','Asia/Shanghai','00:00:00','2024年02月29日','星期四'],
  ['2026-03-08T06:59:59Z','America/New_York','01:59:59','2026年03月08日','星期日'],
  ['2026-03-08T07:00:00Z','America/New_York','03:00:00','2026年03月08日','星期日'],
  ['2026-11-01T05:59:59Z','America/New_York','01:59:59','2026年11月01日','星期日'],
  ['2026-11-01T06:00:00Z','America/New_York','01:00:00','2026年11月01日','星期日'],
  ['2026-10-25T00:00:00Z','Asia/Kathmandu','05:45:00','2026年10月25日','星期日']
]) test(`${instant} in ${zone}`,()=>{const c=clock(instant,{zone});assert.equal(c.text('time'),time);assert.equal(c.text('date'),date);assert.equal(c.text('day'),day);});
test('midnight updates all fields together, without accumulating ticks',()=>{const c=clock('2026-12-31T15:59:59Z');c.setTime('2026-12-31T16:30:00Z');c.tick();assert.equal(c.text('time'),'00:30:00');assert.equal(c.text('date'),'2027年01月01日');assert.equal(c.text('day'),'星期五');assert.equal(c.tasks.size,1);});
test('hidden tabs stop and resume from current device time',()=>{const c=clock('2026-10-25T00:00:00Z');c.document.hidden=true;c.emit('visibilitychange');assert.equal(c.tasks.size,0);c.setTime('2026-10-26T04:30:20Z');c.document.hidden=false;c.emit('visibilitychange');assert.equal(c.text('time'),'12:30:20');assert.equal(c.tasks.size,1);});
test('BFCache, focus and repeated initialization cannot duplicate timers',()=>{const c=clock('2026-10-25T00:00:00Z');c.run();c.window('pageshow');c.window('focus');assert.equal(c.tasks.size,1);c.window('pagehide');assert.equal(c.tasks.size,0);c.setTime('2026-10-25T03:00:00Z');c.window('pageshow');assert.equal(c.text('time'),'11:00:00');assert.equal(c.nodes['plugin-greetings'].textContent,'noon');assert.equal(c.tasks.size,1);});
test('minute mode aligns the next update without second polling',()=>{const c=clock('2026-10-25T00:00:20.500Z',{seconds:'false'});assert.equal(c.text('time'),'08:00');assert.equal([...c.tasks.values()][0].delay,39515);c.setTime('2026-10-25T00:01:00Z');c.tick();assert.equal(c.text('time'),'08:01');});
test('static browser time initializes once, with no recurring work',()=>{const c=clock('2026-10-25T00:00:00Z',{dynamic:'false',mode:'browser'});assert.equal(c.tasks.size,0);assert.ok(c.text('time'));});
test('English format matches Go and midnight uses 00',()=>{const c=clock('2026-10-25T00:00:00Z',{locale:'en',zone:'UTC'});assert.equal(c.text('date'),'Oct 25, 2026');assert.equal(c.text('day'),'Sunday');assert.equal(c.text('time'),'00:00:00');});
test('unsupported zone or Intl leaves fallback untouched',()=>{for(const [o,intl] of [[{zone:'Local'},Intl],[{},{}]]){const c=clock('2026-10-25T00:00:00Z',o,intl);assert.equal(c.tasks.size,0);assert.equal(c.nodes['plugin-greetings'].textContent,'fixed help title');assert.equal(c.root.dataset.initialized,undefined);}});
test('fixed help title has no greeting data and stays unchanged',()=>{const c=clock('2026-10-25T00:00:00Z',{greetings:undefined});assert.equal(c.nodes['plugin-greetings'].textContent,'fixed help title');});
test('fixed and incomplete greetings do not break clock',()=>{for(const [g,want] of [['fixed','fixed'],['morning;noon','fallback']]){const c=clock('2026-10-25T07:00:00Z',{greetings:g});assert.equal(c.nodes['plugin-greetings'].textContent,want);assert.equal(c.tasks.size,1);}});

test('static device clock initializes in an initially hidden tab without recurring work',()=>{
 const instant='2026-10-25T00:00:00Z';
 const c=clock(instant,{dynamic:'false',mode:'browser',locale:'en'},Intl,true);
 const expected=new Intl.DateTimeFormat('en-US',{hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'}).format(new Date(instant));
 assert.equal(c.text('time'),expected);
 assert.ok(c.text('date'));
 assert.notEqual(c.nodes['plugin-greetings'].textContent,'fixed help title');
 assert.equal(c.tasks.size,0);
});
test('dynamic clock opened hidden waits for visibility and starts exactly one timer',()=>{
 const c=clock('2026-10-25T00:00:00Z',{},Intl,true);
 assert.equal(c.text('time'),'');assert.equal(c.tasks.size,0);
 c.setTime('2026-10-25T01:00:00Z');c.document.hidden=false;c.emit('visibilitychange');
 assert.equal(c.text('time'),'09:00:00');assert.equal(c.tasks.size,1);
});
