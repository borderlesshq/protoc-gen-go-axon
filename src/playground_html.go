package src

// playgroundHTML is the web UI served by EnablePlayground. It is emitted into every
// generated *_axon.pb.go as a Go raw string and run through text/template, so:
//   - it must not contain backticks (no JS template literals),
//   - "{{" starts a template action ({{.Name}} is the service name); Vue uses [[ ]].
//
// Dependencies are pinned to exact versions so a CDN release can never break it.
const playgroundHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} · Axon Playground</title>
    <script src="https://cdn.tailwindcss.com/3.4.16"></script>
    <script src="https://unpkg.com/vue@3.5.13/dist/vue.global.prod.js"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    colors: {
                        ink: { 950: '#0b0d12', 900: '#10131a', 850: '#151922', 800: '#1b202b', 700: '#262c3a', 600: '#363e50' },
                    },
                    fontFamily: {
                        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
                        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
                    },
                },
            },
        };
    </script>
    <style>
        [v-cloak] { display: none; }
        textarea { tab-size: 2; }
        ::-webkit-scrollbar { width: 10px; height: 10px; }
        ::-webkit-scrollbar-thumb { background: #363e50; border-radius: 8px; border: 2px solid transparent; background-clip: padding-box; }
        ::-webkit-scrollbar-track { background: transparent; }
    </style>
</head>
<body class="bg-ink-950 text-slate-300 font-sans antialiased">
<div id="app" v-cloak class="flex h-screen overflow-hidden">

    <!-- Sidebar: service and methods -->
    <aside class="w-72 shrink-0 bg-ink-900 border-r border-ink-700 flex flex-col">
        <div class="px-5 pt-5 pb-4 border-b border-ink-700">
            <div class="text-[11px] uppercase tracking-wider text-slate-500 mb-1">Axon playground</div>
            <h1 class="text-base font-semibold text-slate-100 truncate" :title="service">[[ service ]]</h1>
            <div class="text-xs text-slate-500 mt-0.5">[[ methods.length ]] method[[ methods.length === 1 ? '' : 's' ]]</div>
        </div>
        <div class="p-3 border-b border-ink-700">
            <input v-model="filter" type="search" placeholder="Filter methods"
                   class="w-full px-3 py-2 bg-ink-850 border border-ink-700 rounded-md text-sm text-slate-200 placeholder-slate-500 outline-none focus:border-sky-500">
        </div>
        <nav class="flex-1 overflow-y-auto p-2">
            <div v-if="loadError" class="m-2 p-3 rounded-md bg-red-950/50 border border-red-900 text-xs text-red-300">[[ loadError ]]</div>
            <div v-else-if="!filteredMethods.length" class="px-3 py-6 text-center text-xs text-slate-500">No methods match.</div>
            <button v-for="m in filteredMethods" :key="m.name" type="button" @click="select(m)"
                    :class="['w-full text-left px-3 py-2.5 mb-1 rounded-md border transition-colors',
                             selected && selected.name === m.name ? 'bg-sky-500/10 border-sky-500/40' : 'border-transparent hover:bg-ink-800']">
                <div class="text-sm text-slate-100 truncate">[[ m.name ]]</div>
                <span :class="['inline-block mt-1 text-[10px] font-semibold tracking-wide px-1.5 py-0.5 rounded', kindClass(m.streamType)]">[[ kindLabel(m.streamType) ]]</span>
            </button>
        </nav>
    </aside>

    <!-- Main -->
    <main class="flex-1 min-w-0 flex flex-col">
        <div v-if="!selected" class="flex-1 flex flex-col items-center justify-center text-slate-500">
            <svg class="w-10 h-10 mb-4 opacity-40" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"/></svg>
            <div class="text-sm font-medium text-slate-300">Select a method</div>
            <div class="text-xs mt-1">Pick a method on the left to build a request.</div>
        </div>

        <template v-else>
            <!-- Method header -->
            <header class="px-6 py-4 border-b border-ink-700 bg-ink-900 flex items-start gap-4">
                <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                        <h2 class="text-lg font-semibold text-slate-100 truncate">[[ selected.name ]]</h2>
                        <span :class="['text-[10px] font-semibold tracking-wide px-1.5 py-0.5 rounded', kindClass(selected.streamType)]">[[ kindLabel(selected.streamType) ]]</span>
                    </div>
                    <div class="mt-1 text-xs text-slate-500 font-mono truncate">
                        [[ selected.topic ]] · [[ selected.inputType ]] → [[ selected.outputType ]]
                    </div>
                </div>
                <label class="flex items-center gap-2 text-xs text-slate-500 shrink-0">
                    Timeout
                    <input v-model.number="timeout" type="number" min="1"
                           class="w-16 px-2 py-1 bg-ink-850 border border-ink-700 rounded text-slate-200 text-xs outline-none focus:border-sky-500">
                    s
                </label>
            </header>

            <div class="flex-1 min-h-0 flex">
                <!-- Request -->
                <section class="flex-1 min-w-0 flex flex-col border-r border-ink-700">
                    <div class="px-5 h-10 flex items-center justify-between border-b border-ink-700 bg-ink-900">
                        <span class="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Request</span>
                        <div class="flex items-center gap-3 text-xs">
                            <span v-if="jsonError" class="text-red-400 truncate max-w-[16rem]" :title="jsonError">Invalid JSON</span>
                            <button type="button" @click="showHeaders = !showHeaders" class="text-slate-400 hover:text-slate-200">
                                Headers<span v-if="headerCount"> ([[ headerCount ]])</span>
                            </button>
                            <button type="button" @click="formatBody" class="text-slate-400 hover:text-slate-200">Format</button>
                        </div>
                    </div>
                    <div v-if="showHeaders" class="border-b border-ink-700 bg-ink-900/60 p-3">
                        <div v-for="(h, i) in headers" :key="i" class="flex gap-2 mb-2">
                            <input v-model="h.key" placeholder="Header" class="flex-1 min-w-0 px-2 py-1.5 bg-ink-850 border border-ink-700 rounded text-xs font-mono text-slate-200 outline-none focus:border-sky-500">
                            <input v-model="h.value" placeholder="Value" class="flex-[2] min-w-0 px-2 py-1.5 bg-ink-850 border border-ink-700 rounded text-xs font-mono text-slate-200 outline-none focus:border-sky-500">
                            <button type="button" @click="headers.splice(i, 1)" class="px-2 text-slate-500 hover:text-red-400" title="Remove">✕</button>
                        </div>
                        <button type="button" @click="headers.push({ key: '', value: '' })" class="text-xs text-sky-400 hover:text-sky-300">+ Add header</button>
                    </div>
                    <textarea v-model="body" @keydown="onEditorKey" spellcheck="false"
                              class="flex-1 min-h-0 p-5 bg-ink-950 text-slate-200 font-mono text-[13px] leading-relaxed resize-none outline-none"
                              placeholder='{ "field": "value" }'></textarea>
                    <div class="flex items-center gap-2 px-4 py-3 border-t border-ink-700 bg-ink-900">
                        <button type="button" @click="execute" :disabled="busy || streamOpen"
                                class="px-4 py-2 rounded-md text-sm font-semibold bg-sky-600 hover:bg-sky-500 text-white disabled:bg-ink-700 disabled:text-slate-500 disabled:cursor-not-allowed transition-colors">
                            [[ executeLabel ]]
                        </button>
                        <button v-if="sendsMessages" type="button" @click="sendMessage" :disabled="!streamOpen"
                                class="px-4 py-2 rounded-md text-sm font-semibold bg-emerald-600 hover:bg-emerald-500 text-white disabled:bg-ink-700 disabled:text-slate-500 disabled:cursor-not-allowed transition-colors">
                            Send message
                        </button>
                        <button v-if="sendsMessages && streamOpen" type="button" @click="closeStream"
                                class="px-4 py-2 rounded-md text-sm font-semibold bg-ink-700 hover:bg-ink-600 text-slate-200 transition-colors">
                            Close stream
                        </button>
                        <span class="ml-auto text-[11px] text-slate-500">[[ shortcut ]]</span>
                    </div>
                </section>

                <!-- Response -->
                <section class="flex-1 min-w-0 flex flex-col">
                    <div class="px-5 h-10 flex items-center justify-between border-b border-ink-700 bg-ink-900">
                        <span class="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Response</span>
                        <div class="flex items-center gap-3 text-xs">
                            <button v-if="copyText" type="button" @click="copy" class="text-slate-400 hover:text-slate-200">[[ copied ? 'Copied' : 'Copy' ]]</button>
                            <button v-if="log.length || result" type="button" @click="clearOutput" class="text-slate-400 hover:text-slate-200">Clear</button>
                        </div>
                    </div>

                    <div v-if="!result && !log.length" class="flex-1 flex flex-col items-center justify-center text-slate-500 text-sm">
                        <span v-if="busy">Waiting for a response…</span>
                        <span v-else>The response will appear here.</span>
                    </div>

                    <!-- Stream log -->
                    <div v-else-if="log.length" ref="logEl" class="flex-1 min-h-0 overflow-y-auto p-4 space-y-2">
                        <div v-for="(e, i) in log" :key="i" :class="['rounded-md border-l-2 bg-ink-900 px-3 py-2', entryBorder(e.kind)]">
                            <div class="flex justify-between text-[11px] mb-1">
                                <span :class="entryText(e.kind)">[[ e.label ]]</span>
                                <span class="text-slate-500 font-mono">+[[ e.at ]]ms</span>
                            </div>
                            <pre class="font-mono text-xs text-slate-200 whitespace-pre-wrap break-words">[[ e.data ]]</pre>
                        </div>
                    </div>

                    <!-- Unary result -->
                    <pre v-else :class="['flex-1 min-h-0 overflow-auto p-5 font-mono text-[13px] whitespace-pre-wrap break-words', result.error ? 'text-red-300' : 'text-slate-200']">[[ result.data ]]</pre>

                    <div v-if="result || streamOpen" class="px-5 py-2.5 border-t border-ink-700 bg-ink-900 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs">
                        <span v-if="streamOpen" class="flex items-center gap-1.5 text-emerald-400"><span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>Stream open</span>
                        <span v-if="result"><span class="text-slate-500">Status </span><span :class="result.error ? 'text-red-400' : 'text-emerald-400'" class="font-semibold">[[ result.error ? 'Error' : 'OK' ]]</span></span>
                        <span v-if="result"><span class="text-slate-500">Time </span><span class="text-sky-400 font-semibold">[[ result.duration ]]ms</span></span>
                        <span v-for="(v, k) in (result && result.headers) || {}" :key="k" class="font-mono"><span class="text-slate-500">[[ k ]]: </span>[[ v ]]</span>
                    </div>
                </section>
            </div>
        </template>
    </main>
</div>

<script>
(function () {
    var _Vue = Vue, createApp = _Vue.createApp, ref = _Vue.ref, computed = _Vue.computed, nextTick = _Vue.nextTick, onMounted = _Vue.onMounted;

    var KINDS = {
        UNARY: { label: 'UNARY', cls: 'bg-emerald-500/15 text-emerald-300' },
        SERVER_STREAMING: { label: 'SERVER STREAM', cls: 'bg-amber-500/15 text-amber-300' },
        CLIENT_STREAMING: { label: 'CLIENT STREAM', cls: 'bg-violet-500/15 text-violet-300' },
        BIDI_STREAMING: { label: 'BIDI STREAM', cls: 'bg-rose-500/15 text-rose-300' },
    };
    var ENTRY = {
        sent: { border: 'border-emerald-500', text: 'text-emerald-400' },
        received: { border: 'border-sky-500', text: 'text-sky-400' },
        final: { border: 'border-violet-500', text: 'text-violet-400' },
        system: { border: 'border-amber-500', text: 'text-amber-400' },
        error: { border: 'border-red-500', text: 'text-red-400' },
    };

    function pretty(v) {
        if (typeof v === 'string') {
            try { return JSON.stringify(JSON.parse(v), null, 2); } catch (e) { return v; }
        }
        return JSON.stringify(v, null, 2);
    }

    // postJSON never throws on a non-JSON body: http.Error answers are plain text.
    function postJSON(url, body) {
        return fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
            .then(function (res) {
                return res.text().then(function (text) {
                    try { return JSON.parse(text); }
                    catch (e) { return { success: false, error: (text || res.statusText).trim() + ' (HTTP ' + res.status + ')' }; }
                });
            });
    }

    createApp({
        delimiters: ['[[', ']]'],
        setup: function () {
            var service = ref('{{.Name}}');
            var methods = ref([]);
            var loadError = ref('');
            var filter = ref('');
            var selected = ref(null);
            var drafts = {};
            var body = ref('{}');
            var headers = ref([]);
            var showHeaders = ref(false);
            var timeout = ref(30);
            var busy = ref(false);
            var result = ref(null);
            var log = ref([]);
            var logEl = ref(null);
            var streamOpen = ref(false);
            var streamId = ref(null);
            var copied = ref(false);
            var started = 0;
            var es = null;
            var received = 0;

            var isMac = /Mac|iPhone|iPad/.test(navigator.platform);
            var shortcut = (isMac ? '⌘' : 'Ctrl') + ' + Enter to run';

            var filteredMethods = computed(function () {
                var q = filter.value.trim().toLowerCase();
                return q ? methods.value.filter(function (m) { return m.name.toLowerCase().indexOf(q) !== -1; }) : methods.value;
            });
            var kind = computed(function () { return selected.value ? selected.value.streamType : ''; });
            var sendsMessages = computed(function () { return kind.value === 'CLIENT_STREAMING' || kind.value === 'BIDI_STREAMING'; });
            var executeLabel = computed(function () {
                if (busy.value) return 'Running…';
                if (streamOpen.value) return 'Stream open';
                return sendsMessages.value ? 'Open stream' : 'Run';
            });
            var jsonError = computed(function () {
                try { JSON.parse(body.value || '{}'); return ''; } catch (e) { return e.message; }
            });
            var headerCount = computed(function () { return headers.value.filter(function (h) { return h.key.trim(); }).length; });
            var copyText = computed(function () {
                if (log.value.length) return log.value.map(function (e) { return e.data; }).join('\n\n');
                return result.value ? result.value.data : '';
            });

            function timeoutSeconds() { var t = Number(timeout.value); return t > 0 ? Math.round(t) : 30; }

            function headerMap() {
                var out = {};
                headers.value.forEach(function (h) { if (h.key.trim()) out[h.key.trim()] = h.value; });
                return out;
            }
            function elapsed() { return Date.now() - started; }
            function push(kindName, label, data) {
                log.value.push({ kind: kindName, label: label, data: data, at: elapsed() });
                nextTick(function () { if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight; });
            }
            function stopEventSource() { if (es) { es.close(); es = null; } }
            function resetStream() { streamOpen.value = false; streamId.value = null; stopEventSource(); }
            function clearOutput() { result.value = null; log.value = []; received = 0; }

            function select(m) {
                if (selected.value) drafts[selected.value.name] = { body: body.value, headers: headers.value };
                // Close a stream left open on the previous method without reporting into the new view.
                if (streamOpen.value && streamId.value) {
                    var url = kind.value === 'BIDI_STREAMING' ? '/api/bidi-stream/close' : '/api/client-stream/close';
                    postJSON(url, { streamId: streamId.value, method: selected.value.name }).catch(function () {});
                }
                resetStream();
                clearOutput();
                selected.value = m;
                var d = drafts[m.name];
                body.value = d ? d.body : '{}';
                headers.value = d ? d.headers : [];
            }

            function formatBody() {
                try { body.value = JSON.stringify(JSON.parse(body.value || '{}'), null, 2); } catch (e) { /* jsonError shows it */ }
            }

            function onEditorKey(e) {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    if (streamOpen.value && sendsMessages.value) sendMessage(); else execute();
                    return;
                }
                if (e.key === 'Tab') {
                    e.preventDefault();
                    var t = e.target, s = t.selectionStart;
                    body.value = body.value.slice(0, s) + '  ' + body.value.slice(t.selectionEnd);
                    nextTick(function () { t.selectionStart = t.selectionEnd = s + 2; });
                }
            }

            function copy() {
                if (!navigator.clipboard) return;
                navigator.clipboard.writeText(copyText.value).then(function () {
                    copied.value = true;
                    setTimeout(function () { copied.value = false; }, 1200);
                });
            }

            function finish(error, data, extraHeaders) {
                // With a stream log on screen the result body is hidden, so surface the error there.
                if (error && log.value.length && log.value[log.value.length - 1].kind !== 'error') push('error', 'Error', data);
                result.value = { error: !!error, data: data, duration: elapsed(), headers: extraHeaders || null };
            }

            function runUnary(payload) {
                return postJSON('/api/invoke', { method: selected.value.name, payload: payload, headers: headerMap(), timeout: timeoutSeconds() })
                    .then(function (r) {
                        result.value = r.success
                            ? { error: false, data: pretty(r.response), duration: r.durationMs || elapsed(), headers: r.headers || null }
                            : { error: true, data: r.error, duration: r.durationMs || elapsed(), headers: r.headers || null };
                    });
            }

            // Server streaming arrives as SSE over a POST, so it is read by hand. Events can be
            // split across network chunks: keep the unterminated tail in a buffer.
            function runServerStream(payload) {
                return fetch('/api/stream', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ method: selected.value.name, payload: payload, headers: headerMap(), timeout: timeoutSeconds() }),
                }).then(function (res) {
                    if (!res.body) { finish(true, 'Streaming is not supported by this browser'); return; }
                    streamOpen.value = true;
                    var reader = res.body.getReader(), decoder = new TextDecoder(), buf = '', closed = false;
                    function handle(block) {
                        var ev = 'message', data = [];
                        block.split('\n').forEach(function (line) {
                            if (line.indexOf('event:') === 0) ev = line.slice(6).trim();
                            else if (line.indexOf('data:') === 0) data.push(line.slice(5).replace(/^ /, ''));
                        });
                        var d = data.join('\n');
                        if (ev === 'message') { received++; push('received', 'Received #' + received, pretty(d)); }
                        else if (ev === 'started') push('system', 'Started', d);
                        else if (ev === 'error') { push('error', 'Error', d); closed = true; finish(true, d); }
                        else if (ev === 'close') { closed = true; finish(false, d || 'Stream completed'); }
                    }
                    function pump() {
                        return reader.read().then(function (r) {
                            if (r.done) {
                                if (buf.trim()) handle(buf);
                                if (!closed) finish(false, 'Stream ended');
                                streamOpen.value = false;
                                return;
                            }
                            buf += decoder.decode(r.value, { stream: true }).replace(/\r\n/g, '\n');
                            var i;
                            while ((i = buf.indexOf('\n\n')) !== -1) {
                                var block = buf.slice(0, i);
                                buf = buf.slice(i + 2);
                                if (block.trim()) handle(block);
                            }
                            return pump();
                        });
                    }
                    return pump();
                });
            }

            function runClientStream() {
                return postJSON('/api/client-stream/init', { method: selected.value.name, headers: headerMap(), timeout: timeoutSeconds() })
                    .then(function (r) {
                        if (!r.success) { finish(true, r.error); return; }
                        streamId.value = r.streamId;
                        streamOpen.value = true;
                        push('system', 'Stream opened', r.streamId);
                        return sendMessage();
                    });
            }

            function runBidiStream() {
                var url = '/api/bidi-stream?method=' + encodeURIComponent(selected.value.name) + '&timeout=' + encodeURIComponent(timeoutSeconds());
                es = new EventSource(url);
                es.addEventListener('started', function (e) {
                    streamId.value = e.data;
                    streamOpen.value = true;
                    push('system', 'Stream opened', e.data);
                });
                es.addEventListener('message', function (e) { received++; push('received', 'Received #' + received, pretty(e.data)); });
                es.addEventListener('close', function (e) {
                    finish(false, e.data || 'Stream completed');
                    resetStream();
                });
                // Fires both for server "error" events (which carry data) and for a dropped
                // connection (which does not). Either way the stream is over; EventSource
                // must not be left to reconnect and open a fresh stream.
                es.addEventListener('error', function (e) {
                    if (!es) return;
                    var msg = e.data || 'Connection closed';
                    push('error', 'Error', msg);
                    finish(true, msg);
                    resetStream();
                });
            }

            function sendMessage() {
                if (!streamOpen.value || !streamId.value) return Promise.resolve();
                var payload;
                try { payload = JSON.parse(body.value || '{}'); } catch (e) { push('error', 'Invalid JSON', e.message); return Promise.resolve(); }
                var url = kind.value === 'BIDI_STREAMING' ? '/api/bidi-stream/send' : '/api/client-stream/send';
                return postJSON(url, { streamId: streamId.value, method: selected.value.name, payload: payload })
                    .then(function (r) {
                        if (r.success) push('sent', 'Sent', pretty(payload));
                        else push('error', 'Send failed', r.error);
                    })
                    .catch(function (e) { push('error', 'Send failed', e.message); });
            }

            function closeStream() {
                if (!streamOpen.value || !streamId.value) return Promise.resolve();
                var url = kind.value === 'BIDI_STREAMING' ? '/api/bidi-stream/close' : '/api/client-stream/close';
                var id = streamId.value, name = selected.value.name, bidi = kind.value === 'BIDI_STREAMING';
                return postJSON(url, { streamId: id, method: name })
                    .then(function (r) {
                        if (!r.success) { finish(true, r.error); return; }
                        if (!bidi && r.response) push('final', 'Final response', pretty(r.response));
                        finish(false, r.response ? pretty(r.response) : 'Stream closed');
                    })
                    .catch(function (e) { finish(true, e.message); })
                    .then(resetStream);
            }

            function execute() {
                if (!selected.value || busy.value || streamOpen.value || es) return;
                var payload;
                try { payload = JSON.parse(body.value || '{}'); } catch (e) { clearOutput(); started = Date.now(); finish(true, 'Request body is not valid JSON: ' + e.message); return; }
                clearOutput();
                started = Date.now();
                busy.value = true;
                var run;
                switch (kind.value) {
                    case 'UNARY': run = runUnary(payload); break;
                    case 'SERVER_STREAMING': run = runServerStream(payload); break;
                    case 'CLIENT_STREAMING': run = runClientStream(); break;
                    case 'BIDI_STREAMING': runBidiStream(); run = Promise.resolve(); break;
                    default: run = Promise.reject(new Error(kind.value + ' is not supported'));
                }
                run.catch(function (e) { finish(true, e.message); resetStream(); })
                   .then(function () { busy.value = false; });
            }

            function kindLabel(t) { return (KINDS[t] || { label: t }).label; }
            function kindClass(t) { return (KINDS[t] || { cls: 'bg-ink-700 text-slate-300' }).cls; }
            function entryBorder(k) { return ENTRY[k].border; }
            function entryText(k) { return ENTRY[k].text; }

            onMounted(function () {
                fetch('/api/methods')
                    .then(function (res) { if (!res.ok) throw new Error('HTTP ' + res.status); return res.json(); })
                    .then(function (data) {
                        if (data.service) service.value = data.service;
                        methods.value = data.methods || [];
                        if (methods.value.length === 1) select(methods.value[0]);
                    })
                    .catch(function (e) { loadError.value = 'Could not load methods: ' + e.message; });
                window.addEventListener('beforeunload', stopEventSource);
            });

            return {
                service: service, methods: methods, loadError: loadError, filter: filter, filteredMethods: filteredMethods,
                selected: selected, body: body, headers: headers, showHeaders: showHeaders, headerCount: headerCount,
                timeout: timeout, busy: busy, result: result, log: log, logEl: logEl, streamOpen: streamOpen,
                sendsMessages: sendsMessages, executeLabel: executeLabel, jsonError: jsonError, shortcut: shortcut,
                copyText: copyText, copied: copied,
                select: select, formatBody: formatBody, onEditorKey: onEditorKey, copy: copy, clearOutput: clearOutput,
                execute: execute, sendMessage: sendMessage, closeStream: closeStream,
                kindLabel: kindLabel, kindClass: kindClass, entryBorder: entryBorder, entryText: entryText,
            };
        },
    }).mount('#app');
})();
</script>
</body>
</html>`
