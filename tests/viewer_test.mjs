import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const html = readFileSync(new URL('../plugins/transcription-agent-cli/mcp-server/viewer.html', import.meta.url), 'utf8');
const script = html.split('<script>')[1].split('</script>')[0];

// A small DOM/host fixture exercises bridge lifecycle and result handling
// without requiring browser downloads or speech-provider credentials in CI.
function fixture() {
  class Element {
    constructor(tag = 'div') { this.tag = tag; this.children = []; this.listeners = {}; this.value = ''; this.hidden = false; this.disabled = false; this.content = ''; this.classes = new Set(); this.classList = { add: x => this.classes.add(x), remove: x => this.classes.delete(x) }; }
    set textContent(value) { this.content = String(value); this.children = []; }
    get textContent() { return this.content + this.children.map(child => child.textContent).join(''); }
    replaceChildren() { this.content = ''; this.children = []; }
    appendChild(child) { this.children.push(child); return child; }
    addEventListener(name, handler) { this.listeners[name] = handler; }
    scrollIntoView() { this.scrolled = true; }
    getBoundingClientRect() { return { width: 680, height: 580 }; }
    fire(name, event = {}) { return this.listeners[name]?.(event); }
  }
  const elements = Object.fromEntries(['heading', 'metadata', 'format', 'file', 'path', 'search', 'previous', 'next', 'match-count', 'copy', 'load', 'notice', 'transcript', 'feedback'].map(id => [id, new Element()]));
  const styles = new Map(), messages = [], listeners = new Map(), timers = new Map();
  const parent = { postMessage: message => messages.push(message) };
  const copied = [], selection = { removeAllRanges() {}, addRange() { this.selected = true; } };
  const window = { parent, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name), getSelection: () => selection };
  const document = { getElementById: id => elements[id], createDocumentFragment: () => new Element(), createTextNode: text => { const node = new Element('#text'); node.textContent = text; return node; }, createElement: tag => new Element(tag), createRange: () => ({ selectNodeContents() {} }), body: new Element(), documentElement: { dataset: {}, style: { setProperty: (key, value) => styles.set(key, value), removeProperty: key => styles.delete(key) } } };
  const navigator = { clipboard: { writeText: async value => copied.push(value) } };
  let timerId = 1;
  const context = { window, document, navigator, Intl, setTimeout: fn => { const id = timerId++; timers.set(id, fn); return id; }, clearTimeout: id => timers.delete(id) };
  vm.runInNewContext(script, context);
  const send = data => listeners.get('message')?.({ source: parent, data: { jsonrpc: '2.0', ...data } });
  const initialize = async (resources = true) => {
    const request = messages.find(message => message.method === 'ui/initialize');
    send({ id: request.id, result: { protocolVersion: '2026-01-26', hostCapabilities: resources ? { serverResources: {} } : {}, hostContext: { theme: 'dark', styles: { variables: { '--color-text-primary': '#fff' } } } } });
    await Promise.resolve();
  };
  const show = overrides => send({ method: 'ui/notifications/tool-result', params: { structuredContent: { output_path: '/tmp/meeting.txt', format: 'txt', model: 'gemini-3.5-transcribe', bytes: 50, transcript: 'Speaker 1: Hello\nSpeaker 2: hello again\n', truncated: false, resource_uri: 'transcript://issued/id', ...overrides } } });
  return { elements, messages, send, show, initialize, copied, navigator, selection, document, styles, listeners, timers, parent };
}

test('negotiates MCP Apps, applies theme, and ignores foreign messages', async () => {
  const f = fixture();
  assert.equal(f.messages[0].method, 'ui/initialize');
  assert.equal(f.messages[0].params.appInfo.name, 'transcription-agent-viewer');
  await f.initialize();
  assert.equal(f.document.documentElement.dataset.theme, 'dark');
  assert.equal(f.styles.get('--color-text-primary'), '#fff');
  assert.equal(f.messages.at(-1).method, 'ui/notifications/initialized');
  f.listeners.get('message')({ source: {}, data: { jsonrpc: '2.0', method: 'ui/notifications/tool-result', params: { structuredContent: { transcript: 'foreign' } } } });
  assert.ok(!f.elements.transcript.textContent.includes('foreign'));
  f.send({ method: 'ui/notifications/host-context-changed', params: { theme: 'light' } });
  assert.equal(f.document.documentElement.dataset.theme, 'light');
});

test('renders untrusted transcript as text and literal searches navigate matches', async () => {
  const f = fixture(); await f.initialize();
  const text = '<script>alert(1)</script> [a.*] [A.*] İ hello';
  f.show({ transcript: text });
  assert.equal(f.elements.transcript.textContent, text);
  assert.equal(f.elements.transcript.children.length, 0);
  f.elements.search.value = '[a.*]'; f.elements.search.fire('input');
  assert.equal(f.elements['match-count'].textContent, '1/2');
  f.elements.next.fire('click'); assert.equal(f.elements['match-count'].textContent, '2/2');
  f.elements.previous.fire('click'); assert.equal(f.elements['match-count'].textContent, '1/2');
  f.elements.search.value = 'hello'; f.elements.search.fire('input');
  assert.equal(f.elements.transcript.textContent, text);
  assert.equal(f.elements['match-count'].textContent, '1/1');
  f.elements.search.value = 'missing'; f.elements.search.fire('input');
  assert.equal(f.elements['match-count'].textContent, '0 found');
  assert.equal(f.elements.next.disabled, true);
  f.elements.search.value = ''; f.elements.search.fire('input');
  assert.equal(f.elements.transcript.textContent, text);
});

test('labels shortened previews and loads only the issued full resource', async () => {
  const f = fixture(); await f.initialize(); f.show({ transcript: 'short', truncated: true });
  assert.equal(f.elements.copy.textContent, 'Copy preview');
  assert.equal(f.elements.load.hidden, false);
  assert.match(f.elements.notice.textContent, /shortened preview/);
  await f.elements.copy.fire('click'); assert.equal(f.copied[0], 'short');
  const loading = f.elements.load.fire('click');
  const request = f.messages.at(-1); assert.equal(request.method, 'resources/read');
  assert.equal(request.params.uri, 'transcript://issued/id');
  f.send({ id: request.id, result: { contents: [{ uri: request.params.uri, text: 'Full transcript text.' }] } });
  await loading;
  assert.equal(f.elements.transcript.textContent, 'Full transcript text.');
  assert.equal(f.elements.notice.textContent, '');
  assert.equal(f.elements.copy.textContent, 'Copy transcript');
  assert.equal(f.elements.load.hidden, true);
  await f.elements.copy.fire('click'); assert.equal(f.copied[1], 'Full transcript text.');
});

test('failed and mismatched full reads retain the usable preview', async () => {
  const f = fixture(); await f.initialize(); f.show({ transcript: 'preview', truncated: true });
  const first = f.elements.load.fire('click'), request = f.messages.at(-1);
  f.send({ id: request.id, error: { message: 'Access denied.' } }); await first;
  assert.equal(f.elements.transcript.textContent, 'preview');
  assert.match(f.elements.feedback.textContent, /Access denied/);
  const second = f.elements.load.fire('click'), retry = f.messages.at(-1);
  f.send({ id: retry.id, result: { contents: [{ uri: 'transcript://other/id', text: 'wrong document' }] } }); await second;
  assert.equal(f.elements.transcript.textContent, 'preview');
  assert.match(f.elements.feedback.textContent, /requested transcript/);
});

test('stale full reads cannot replace a newer transcription result', async () => {
  const f = fixture(); await f.initialize(); f.show({ transcript: 'old', truncated: true });
  const loading = f.elements.load.fire('click'), request = f.messages.at(-1);
  f.show({ transcript: 'new', truncated: false });
  f.send({ id: request.id, result: { contents: [{ uri: request.params.uri, text: 'old full transcript' }] } }); await loading;
  assert.equal(f.elements.transcript.textContent, 'new');
});

test('capability absence, clipboard rejection, failure, and cancellation degrade clearly', async () => {
  const f = fixture(); await f.initialize(false); f.show({ transcript: 'preview', truncated: true });
  assert.equal(f.elements.load.hidden, true);
  f.navigator.clipboard.writeText = async () => { throw new Error('Denied'); };
  await f.elements.copy.fire('click'); assert.equal(f.selection.selected, true);
  assert.match(f.elements.feedback.textContent, /Text selected/);
  f.send({ method: 'ui/notifications/tool-result', params: { isError: true, structuredContent: { error: { message: 'Missing provider key.' } } } });
  assert.equal(f.elements.heading.textContent, 'Transcription failed');
  assert.equal(f.elements.copy.disabled, true);
  assert.equal(f.elements.notice.textContent, 'Missing provider key.');
  f.send({ method: 'ui/notifications/tool-cancelled' });
  assert.equal(f.elements.notice.textContent, 'Transcription was cancelled.');
});

test('search is bounded and teardown acknowledges the host and releases listeners', async () => {
  const f = fixture(); await f.initialize(); f.show({ transcript: 'hello '.repeat(1200) });
  f.elements.search.value = 'hello'; f.elements.search.fire('input');
  assert.equal(f.elements['match-count'].textContent, '1/1000+');
  f.send({ method: 'ui/resource-teardown', id: 80 });
  assert.equal(f.listeners.has('message'), false);
  assert.equal(f.timers.size, 0);
  assert.equal(f.messages.at(-1).id, 80);
});

test('a publish failure shows a preserved recovery transcript without a success preview', async () => {
  const f = fixture(); await f.initialize(); f.show();
  const path = '/tmp/meeting.recovered-unique.txt';
  f.send({ method: 'ui/notifications/tool-result', params: { isError: true, structuredContent: { error: { message: 'The requested path already exists.', next_action: 'Read the recovery file before retrying.', recovery_output_path: path, resource_uri: 'transcript://issued/recovered' } } } });
  assert.equal(f.elements.heading.textContent, 'Transcription failed');
  assert.equal(f.elements.metadata.textContent, 'Transcript saved to recovery file.');
  assert.equal(f.elements.path.textContent, path);
  assert.equal(f.elements.file.hidden, false);
  assert.equal(f.elements.format.hidden, true);
  assert.match(f.elements.notice.textContent, /Read the recovery file before retrying/);
  assert.equal(f.elements.transcript.textContent, 'Open the saved recovery file to view the transcript.');
  assert.equal(f.elements.copy.disabled, true);
  assert.equal(f.elements.search.disabled, true);
  assert.equal(f.elements.load.hidden, true);
  f.send({ method: 'ui/notifications/tool-cancelled' });
  assert.equal(f.elements.file.hidden, true);
  assert.equal(f.elements.path.textContent, '');
});
