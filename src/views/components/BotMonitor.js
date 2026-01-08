export default {
    name: 'BotMonitor',
    data() {
        return {
            stats: null,
            loading: false,
            error: null,
            autoRefresh: true,
            refreshInterval: null,
            page: 1,
            pageSize: 15,
            expandedTraces: {},
            filterErrorsOnly: false,
            filterInstance: '',
            filterChat: '',
            filterProvider: '',
        }
    },
    computed: {
        filteredEvents() {
            if (!this.stats || !this.stats.recent_events) return [];

            const instance = String(this.filterInstance || '').trim().toLowerCase();
            const chat = String(this.filterChat || '').trim().toLowerCase();
            const provider = String(this.filterProvider || '').trim().toLowerCase();

            return this.stats.recent_events.filter((e) => {
                if (this.filterErrorsOnly && e.status !== 'error') return false;
                if (instance && String(e.instance_id || '').toLowerCase().indexOf(instance) === -1) return false;
                if (chat && String(e.chat_jid || '').toLowerCase().indexOf(chat) === -1) return false;
                if (provider && String(e.provider || '').toLowerCase().indexOf(provider) === -1) return false;
                return true;
            });
        },

        groupedTraces() {
            const ev = [...this.filteredEvents];
            ev.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));

            const map = new Map();
            for (const e of ev) {
                const trace = e.trace_id || `${e.instance_id}|${e.chat_jid}|${e.timestamp}`;
                let g = map.get(trace);
                if (!g) {
                    g = {
                        trace_id: trace,
                        instance_id: e.instance_id,
                        chat_jid: e.chat_jid,
                        provider: e.provider,
                        latest_ts: new Date(e.timestamp).getTime(),
                        has_error: false,
                        events: [],
                    };
                    map.set(trace, g);
                }
                const ts = new Date(e.timestamp).getTime();
                if (ts > g.latest_ts) {
                    g.latest_ts = ts;
                }
                if (e.status === 'error') {
                    g.has_error = true;
                }
                g.events.push(e);
            }

            const groups = Array.from(map.values());
            for (const g of groups) {
                g.events.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
            }

            groups.sort((a, b) => b.latest_ts - a.latest_ts);
            return groups;
        },

        totalPages() {
            return Math.max(1, Math.ceil(this.groupedTraces.length / this.pageSize));
        },

        pagedGroups() {
            const start = (this.page - 1) * this.pageSize;
            return this.groupedTraces.slice(start, start + this.pageSize);
        },
    },
    watch: {
        filterErrorsOnly() {
            this.page = 1;
        },
        filterInstance() {
            this.page = 1;
        },
        filterChat() {
            this.page = 1;
        },
        filterProvider() {
            this.page = 1;
        },
    },
    beforeUnmount() {
        this.stopAutoRefresh();
    },
    methods: {
        openModal() {
            $('#modalBotMonitor').modal({
                observeChanges: true,
                onVisible: () => {
                    setTimeout(() => {
                        const $m = $('#modalBotMonitor');
                        $m.find('.scrolling.content, .content').scrollTop(0);
                        $m.find('.ui.checkbox').checkbox();
                        $m.modal('refresh');
                    }, 0);
                },
                onHidden: () => {
                    this.stopAutoRefresh();
                },
                onApprove: function () {
                    return false;
                }
            }).modal('show');

            if (this.autoRefresh) {
                this.startAutoRefresh();
            } else {
                this.fetchStats();
            }
        },

        async fetchStats() {
            try {
                this.loading = true;
                this.error = null;

                if (!window.http) {
                    throw new Error('window.http is not initialized');
                }

                const { data } = await window.http.get('/api/bot-monitor/stats');
                this.stats = data;

                if (this.page > this.totalPages) {
                    this.page = this.totalPages;
                }

                const $m = $('#modalBotMonitor');
                $m.find('.ui.checkbox').checkbox();
                $m.modal('refresh');
            } catch (err) {
                this.error = `Error loading stats: ${err.message}`;
                console.error('BotMonitor stats error:', err);
            } finally {
                this.loading = false;
            }
        },

        toggleAutoRefresh() {
            this.autoRefresh = !this.autoRefresh;
            if (this.autoRefresh) {
                this.startAutoRefresh();
            } else {
                this.stopAutoRefresh();
            }
        },

        startAutoRefresh() {
            this.stopAutoRefresh();
            this.fetchStats();
            this.refreshInterval = setInterval(() => {
                this.fetchStats();
            }, 2000);
        },

        stopAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
        },

        formatTs(ts) {
            if (!ts) return '';
            // show local time
            const d = new Date(ts);
            return d.toLocaleString();
        },

        shortChat(chat) {
            if (!chat) return '';
            return chat.length > 32 ? chat.substring(0, 32) + '...' : chat;
        },

        shortInst(inst) {
            if (!inst) return '';
            return inst.length > 10 ? inst.substring(0, 10) + '...' : inst;
        },

        toggleTrace(traceId) {
            const k = String(traceId);
            this.expandedTraces[k] = !this.expandedTraces[k];
            setTimeout(() => {
                $('#modalBotMonitor').modal('refresh');
            }, 0);
        },

        isExpanded(traceId) {
            return !!this.expandedTraces[String(traceId)];
        },

        prevPage() {
            this.page = Math.max(1, this.page - 1);
            setTimeout(() => {
                const $m = $('#modalBotMonitor');
                $m.find('.scrolling.content, .content').scrollTop(0);
                $m.modal('refresh');
            }, 0);
        },

        nextPage() {
            this.page = Math.min(this.totalPages, this.page + 1);
            setTimeout(() => {
                const $m = $('#modalBotMonitor');
                $m.find('.scrolling.content, .content').scrollTop(0);
                $m.modal('refresh');
            }, 0);
        },
    },
    template: `
    <div class="violet card" @click="openModal()" style="cursor: pointer">
        <div class="content">
            <a class="ui violet right ribbon label">System</a>
            <div class="header">Bot Monitor</div>
            <div class="description">AI/bot traffic monitor (requests, replies, outbound sends, errors)</div>
        </div>
    </div>

    <div class="ui large modal" id="modalBotMonitor">
        <i class="close icon"></i>
        <div class="header">Bot Monitor</div>
        <div class="scrolling content">
            <div style="display:flex; align-items:center; justify-content:space-between; gap: 0.75rem; margin-bottom: 1rem;">
                <div class="ui small header" style="margin:0;">Auto refresh</div>
                <button @click="toggleAutoRefresh" :class="['ui button', { 'green': autoRefresh, 'grey': !autoRefresh }]">
                    <i :class="[autoRefresh ? 'pause' : 'play', 'icon']"></i>
                    {{ autoRefresh ? 'Pause' : 'Resume' }}
                </button>
            </div>

            <div v-if="loading && !stats" class="ui active loader"></div>
            <div v-else-if="error" class="ui negative message">
                <i class="times icon"></i>
                {{ error }}
            </div>
            <div v-else-if="stats">
                <div class="ui five stackable cards">
                    <div class="ui card">
                        <div class="content">
                            <div class="header">{{ stats.total_inbound }}</div>
                            <div class="meta">Inbound triggers</div>
                        </div>
                    </div>
                    <div class="ui card">
                        <div class="content">
                            <div class="header">{{ stats.total_ai_requests }}</div>
                            <div class="meta">AI requests</div>
                        </div>
                    </div>
                    <div class="ui card">
                        <div class="content">
                            <div class="header">{{ stats.total_ai_replies }}</div>
                            <div class="meta">AI replies</div>
                        </div>
                    </div>
                    <div class="ui card">
                        <div class="content">
                            <div class="header">{{ stats.total_outbound }}</div>
                            <div class="meta">Outbound sent</div>
                        </div>
                    </div>
                    <div class="ui card">
                        <div class="content">
                            <div class="header">{{ stats.total_errors }}</div>
                            <div class="meta">Errors</div>
                        </div>
                        <div class="extra content">
                            <i class="exclamation triangle icon red"></i>
                        </div>
                    </div>
                </div>

                <div class="ui segment" style="margin-top: 1.5em;">
                    <h4 class="ui header">
                        <i class="list icon"></i>
                        Traces (grouped by message)
                    </h4>

                    <div class="ui form" style="margin-bottom: 0.75rem;">
                        <div class="fields" style="margin-bottom: 0;">
                            <div class="four wide field">
                                <div class="ui checkbox">
                                    <input type="checkbox" v-model="filterErrorsOnly">
                                    <label>Errors only</label>
                                </div>
                            </div>
                            <div class="four wide field">
                                <input type="text" v-model="filterInstance" placeholder="Filter instance">
                            </div>
                            <div class="four wide field">
                                <input type="text" v-model="filterChat" placeholder="Filter chat">
                            </div>
                            <div class="four wide field">
                                <input type="text" v-model="filterProvider" placeholder="Filter provider">
                            </div>
                        </div>
                    </div>

                    <div style="display:flex; align-items:center; justify-content:space-between; gap:0.75rem; margin-bottom: 0.75rem;">
                        <div class="ui tiny header" style="margin:0;">Page {{ page }} / {{ totalPages }}</div>
                        <div>
                            <button class="ui tiny button" @click="prevPage" :disabled="page <= 1">Prev</button>
                            <button class="ui tiny button" @click="nextPage" :disabled="page >= totalPages">Next</button>
                        </div>
                    </div>

                    <div style="overflow:auto;">
                        <table class="ui very compact celled table">
                            <thead>
                                <tr>
                                    <th style="width: 160px;">Time</th>
                                    <th style="width: 120px;">Instance</th>
                                    <th>Chat</th>
                                    <th style="width: 90px;">Provider</th>
                                    <th style="width: 120px;">Events</th>
                                    <th style="width: 90px;">Status</th>
                                </tr>
                            </thead>
                            <tbody>
                                <template v-for="g in pagedGroups" :key="g.trace_id">
                                    <tr @click="toggleTrace(g.trace_id)" style="cursor:pointer;">
                                        <td>{{ formatTs(new Date(g.latest_ts).toISOString()) }}</td>
                                        <td><code>{{ shortInst(g.instance_id) }}</code></td>
                                        <td><code>{{ shortChat(g.chat_jid) }}</code></td>
                                        <td>{{ g.provider }}</td>
                                        <td>
                                            <span class="ui tiny label">{{ g.events.length }} events</span>
                                            <span v-if="isExpanded(g.trace_id)" class="ui tiny grey label">collapse</span>
                                            <span v-else class="ui tiny grey label">expand</span>
                                        </td>
                                        <td>
                                            <span v-if="g.has_error" class="ui tiny red label">error</span>
                                            <span v-else class="ui tiny green label">ok</span>
                                        </td>
                                    </tr>
                                    <tr v-if="isExpanded(g.trace_id)">
                                        <td colspan="6">
                                            <div style="overflow:auto;">
                                                <table class="ui very compact celled table">
                                                    <thead>
                                                        <tr>
                                                            <th style="width: 160px;">Time</th>
                                                            <th style="width: 110px;">Stage</th>
                                                            <th style="width: 90px;">Kind</th>
                                                            <th style="width: 90px;">Status</th>
                                                            <th style="width: 90px;">Duration</th>
                                                            <th>Error</th>
                                                        </tr>
                                                    </thead>
                                                    <tbody>
                                                        <tr v-for="(e, idx) in g.events" :key="g.trace_id + '-' + idx">
                                                            <td>{{ formatTs(e.timestamp) }}</td>
                                                            <td>
                                                                <span v-if="e.stage === 'mcp_call'" class="ui tiny teal label">
                                                                    <i class="wrench icon"></i> {{ e.stage }}
                                                                </span>
                                                                <span v-else>{{ e.stage }}</span>
                                                            </td>
                                                            <td>
                                                                <span v-if="e.stage === 'mcp_call'">
                                                                    <i class="wrench icon teal"></i> <strong>{{ e.kind }}</strong>
                                                                </span>
                                                                <span v-else>{{ e.kind }}</span>
                                                            </td>
                                                            <td>
                                                                <span v-if="e.status === 'ok'" class="ui tiny green label">ok</span>
                                                                <span v-else-if="e.status === 'error'" class="ui tiny red label">error</span>
                                                                <span v-else class="ui tiny grey label">{{ e.status }}</span>
                                                            </td>
                                                            <td>
                                                                <span v-if="e.duration_ms">{{ e.duration_ms }} ms</span>
                                                            </td>
                                                            <td style="max-width: 520px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                                                                <span v-if="e.error">{{ e.error }}</span>
                                                            </td>
                                                        </tr>
                                                    </tbody>
                                                </table>
                                            </div>
                                        </td>
                                    </tr>
                                </template>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </div>
    </div>
    `
}
