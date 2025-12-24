export default {
    name: 'WorkerPoolMonitor',
    data() {
        return {
            stats: null,
            webhookStats: null,
            loading: false,
            error: null,
            webhookError: null,
            autoRefresh: true,
            refreshInterval: null,
            modalOpen: false,
            prevWorkerProcessing: {},
            workerFlash: {},
            prevWebhookWorkerProcessing: {},
            webhookWorkerFlash: {},
        }
    },
    computed: {
        totalQueueDepth() {
            if (!this.stats || !this.stats.worker_stats) return 0;
            return this.stats.worker_stats.reduce((sum, w) => sum + w.queue_depth, 0);
        },
        activeChatsCount() {
            if (!this.stats || !this.stats.active_chats) return 0;
            return Object.keys(this.stats.active_chats).length;
        },
        webhookTotalQueueDepth() {
            if (!this.webhookStats || !this.webhookStats.worker_stats) return 0;
            return this.webhookStats.worker_stats.reduce((sum, w) => sum + w.queue_depth, 0);
        },
        webhookActiveChatsCount() {
            if (!this.webhookStats || !this.webhookStats.active_chats) return 0;
            return Object.keys(this.webhookStats.active_chats).length;
        },
    },
    mounted() {
        // No auto-fetch en mount, solo cuando se abre el modal
    },
    beforeUnmount() {
        this.stopAutoRefresh();
    },
    methods: {
        openModal() {
            this.modalOpen = true;

            $('#modalWorkerPoolMonitor').modal({
                observeChanges: true,
                onVisible: () => {
                    setTimeout(() => {
                        const $m = $('#modalWorkerPoolMonitor');
                        $m.find('.scrolling.content, .content').scrollTop(0);
                        $m.modal('refresh');
                    }, 0);
                },
                onHidden: () => {
                    this.stopAutoRefresh();
                    this.modalOpen = false;
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
                this.webhookError = null;

                // Importante: usar window.http (axios) para heredar Authorization
                // y evitar que el navegador dispare el prompt de BasicAuth repetidamente.
                if (!window.http) {
                    throw new Error('window.http is not initialized');
                }

                const [mainRes, webhookRes] = await Promise.allSettled([
                    window.http.get('/api/worker-pool/stats'),
                    window.http.get('/api/bot-webhook-pool/stats'),
                ]);

                if (mainRes.status === 'fulfilled') {
                    this.stats = mainRes.value.data;
                } else {
                    throw mainRes.reason;
                }

                if (webhookRes.status === 'fulfilled') {
                    this.webhookStats = webhookRes.value.data;
                } else {
                    const status = webhookRes.reason?.response?.status;
                    if (status === 503) {
                        this.webhookStats = null;
                        this.webhookError = 'Bot webhook pool not initialized yet';
                    } else {
                        this.webhookStats = null;
                        this.webhookError = `Error loading bot webhook pool stats: ${webhookRes.reason?.message || webhookRes.reason}`;
                    }
                }

                if (this.stats && this.stats.worker_stats) {
                    for (const w of this.stats.worker_stats) {
                        const id = String(w.worker_id);
                        const wasProcessing = !!this.prevWorkerProcessing[id];
                        const isProcessing = !!w.is_processing;
                        if (isProcessing && !wasProcessing) {
                            this.workerFlash[id] = true;
                            setTimeout(() => {
                                this.workerFlash[id] = false;
                            }, 900);
                        }
                        this.prevWorkerProcessing[id] = isProcessing;
                    }
                }

                if (this.webhookStats && this.webhookStats.worker_stats) {
                    for (const w of this.webhookStats.worker_stats) {
                        const id = String(w.worker_id);
                        const wasProcessing = !!this.prevWebhookWorkerProcessing[id];
                        const isProcessing = !!w.is_processing;
                        if (isProcessing && !wasProcessing) {
                            this.webhookWorkerFlash[id] = true;
                            setTimeout(() => {
                                this.webhookWorkerFlash[id] = false;
                            }, 900);
                        }
                        this.prevWebhookWorkerProcessing[id] = isProcessing;
                    }
                }

                if (this.modalOpen) {
                    $('#modalWorkerPoolMonitor').modal('refresh');
                }
            } catch (err) {
                this.error = `Error loading stats: ${err.message}`;
                console.error('WorkerPool stats error:', err);
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
        
        formatChatKey(key) {
            const parts = key.split('|');
            if (parts.length === 2) {
                const instanceShort = parts[0].substring(0, 8) + '...';
                const chatShort = parts[1].length > 20 ? parts[1].substring(0, 20) + '...' : parts[1];
                return `${instanceShort} | ${chatShort}`;
            }
            return key;
        },
        formatWebhookChatKey(key) {
            const parts = key.split('|');
            if (parts.length === 2) {
                const instanceShort = parts[0].length > 20 ? parts[0].substring(0, 20) + '...' : parts[0];
                const chatShort = parts[1].length > 20 ? parts[1].substring(0, 20) + '...' : parts[1];
                return `${instanceShort} | ${chatShort}`;
            }
            return key;
        },
    },
    template: `
        <div class="blue card" @click="openModal()" style="cursor: pointer">
            <div class="content">
                <a class="ui blue right ribbon label">System</a>
                <div class="header">Worker Pool Monitor</div>
                <div class="description">
                    Real-time worker pool metrics (queues, throughput, active chats)
                </div>
            </div>
        </div>

        <div class="ui large modal" id="modalWorkerPoolMonitor">
            <i class="close icon"></i>
            <div class="header">Worker Pool Monitor</div>
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

                <div v-else-if="stats" class="stats-container">
                    <div class="ui four stackable cards">
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ stats.active_workers }} / {{ stats.num_workers }}</div>
                                <div class="meta">Active workers</div>
                            </div>
                            <div class="extra content"><i class="server icon blue"></i></div>
                        </div>
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ stats.total_processed }}</div>
                                <div class="meta">Processed</div>
                            </div>
                            <div class="extra content"><i class="check circle icon green"></i></div>
                        </div>
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ totalQueueDepth }}</div>
                                <div class="meta">Total queued</div>
                            </div>
                            <div class="extra content"><i class="hourglass half icon orange"></i></div>
                        </div>
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ activeChatsCount }}</div>
                                <div class="meta">Active chats</div>
                            </div>
                            <div class="extra content"><i class="comments icon purple"></i></div>
                        </div>
                    </div>

                    <div class="ui three stackable cards" style="margin-top: 1em;">
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ stats.total_dispatched }}</div>
                                <div class="meta">Dispatched</div>
                            </div>
                        </div>
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ stats.total_errors }}</div>
                                <div class="meta">Errors</div>
                            </div>
                            <div class="extra content"><i class="exclamation triangle icon red"></i></div>
                        </div>
                        <div class="ui card">
                            <div class="content">
                                <div class="header">{{ stats.total_dropped }}</div>
                                <div class="meta">Dropped</div>
                            </div>
                            <div class="extra content"><i class="trash icon red"></i></div>
                        </div>
                    </div>

                    <div class="ui segment" style="margin-top: 2em;">
                        <h4 class="ui header"><i class="cog icon"></i>Workers</h4>
                        <div class="ui three stackable cards">
                            <div v-for="worker in stats.worker_stats"
                                 :key="worker.worker_id"
                                 :class="['ui card', { 'worker-busy': worker.is_processing, 'worker-flash': workerFlash[String(worker.worker_id)] }]">
                                <div class="content">
                                    <div class="header">
                                        Worker #{{ worker.worker_id }}
                                        <span v-if="worker.is_processing" class="ui tiny green label"><i class="fire icon"></i> Busy</span>
                                        <span v-else class="ui tiny grey label"><i class="moon icon"></i> Idle</span>
                                    </div>
                                    <div class="meta" style="margin-top: 0.5em;">
                                        Queue: <strong>{{ worker.queue_depth }}</strong> / {{ stats.queue_size }}
                                    </div>
                                    <div class="description">
                                        Processed: <strong>{{ worker.jobs_processed }}</strong>
                                    </div>
                                </div>
                                <div class="extra content">
                                    <div class="ui small blue progress"
                                         :data-percent="Math.round((worker.queue_depth / stats.queue_size) * 100)">
                                        <div class="bar"
                                             :style="{ width: (worker.queue_depth / stats.queue_size * 100) + '%' }"></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div v-if="activeChatsCount > 0" class="ui segment" style="margin-top: 1em;">
                        <h4 class="ui header"><i class="comments icon"></i>Active chats ({{ activeChatsCount }})</h4>
                        <div class="ui relaxed divided list">
                            <div v-for="(workerId, chatKey) in stats.active_chats"
                                 :key="chatKey"
                                 class="item">
                                <i class="comment icon"></i>
                                <div class="content">
                                    <div class="description">
                                        <code>{{ formatChatKey(chatKey) }}</code>
                                        <i class="arrow right icon"></i>
                                        <span class="ui tiny blue label">Worker #{{ workerId }}</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="ui segment" style="margin-top: 2em;">
                        <h4 class="ui header"><i class="robot icon"></i>Bot Webhook Pool</h4>

                        <div v-if="webhookError && !webhookStats" class="ui warning message">
                            <i class="info circle icon"></i>
                            {{ webhookError }}
                        </div>

                        <div v-else-if="webhookStats">
                            <div class="ui four stackable cards">
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookStats.active_workers }} / {{ webhookStats.num_workers }}</div>
                                        <div class="meta">Active workers</div>
                                    </div>
                                    <div class="extra content"><i class="server icon blue"></i></div>
                                </div>
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookStats.total_processed }}</div>
                                        <div class="meta">Processed</div>
                                    </div>
                                    <div class="extra content"><i class="check circle icon green"></i></div>
                                </div>
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookTotalQueueDepth }}</div>
                                        <div class="meta">Total queued</div>
                                    </div>
                                    <div class="extra content"><i class="hourglass half icon orange"></i></div>
                                </div>
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookActiveChatsCount }}</div>
                                        <div class="meta">Active chats</div>
                                    </div>
                                    <div class="extra content"><i class="comments icon purple"></i></div>
                                </div>
                            </div>

                            <div class="ui three stackable cards" style="margin-top: 1em;">
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookStats.total_dispatched }}</div>
                                        <div class="meta">Dispatched</div>
                                    </div>
                                </div>
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookStats.total_errors }}</div>
                                        <div class="meta">Errors</div>
                                    </div>
                                    <div class="extra content"><i class="exclamation triangle icon red"></i></div>
                                </div>
                                <div class="ui card">
                                    <div class="content">
                                        <div class="header">{{ webhookStats.total_dropped }}</div>
                                        <div class="meta">Dropped</div>
                                    </div>
                                    <div class="extra content"><i class="trash icon red"></i></div>
                                </div>
                            </div>

                            <div class="ui segment" style="margin-top: 2em;">
                                <h4 class="ui header"><i class="cog icon"></i>Workers</h4>
                                <div class="ui three stackable cards">
                                    <div v-for="worker in webhookStats.worker_stats"
                                         :key="'webhook-' + worker.worker_id"
                                         :class="['ui card', { 'worker-busy': worker.is_processing, 'worker-flash': webhookWorkerFlash[String(worker.worker_id)] }]">
                                        <div class="content">
                                            <div class="header">
                                                Worker #{{ worker.worker_id }}
                                                <span v-if="worker.is_processing" class="ui tiny green label"><i class="fire icon"></i> Busy</span>
                                                <span v-else class="ui tiny grey label"><i class="moon icon"></i> Idle</span>
                                            </div>
                                            <div class="meta" style="margin-top: 0.5em;">
                                                Queue: <strong>{{ worker.queue_depth }}</strong> / {{ webhookStats.queue_size }}
                                            </div>
                                            <div class="description">
                                                Processed: <strong>{{ worker.jobs_processed }}</strong>
                                            </div>
                                        </div>
                                        <div class="extra content">
                                            <div class="ui small blue progress"
                                                 :data-percent="Math.round((worker.queue_depth / webhookStats.queue_size) * 100)">
                                                <div class="bar"
                                                     :style="{ width: (worker.queue_depth / webhookStats.queue_size * 100) + '%' }"></div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div v-if="webhookActiveChatsCount > 0" class="ui segment" style="margin-top: 1em;">
                                <h4 class="ui header"><i class="comments icon"></i>Active chats ({{ webhookActiveChatsCount }})</h4>
                                <div class="ui relaxed divided list">
                                    <div v-for="(workerId, chatKey) in webhookStats.active_chats"
                                         :key="'webhook-chat-' + chatKey"
                                         class="item">
                                        <i class="comment icon"></i>
                                        <div class="content">
                                            <div class="description">
                                                <code>{{ formatWebhookChatKey(chatKey) }}</code>
                                                <i class="arrow right icon"></i>
                                                <span class="ui tiny blue label">Worker #{{ workerId }}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
}
