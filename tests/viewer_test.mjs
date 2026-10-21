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
    constructor(tag = 'div') { this.tag = tag; this.children = []; this.listeners = {}; this.attributes = {}; this.style = {}; this.className = ''; this.title = ''; this.value = ''; this.hidden = false; this.disabled = false; this.content = ''; this.classes = new Set(); this.classList = { add: x => this.classes.add(x), remove: x => this.classes.delete(x) }; }
    setAttribute(name, value) { this.attributes[name] = String(value); }
    getAttribute(name) { return this.attributes[name] ?? null; }
    set textContent(value) { this.content = String(value); this.children = []; }
    get textContent() { return this.content + this.children.map(child => child.textContent).join(''); }
    replaceChildren() { this.content = ''; this.children = []; }
    appendChild(child) { this.children.push(child); return child; }
    addEventListener(name, handler) { this.listeners[name] = handler; }
    scrollIntoView() { this.scrolled = true; }
    getBoundingClientRect() { return { width: 680, height: 580 }; }
    fire(name, event = {}) { return this.listeners[name]?.(event); }
  }
  const ids = ['app', 'heading', 'metadata', 'format', 'fullscreen', 'progress', 'progress-title', 'progress-detail', 'elapsed', 'file', 'file-label', 'path', 'speakers',
    'views', 'view-conversation', 'view-raw', 'search', 'previous', 'next', 'match-count', 'copy', 'load', 'notice', 'conversation', 'transcript',
    'ask', 'ask-summary', 'ask-actions', 'ask-decisions', 'feedback'];
  const elements = Object.fromEntries(ids.map(id => [id, new Element()]));
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
  const initialize = async (resources = true, protocolVersion = '2026-01-26', context = {}) => {
    const request = messages.find(message => message.method === 'ui/initialize');
    send({ id: request.id, result: { protocolVersion, hostCapabilities: resources ? { serverResources: {} } : {}, hostContext: { theme: 'dark', styles: { variables: { '--color-text-primary': '#fff' } }, ...context } } });
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

test('accepts the protocol version a host negotiates', async () => {
  const f = fixture();
  await f.initialize(true, '2025-06-18');
  assert.equal(f.messages.at(-1).method, 'ui/notifications/initialized');
  f.show({ truncated: true });
  assert.equal(f.elements.load.hidden, false);
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

// Reads the rendered conversation as [speaker, time, text] rows.
function turns(f) {
  return f.elements.conversation.children.filter(item => item.tag === 'article').map(item => {
    const [meta, paragraph] = item.children[1].children;
    const time = meta.children.find(child => child.tag === 'time');
    return [meta.children[0].textContent, time ? time.textContent : null, paragraph.textContent];
  });
}

test('timestamped text becomes a speaker conversation with stats and a raw file view', async () => {
  const f = fixture(); await f.initialize();
  const transcript = '[00:00:01] Speaker 1: Hello there.\n[00:00:04] Speaker 1: Shall we start?\n[00:01:30] Speaker 2: Yes, let us begin the review now.\n';
  f.show({ transcript, output_path: '/tmp/standup.txt' });
  assert.equal(f.elements.heading.textContent, 'standup');
  assert.equal(f.elements.metadata.textContent, 'Gemini 3.5 Transcribe · 1:30 · 2 speakers · 12 words');
  assert.equal(f.elements.views.hidden, false);
  assert.equal(f.elements.conversation.hidden, false);
  assert.equal(f.elements.transcript.hidden, true);
  assert.deepEqual(turns(f), [['Speaker 1', '0:01', 'Hello there. Shall we start?'], ['Speaker 2', '1:30', 'Yes, let us begin the review now.']]);
  const chips = f.elements.speakers.children;
  assert.deepEqual(chips.map(chip => chip.children[1].textContent), ['42%', '58%']);
  assert.notEqual(chips[0].className, chips[1].className);
  f.elements['view-raw'].fire('click');
  assert.equal(f.elements.transcript.hidden, false);
  assert.equal(f.elements.transcript.textContent, transcript);
  assert.equal(f.elements['view-raw'].getAttribute('aria-selected'), 'true');
});

test('every text export format parses into speaker turns', async () => {
  const cases = [
    ['json', JSON.stringify({ segments: [{ timestamp: '[00:00:02]', speaker: 'Ana', text: 'Hi.', start: 2.5 }, { timestamp: '[00:00:09]', speaker: 'Ben', text: 'Hello.' }], metadata: { duration: 75 } }), [['Ana', '0:02', 'Hi.'], ['Ben', '0:09', 'Hello.']]],
    ['csv', 'start,end,speaker,text\n00:00:01.000,00:00:03.000,Ana,"Hi, all ""team"""\n00:00:04.000,00:00:06.000,Ben,\'=SUM(A1)\n', [['Ana', '0:01', 'Hi, all "team"'], ['Ben', '0:04', '=SUM(A1)']]],
    ['srt', '1\n00:00:01,000 --> 00:00:03,000\nAna: Hello there\n\n2\n00:00:03,000 --> 00:00:05,000\nand welcome\n\n3\n00:00:06,000 --> 00:00:08,000\nBen: Thanks\n\n', [['Ana', '0:01', 'Hello there and welcome'], ['Ben', '0:06', 'Thanks']]],
    ['vtt', 'WEBVTT\n\n00:00:01.000 --> 00:00:03.000\n<v Ana &amp; Co>Fish &lt;3 chips\n\n00:00:04.000 --> 00:00:06.000\n<v Ben>Same\n\n', [['Ana & Co', '0:01', 'Fish <3 chips'], ['Ben', '0:04', 'Same']]],
    ['md', '# Standup\n\n_Duration 1:00 · 2 speakers_\n\n**Ana** · `0:05`\n\nUse the \\*real\\* config\n\n**Ben** · `0:40`\n\nAgreed\n', [['Ana', '0:05', 'Use the *real* config'], ['Ben', '0:40', 'Agreed']]],
    ['txt', 'Ana: First thought.\n\nBen: Second thought.\n', [['Ana', null, 'First thought.'], ['Ben', null, 'Second thought.']]],
  ];
  for (const [format, transcript, expected] of cases) {
    const f = fixture(); await f.initialize();
    f.show({ format, transcript });
    assert.deepEqual(turns(f), expected, format);
  }
  const f = fixture(); await f.initialize();
  f.show({ format: 'json', transcript: '{"segments": [{"speaker": "Ana", "text": "cut o', truncated: true });
  assert.equal(f.elements.views.hidden, true, 'a cut-off JSON preview falls back to the raw file');
  assert.equal(f.elements.transcript.textContent, '{"segments": [{"speaker": "Ana", "text": "cut o');
});

test('speaker chips filter turns, keep one speaker, and scope copy to what is shown', async () => {
  const f = fixture(); await f.initialize();
  f.show({ transcript: '[00:00:01] Ana: One.\n[00:00:02] Ben: Two.\n[00:00:03] Ana: Three.\n' });
  const [ana] = f.elements.speakers.children;
  ana.fire('click');
  assert.deepEqual(turns(f), [['Ben', '0:02', 'Two.']]);
  assert.equal(f.elements.speakers.children[0].getAttribute('aria-pressed'), 'false');
  assert.equal(f.elements.copy.textContent, 'Copy shown');
  await f.elements.copy.fire('click');
  assert.equal(f.copied.at(-1), 'Ben (0:02): Two.');
  f.elements.speakers.children[1].fire('click');
  assert.deepEqual(turns(f), [['Ben', '0:02', 'Two.']], 'the last visible speaker cannot be hidden');
  f.elements.speakers.children[0].fire('click');
  assert.equal(turns(f).length, 3);
  assert.equal(f.elements.copy.textContent, 'Copy transcript');
});

test('search highlights across turns and per-turn copy copies one turn', async () => {
  const f = fixture(); await f.initialize();
  f.show({ transcript: '[00:00:01] Ana: Budget is tight.\n[00:00:09] Ben: The budget review is Friday.\n' });
  f.elements.search.value = 'budget'; f.elements.search.fire('input');
  assert.equal(f.elements['match-count'].textContent, '1/2');
  f.elements.next.fire('click'); assert.equal(f.elements['match-count'].textContent, '2/2');
  f.elements.search.fire('keydown', { key: 'Escape', preventDefault() {} });
  assert.equal(f.elements.search.value, '');
  assert.equal(f.elements['match-count'].textContent, '');
  const copy = f.elements.conversation.children[1].children[1].children[0].children.at(-1);
  await copy.fire('click');
  assert.equal(f.copied.at(-1), 'Ben (0:09): The budget review is Friday.');
  assert.equal(f.elements.feedback.textContent, 'Turn copied.');
});

test('speaker names and text from the recording stay inert text', async () => {
  const f = fixture(); await f.initialize();
  f.show({ format: 'json', transcript: JSON.stringify({ segments: [{ speaker: '<img src=x onerror=alert(1)>', text: '<b>bold</b>', start: 0 }] }) });
  assert.deepEqual(turns(f), [['<img src=x onerror=alert(1)>', '0:00', '<b>bold</b>']]);
  assert.ok(f.elements.conversation.children.every(child => child.tag === 'article'));
});

test('tool input shows a live transcribing state that the result replaces', async () => {
  const f = fixture(); await f.initialize();
  f.send({ method: 'ui/notifications/tool-input', params: { arguments: { audio_path: '/Users/me/Recordings/call.m4a', model: 'MAI-Transcribe-2', format: 'srt' } } });
  assert.equal(f.elements.progress.hidden, false);
  assert.equal(f.elements['progress-title'].textContent, 'Transcribing call.m4a');
  assert.equal(f.elements['progress-detail'].textContent, 'Microsoft MAI-Transcribe-2 · SRT');
  assert.equal(f.elements.elapsed.textContent, '0:00');
  assert.equal(f.timers.size, 1);
  f.show();
  assert.equal(f.elements.progress.hidden, true);
  assert.equal(f.timers.size, 0);
  f.send({ method: 'ui/notifications/tool-input', params: { arguments: { audio_path: '/late.m4a' } } });
  assert.equal(f.elements.progress.hidden, true, 'late input does not hide a finished result');
});

test('ask buttons send a follow-up message naming the saved file', async () => {
  const f = fixture(); await f.initialize(); f.show();
  assert.equal(f.elements.ask.hidden, false);
  const asking = f.elements['ask-actions'].fire('click');
  const request = f.messages.at(-1);
  assert.equal(request.method, 'ui/message');
  assert.equal(request.params.role, 'user');
  assert.match(request.params.content[0].text, /action items in the transcript saved at \/tmp\/meeting\.txt/);
  f.send({ id: request.id, result: {} }); await asking;
  assert.equal(f.elements.feedback.textContent, 'Sent to the chat.');
  const denied = f.elements['ask-summary'].fire('click'), retry = f.messages.at(-1);
  f.send({ id: retry.id, error: { message: 'Message sending denied' } }); await denied;
  assert.equal(f.elements.feedback.textContent, 'Message sending denied');
  const offline = fixture(); offline.show();
  assert.equal(offline.elements.ask.hidden, true, 'no follow-ups before the host connects');
});

test('full screen is offered only when the host supports it', async () => {
  const plain = fixture(); await plain.initialize();
  assert.equal(plain.elements.fullscreen.hidden, true);
  const f = fixture(); await f.initialize(true, '2026-01-26', { displayMode: 'inline', availableDisplayModes: ['inline', 'fullscreen'], containerDimensions: { maxHeight: 600 } });
  assert.equal(JSON.stringify(f.messages[0].params.appCapabilities.availableDisplayModes), '["inline","fullscreen"]');
  assert.equal(f.elements.fullscreen.hidden, false);
  const switching = f.elements.fullscreen.fire('click'), request = f.messages.at(-1);
  assert.equal(request.method, 'ui/request-display-mode');
  assert.equal(request.params.mode, 'fullscreen');
  f.send({ id: request.id, result: { mode: 'fullscreen' } }); await switching;
  assert.equal(f.document.documentElement.dataset.mode, 'fullscreen');
  assert.equal(f.elements.fullscreen.getAttribute('aria-label'), 'Exit full screen');
  f.send({ method: 'ui/notifications/host-context-changed', params: { displayMode: 'inline', containerDimensions: { height: 500 } } });
  assert.equal(f.document.documentElement.dataset.mode, 'inline');
  assert.equal(f.document.documentElement.dataset.fill, 'true');
});
