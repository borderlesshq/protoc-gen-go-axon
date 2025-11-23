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
    <script src="https://unpkg.com/vue@3/dist/vue.global.js"></script>
    <script src="https://unpkg.com/primevue/umd/primevue.min.js"></script>
    <script src="https://unpkg.com/@primevue/themes/umd/aura.min.js"></script>
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
                    <p-datatable 
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
                    </p-datatable>
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
                                    v-if="isClientStreaming || isBidiStreaming"
                                    @click="sendStreamMessage"
                                    :disabled="!isStreamActive"
                                    class="px-5 py-2.5 bg-green-600 hover:bg-green-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white rounded text-sm font-semibold transition-colors"
                                >
                                    Send Message
                                </button>
                                <button
                                    @click="executeMethod"
                                    :disabled="isExecuting"
                                    class="px-5 py-2.5 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white rounded text-sm font-semibold transition-colors"
                                >
                                    [[ getExecuteButtonLabel() ]]
                                </button>
                                <button
                                    v-if="(isClientStreaming || isBidiStreaming) && isStreamActive"
                                    @click="closeStream"
                                    class="px-5 py-2.5 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-semibold transition-colors"
                                >
                                    Close Stream
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
                                     :class="['bg-gray-800 border-l-2 p-3 mb-2.5 rounded font-mono text-xs',
                                              msg.error ? 'border-red-500' :
                                              msg.sent ? 'border-green-500' :
                                              msg.final ? 'border-purple-500' :
                                              msg.system ? 'border-yellow-500' :
                                              'border-blue-500']">
                                    <div class="flex justify-between mb-2 text-gray-500 text-[11px]">
                                        <span v-if="msg.error" class="text-red-400">Error</span>
                                        <span v-else-if="msg.sent" class="text-green-400">Sent</span>
                                        <span v-else-if="msg.final" class="text-purple-400">Final Response</span>
                                        <span v-else-if="msg.system" class="text-yellow-400">System</span>
										<span v-else v-text="'Received #' + (idx + 1)"></span>
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
const { createApp, ref, onMounted, computed } = Vue;

createApp({
    delimiters: ['[[', ']]'],
    setup() {
        // Reactive state
        const methods = ref([]);
        const selectedMethod = ref(null);
        const requestBody = ref('{}');
        const timeout = ref(30);
        const response = ref(null);
        const isExecuting = ref(false);
        const isStreaming = ref(false);
        const streamMessages = ref([]);
        const streamStartTime = ref(null);
        const isStreamActive = ref(false);
        const streamId = ref(null);
        const eventSource = ref(null);

        // Methods
        const loadMethods = async () => {
            try {
                const res = await fetch('/api/methods');
                const data = await res.json();
                methods.value = data.methods;
            } catch (error) {
                console.error('Failed to load methods:', error);
            }
        };

        const onMethodSelect = (event) => {
            requestBody.value = '{}';
            response.value = null;
            streamMessages.value = [];
            isStreaming.value = false;
            isStreamActive.value = false;
            streamId.value = null;
            if (eventSource.value) {
                eventSource.value.close();
                eventSource.value = null;
            }
        };

        const isClientStreaming = computed(() => {
            return selectedMethod.value &&
                   (selectedMethod.value.streamType === 'CLIENT_STREAMING' ||
                    selectedMethod.value.streamType === 'BIDI_STREAMING');
        });

        const isBidiStreaming = computed(() => {
            return selectedMethod.value && selectedMethod.value.streamType === 'BIDI_STREAMING';
        });

        const getExecuteButtonLabel = () => {
            if (isExecuting.value) return 'Executing...';
            if (isStreamActive.value) return 'Stream Active';
            if (isClientStreaming.value || isBidiStreaming.value) {
                return 'Start Stream';
            }
            return 'Execute';
        };

        const getStreamTypeClass = (type) => {
            const map = {
                'UNARY': 'bg-green-700 text-white',
                'SERVER_STREAMING': 'bg-yellow-700 text-white',
                'CLIENT_STREAMING': 'bg-purple-700 text-white',
                'BIDI_STREAMING': 'bg-red-700 text-white'
            };
            return map[type] || 'bg-gray-700 text-white';
        };

        const formatStreamType = (type) => {
            return type.replace(/_/g, ' ');
        };

        const executeUnary = async (payload) => {
            const startTime = Date.now();
            const res = await fetch('/api/invoke', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    method: selectedMethod.value.name,
                    payload: payload,
                    headers: {},
                    timeout: timeout.value
                })
            });

            const result = await res.json();
            const duration = result.durationMs || (Date.now() - startTime);

            if (result.success) {
                response.value = {
                    error: false,
                    data: JSON.stringify(result.response, null, 2),
                    duration
                };
            } else {
                response.value = {
                    error: true,
                    data: result.error,
                    duration
                };
            }
        };

        const executeServerStream = async (payload) => {
            isStreaming.value = true;
            streamStartTime.value = Date.now();

            const res = await fetch('/api/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    method: selectedMethod.value.name,
                    payload: payload,
                    headers: {},
                    timeout: timeout.value
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
                            streamMessages.value.push({
                                data: JSON.stringify(JSON.parse(data), null, 2),
                                timestamp: Date.now() - streamStartTime.value,
                                error: false
                            });
                        } else if (event === 'error') {
                            streamMessages.value.push({
                                data: data,
                                timestamp: Date.now() - streamStartTime.value,
                                error: true
                            });
                        } else if (event === 'close') {
                            response.value = {
                                error: false,
                                data: '',
                                duration: Date.now() - streamStartTime.value
                            };
                        }
                    }
                }
            }

            isStreaming.value = false;
        };

        const executeClientStream = async (payload) => {
            // Initialize client stream
            const res = await fetch('/api/client-stream/init', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    method: selectedMethod.value.name,
                    headers: {},
                    timeout: timeout.value
                })
            });

            const result = await res.json();
            if (!result.success) {
                response.value = {
                    error: true,
                    data: result.error,
                    duration: 0
                };
                return;
            }

            streamId.value = result.streamId;
            isStreamActive.value = true;
            isStreaming.value = true;
            streamStartTime.value = Date.now();

            // Send first message
            await sendStreamMessage();
        };

        const executeBidiStream = async (payload) => {
            isStreaming.value = true;
            streamStartTime.value = Date.now();

            const url = '/api/bidi-stream?method=' + encodeURIComponent(selectedMethod.value.name) +
                        '&timeout=' + timeout.value;

            eventSource.value = new EventSource(url);

            eventSource.value.addEventListener('started', (e) => {
                streamId.value = e.data;
                isStreamActive.value = true;
                streamMessages.value.push({
                    data: 'Stream started with ID: ' + e.data,
                    timestamp: Date.now() - streamStartTime.value,
                    error: false,
                    system: true
                });
            });

            eventSource.value.addEventListener('message', (e) => {
                try {
                    const data = JSON.parse(e.data);
                    streamMessages.value.push({
                        data: JSON.stringify(data, null, 2),
                        timestamp: Date.now() - streamStartTime.value,
                        error: false
                    });
                } catch (err) {
                    streamMessages.value.push({
                        data: e.data,
                        timestamp: Date.now() - streamStartTime.value,
                        error: false
                    });
                }
            });

            eventSource.value.addEventListener('error', (e) => {
                const errorData = e.data || 'Stream error occurred';
                streamMessages.value.push({
                    data: errorData,
                    timestamp: Date.now() - streamStartTime.value,
                    error: true
                });
            });

            eventSource.value.addEventListener('close', (e) => {
                isStreamActive.value = false;
                isStreaming.value = false;
                response.value = {
                    error: false,
                    data: '',
                    duration: Date.now() - streamStartTime.value
                };
                if (eventSource.value) {
                    eventSource.value.close();
                    eventSource.value = null;
                }
            });

            eventSource.value.onerror = () => {
                isStreamActive.value = false;
                isStreaming.value = false;
                if (eventSource.value) {
                    eventSource.value.close();
                    eventSource.value = null;
                }
            };
        };

        const sendStreamMessage = async () => {
            if (!isStreamActive.value || !streamId.value) return;

            try {
                const payload = JSON.parse(requestBody.value);
                const endpoint = isBidiStreaming.value ? '/api/bidi-stream/send' : '/api/client-stream/send';

                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        streamId: streamId.value,
                        method: selectedMethod.value.name,
                        payload: payload
                    })
                });

                const result = await res.json();
                if (!result.success) {
                    streamMessages.value.push({
                        data: 'Failed to send: ' + result.error,
                        timestamp: Date.now() - streamStartTime.value,
                        error: true
                    });
                } else {
                    streamMessages.value.push({
                        data: 'Sent: ' + JSON.stringify(payload, null, 2),
                        timestamp: Date.now() - streamStartTime.value,
                        error: false,
                        sent: true
                    });
                }
            } catch (error) {
                streamMessages.value.push({
                    data: 'Error: ' + error.message,
                    timestamp: Date.now() - streamStartTime.value,
                    error: true
                });
            }
        };

        const closeStream = async () => {
            if (!isStreamActive.value || !streamId.value) return;

            try {
                const endpoint = isBidiStreaming.value ? '/api/bidi-stream/close' : '/api/client-stream/close';

                const res = await fetch(endpoint, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        streamId: streamId.value,
                        method: selectedMethod.value.name
                    })
                });

                const result = await res.json();

                if (result.success) {
                    if (!isBidiStreaming.value && result.response) {
                        // For client streaming, show final response
                        streamMessages.value.push({
                            data: JSON.stringify(result.response, null, 2),
                            timestamp: Date.now() - streamStartTime.value,
                            error: false,
                            final: true
                        });
                    }
                    response.value = {
                        error: false,
                        data: result.response ? JSON.stringify(result.response, null, 2) : 'Stream closed',
                        duration: Date.now() - streamStartTime.value
                    };
                } else {
                    response.value = {
                        error: true,
                        data: result.error,
                        duration: Date.now() - streamStartTime.value
                    };
                }
            } catch (error) {
                response.value = {
                    error: true,
                    data: error.message,
                    duration: Date.now() - streamStartTime.value
                };
            } finally {
                isStreamActive.value = false;
                isStreaming.value = false;
                streamId.value = null;
                if (eventSource.value) {
                    eventSource.value.close();
                    eventSource.value = null;
                }
            }
        };

        const executeMethod = async () => {
            if (!selectedMethod.value) return;

            isExecuting.value = true;
            response.value = null;
            streamMessages.value = [];

            try {
                const payload = JSON.parse(requestBody.value);

                if (selectedMethod.value.streamType === 'SERVER_STREAMING') {
                    await executeServerStream(payload);
                } else if (selectedMethod.value.streamType === 'UNARY') {
                    await executeUnary(payload);
                } else if (selectedMethod.value.streamType === 'CLIENT_STREAMING') {
                    await executeClientStream(payload);
                } else if (selectedMethod.value.streamType === 'BIDI_STREAMING') {
                    await executeBidiStream(payload);
                } else {
                    const msg = selectedMethod.value.streamType + " not yet supported";
                    throw new Error(msg);
                }
            } catch (error) {
                response.value = { error: true, data: error.message, duration: 0 };
            } finally {
                isExecuting.value = false;
            }
        };

        // Lifecycle
        onMounted(() => {
            loadMethods();
        });

        // Return everything that needs to be available in the template
        return {
            methods,
            selectedMethod,
            requestBody,
            timeout,
            response,
            isExecuting,
            isStreaming,
            streamMessages,
            streamStartTime,
            isStreamActive,
            streamId,
            eventSource,
            loadMethods,
            onMethodSelect,
            getStreamTypeClass,
            formatStreamType,
            executeMethod,
            sendStreamMessage,
            closeStream,
            isClientStreaming,
            isBidiStreaming,
            getExecuteButtonLabel
        };
    }
})
.use(PrimeVue.Config, {
    theme: {
        preset: PrimeVue.Themes.Aura
    }
})
.component('p-datatable', PrimeVue.DataTable)
.component('Column', PrimeVue.Column)
.mount('#app');
</script>
</body>
</html>`
