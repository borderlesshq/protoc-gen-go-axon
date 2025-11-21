package src

const playgroundHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} Playground</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://unpkg.com/primevue@3.52.0/resources/themes/viva-dark/theme.css">
    <link rel="stylesheet" href="https://unpkg.com/primevue@3.52.0/resources/primevue.min.css">
    <link rel="stylesheet" href="https://unpkg.com/primeicons@6.0.1/primeicons.css">
    <script src="https://unpkg.com/vue@3.4.21/dist/vue.global.prod.js"></script>
    <script src="https://unpkg.com/primevue@3.52.0/core/core.min.js"></script>
    <script src="https://unpkg.com/primevue@3.52.0/datatable/datatable.min.js"></script>
    <script src="https://unpkg.com/primevue@3.52.0/column/column.min.js"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    fontFamily: {
                        mono: ['Menlo', 'Monaco', 'Consolas', 'monospace'],
                    }
                }
            }
        }
    </script>
</head>
<body class="dark">
    <div id="app" class="bg-gray-900 text-gray-300 h-screen overflow-hidden">
        <div class="flex h-screen">
            <!-- Sidebar -->
            <div class="w-80 bg-gray-800 border-r border-gray-700 flex flex-col">
                <div class="p-5 bg-gray-850 border-b border-gray-700">
                    <h1 class="text-lg font-semibold text-gray-200 mb-2">{{.Name}}</h1>
                    <p class="text-xs text-gray-500">RPC Playground</p>
                </div>
                
                <div class="flex-1 overflow-y-auto p-2.5">
                    <DataTable 
                        :value="methods" 
                        selection-mode="single"
                        v-model:selection="selectedMethod"
                        @row-select="onMethodSelect"
                        :pt="{
                            root: { class: 'bg-transparent border-0' },
                            header: { class: 'hidden' },
                            wrapper: { class: 'overflow-visible' },
                            table: { class: 'w-full' },
                            bodyRow: { class: 'cursor-pointer hover:bg-gray-700 transition-colors border-l-2 border-transparent hover:border-blue-500 data-[p-selected=true]:bg-blue-900 data-[p-selected=true]:border-blue-500' },
                            bodyCell: { class: 'p-3 border-0' }
                        }"
                    >
                        <Column field="name" header="Name">
                            <template #body="{ data }">
                                <div>
                                    <div class="text-sm text-gray-200 mb-1">[[ data.name ]]</div>
                                    <span :class="getStreamTypeClass(data.streamType)" class="text-[11px] px-2 py-0.5 rounded font-semibold">
                                        [[ formatStreamType(data.streamType) ]]
                                    </span>
                                </div>
                            </template>
                        </Column>
                    </DataTable>
                </div>
            </div>

            <!-- Main Content -->
            <div class="flex-1 flex flex-col">
                <!-- Empty State -->
                <div v-if="!selectedMethod" class="flex-1 flex flex-col items-center justify-center text-gray-500">
                    <div class="text-5xl mb-4 opacity-50">🚀</div>
                    <h2 class="text-xl font-semibold mb-2">Select a method</h2>
                    <p>Choose a method from the sidebar to test</p>
                </div>

                <!-- Method View -->
                <div v-else class="flex-1 flex flex-col">
                    <!-- Method Header -->
                    <div class="p-5 bg-gray-850 border-b border-gray-700">
                        <div class="text-xl font-semibold mb-2 text-gray-200">[[ selectedMethod.name ]]</div>
                        <div class="text-xs text-gray-500">[[ selectedMethod.streamType ]] • [[ selectedMethod.topic ]]</div>
                    </div>

                    <!-- Editor Container -->
                    <div class="flex-1 flex overflow-hidden">
                        <!-- Input Section -->
                        <div class="flex-1 flex flex-col border-r border-gray-700">
                            <div class="px-5 py-3 bg-gray-850 border-b border-gray-700 text-xs font-semibold text-gray-200 uppercase">
                                Request
                            </div>
                            <textarea 
                                v-model="requestBody"
                                class="flex-1 p-5 bg-gray-900 text-gray-300 border-0 font-mono text-sm resize-none outline-none"
                                placeholder='{"field": "value"}'
                            ></textarea>
                            <div class="flex gap-2.5 p-3 bg-gray-850 border-t border-gray-700">
                                <button 
                                    @click="executeMethod"
                                    :disabled="isExecuting"
                                    class="px-5 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white rounded text-sm font-semibold transition-colors"
                                >
                                    [[ isExecuting ? 'Executing...' : 'Execute' ]]
                                </button>
                                <input 
                                    v-model.number="timeout"
                                    type="number" 
                                    placeholder="Timeout (s)"
                                    class="w-24 px-2.5 py-2 bg-gray-700 border border-gray-600 text-gray-300 rounded text-sm"
                                >
                            </div>
                        </div>

                        <!-- Output Section -->
                        <div class="flex-1 flex flex-col">
                            <div class="px-5 py-3 bg-gray-850 border-b border-gray-700 text-xs font-semibold text-gray-200 uppercase">
                                Response
                            </div>
                            
                            <!-- Empty State -->
                            <div v-if="!response && !isStreaming" class="flex-1 flex flex-col items-center justify-center text-gray-500">
                                <div class="text-5xl mb-4 opacity-50">⚡</div>
                                <p>Response will appear here</p>
                            </div>

                            <!-- Stream Messages -->
                            <div v-else-if="isStreaming" class="flex-1 overflow-y-auto p-5">
                                <div v-for="(msg, idx) in streamMessages" :key="idx" 
                                     :class="['bg-gray-800 border-l-2 p-3 mb-2.5 rounded font-mono text-xs', msg.error ? 'border-red-500' : 'border-blue-500']">
                                    <div class="flex justify-between mb-2 text-gray-500 text-[11px]">
                                        <span>[[ msg.error ? 'Error' : ` + `Message #${idx + 1}` + ` ]]</span>
                                        <span>[[ msg.timestamp ]]ms</span>
                                    </div>
                                    <pre class="whitespace-pre-wrap">[[ msg.data ]]</pre>
                                </div>
                            </div>

                            <!-- Unary Response -->
                            <div v-else class="flex-1 overflow-y-auto p-5 font-mono text-sm whitespace-pre-wrap" :class="response.error ? 'text-red-400' : 'text-gray-300'">
                                [[ response.data ]]
                            </div>

                            <!-- Meta -->
                            <div v-if="response" class="px-5 py-3 bg-gray-800 border-t border-gray-700 text-xs flex gap-5">
                                <div class="flex gap-2">
                                    <span class="text-gray-500">Status:</span>
                                    <span :class="response.error ? 'text-red-400' : 'text-green-400'" class="font-semibold">
                                        [[ response.error ? 'Error' : 'Success' ]]
                                    </span>
                                </div>
                                <div class="flex gap-2">
                                    <span class="text-gray-500">Time:</span>
                                    <span class="text-blue-400 font-semibold">[[ response.duration ]]ms</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        const { createApp } = Vue;

        createApp({
			delimiters: ['[[', ']]'],
            data() {
                return {
                    methods: [],
                    selectedMethod: null,
                    requestBody: '{}',
                    timeout: 30,
                    response: null,
                    isExecuting: false,
                    isStreaming: false,
                    streamMessages: [],
                    streamStartTime: null
                }
            },
            mounted() {
                this.loadMethods();
            },
            methods: {
                async loadMethods() {
                    try {
                        const res = await fetch('/api/methods');
                        const data = await res.json();
                        this.methods = data.methods;
                    } catch (error) {
                        console.error('Failed to load methods:', error);
                    }
                },
                onMethodSelect(event) {
                    this.requestBody = '{}';
                    this.response = null;
                    this.streamMessages = [];
                    this.isStreaming = false;
                },
                getStreamTypeClass(type) {
                    const map = {
                        'UNARY': 'bg-green-700 text-white',
                        'SERVER_STREAMING': 'bg-yellow-700 text-white',
                        'CLIENT_STREAMING': 'bg-purple-700 text-white',
                        'BIDI_STREAMING': 'bg-red-700 text-white'
                    };
                    return map[type] || 'bg-gray-700 text-white';
                },
                formatStreamType(type) {
                    return type.replace(/_/g, ' ');
                },
                async executeMethod() {
                    if (!this.selectedMethod) return;

                    this.isExecuting = true;
                    this.response = null;
                    this.streamMessages = [];

                    try {
                        const payload = JSON.parse(this.requestBody);

                        if (this.selectedMethod.streamType === 'SERVER_STREAMING') {
                            await this.executeServerStream(payload);
                        } else if (this.selectedMethod.streamType === 'UNARY') {
                            await this.executeUnary(payload);
                        } else {
                            throw new Error(` + `${this.selectedMethod.streamType} not yet supported` + `);
                        }
                    } catch (error) {
                        this.response = { error: true, data: error.message, duration: 0 };
                    } finally {
                        this.isExecuting = false;
                    }
                },
                async executeUnary(payload) {
                    const startTime = Date.now();
                    const res = await fetch('/api/invoke', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            method: this.selectedMethod.name,
                            payload: payload,
                            headers: {},
                            timeout: this.timeout
                        })
                    });

                    const result = await res.json();
                    const duration = result.durationMs || (Date.now() - startTime);

                    if (result.success) {
                        this.response = {
                            error: false,
                            data: JSON.stringify(result.response, null, 2),
                            duration
                        };
                    } else {
                        this.response = {
                            error: true,
                            data: result.error,
                            duration
                        };
                    }
                },
                async executeServerStream(payload) {
                    this.isStreaming = true;
                    this.streamStartTime = Date.now();

                    const res = await fetch('/api/stream', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            method: this.selectedMethod.name,
                            payload: payload,
                            headers: {},
                            timeout: this.timeout
                        })
                    });

                    const reader = res.body.getReader();
                    const decoder = new TextDecoder();

                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;

                        const chunk = decoder.decode(value);
                        const lines = chunk.split('\n\n');

                        for (const line of lines) {
                            if (!line.trim()) continue;
                            const eventMatch = line.match(/^event: (.+)$/m);
                            const dataMatch = line.match(/^data: (.+)$/m);

                            if (eventMatch && dataMatch) {
                                const event = eventMatch[1];
                                const data = dataMatch[1];

                                if (event === 'message') {
                                    this.streamMessages.push({
                                        data: JSON.stringify(JSON.parse(data), null, 2),
                                        timestamp: Date.now() - this.streamStartTime,
                                        error: false
                                    });
                                } else if (event === 'error') {
                                    this.streamMessages.push({
                                        data: data,
                                        timestamp: Date.now() - this.streamStartTime,
                                        error: true
                                    });
                                } else if (event === 'close') {
                                    this.response = {
                                        error: false,
                                        data: '',
                                        duration: Date.now() - this.streamStartTime
                                    };
                                }
                            }
                        }
                    }

                    this.isStreaming = false;
                }
            }
        })
        .use(primevue.config.default)
        .component('DataTable', primevue.datatable)
        .component('Column', primevue.column)
        .mount('#app');
    </script>
</body>
</html>`
