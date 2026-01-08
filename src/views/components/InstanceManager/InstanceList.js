import { showSuccessInfo, showErrorInfo, handleApiError } from './utils.js';

export default {
    name: 'InstanceList',
    props: {
        instances: {
            type: Array,
            default: () => [],
        },
        selectedToken: {
            type: String,
            default: '',
        },
        healthStatus: {
            type: Object,
            default: () => ({}),
        },
    },
    data() {
        return {
            tokenInput: '',
            cacheSizes: {},
        };
    },
    watch: {
        selectedToken: {
            immediate: true,
            handler(val) {
                this.tokenInput = val || '';
            },
        },
        instances: {
            immediate: true,
            handler(newInstances) {
                if (newInstances && newInstances.length > 0) {
                    newInstances.forEach(inst => {
                        this.fetchCacheSize(inst.id);
                    });
                }
            }
        }
    },
    methods: {
        async fetchCacheSize(id) {
            try {
                const { data } = await window.http.get(`/api/instances/${id}/cache/stats`);
                if (data.results) {
                    this.cacheSizes[id] = data.results.human_size;
                }
            } catch (err) {
                console.error('Failed to fetch cache size for instance ' + id, err);
            }
        },
        async clearInstanceCache(inst) {
            if (!confirm(`Clear cache for instance "${inst.name}"? This will delete its downloaded media and history logs.`)) {
                return;
            }
            try {
                await window.http.post(`/api/instances/${inst.id}/cache/clear`);
                showSuccessInfo(`Cache cleared for ${inst.name}`);
                this.fetchCacheSize(inst.id);
            } catch (err) {
                handleApiError(err, 'Failed to clear instance cache');
            }
        },
        applyToken() {
            const token = (this.tokenInput || '').trim();
            if (!token) {
                showErrorInfo('You must select an instance token; global session is disabled.');
                return;
            }
            this.$emit('set-active-token', token);
            showSuccessInfo('Active instance token set for this UI session.');
        },
        useInstance(inst) {
            if (!inst || !inst.token) {
                showErrorInfo('Selected instance has no token available.');
                return;
            }
            this.tokenInput = inst.token;
            this.applyToken();
        },
        async deleteInstance(inst) {
            if (!inst || !inst.id) {
                return;
            }
            if (!window.confirm(`Delete instance "${inst.name}"? This will remove it from the UI.`)) {
                return;
            }
            try {
                await window.http.delete(`/instances/${inst.id}`);
                if (inst.token && inst.token === this.selectedToken) {
                    this.tokenInput = '';
                    this.$emit('set-active-token', '');
                }
                this.$emit('refresh-instances');
                showSuccessInfo('Instance deleted.');
            } catch (err) {
                handleApiError(err, 'Failed to delete instance');
            }
        },
        openWebhookEditor(inst) {
            this.$emit('open-webhook-editor', inst);
        },
        openChatwootEditor(inst) {
            this.$emit('open-chatwoot-editor', inst);
        },
        openGeminiEditor(inst) {
            this.$emit('open-gemini-editor', inst);
        },
    },
    template: `
        <div>
            <div class="ui divider"></div>
            <div v-if="instances && instances.length">
                <h4 class="ui header">Existing instances</h4>
                <table class="ui very basic compact table">
                    <thead>
                    <tr>
                        <th>ID</th>
                        <th>Name</th>
                        <th>Status</th>
                        <th>Cache</th>
                        <th>Actions</th>
                    </tr>
                    </thead>
                    <tbody>
                    <tr v-for="inst in instances" :key="inst.id" :class="{ active: inst.token && inst.token === selectedToken }">
                        <td>{{ inst.id }}</td>
                        <td>{{ inst.name }}</td>
                        <td>
                            <div
                                class="ui tiny label"
                                :class="{
                                    green: inst.status === 'ONLINE',
                                    red: inst.status === 'OFFLINE',
                                    grey: inst.status !== 'ONLINE' && inst.status !== 'OFFLINE'
                                }"
                            >
                                {{ inst.status === 'ONLINE' ? 'Online' : inst.status === 'OFFLINE' ? 'Offline' : inst.status }}
                            </div>
                        </td>
                        <td>
                            <div class="ui transparent input" style="font-size: 0.9em;">
                                <span>{{ cacheSizes[inst.id] || '0 B' }}</span>
                                <i v-if="cacheSizes[inst.id] && cacheSizes[inst.id] !== '0 B' && cacheSizes[inst.id] !== '...'" 
                                   class="trash alternate outline red icon" 
                                   style="cursor: pointer; margin-left: 8px;"
                                   title="Clear Instance Cache"
                                   @click="clearInstanceCache(inst)"></i>
                            </div>
                        </td>
                        <td>
                            <button type="button" class="ui mini button" @click="useInstance(inst)">
                                Use in UI
                            </button>
                            <button type="button" class="ui mini red basic button" @click="deleteInstance(inst)" style="margin-left: 0.5em;">
                                Delete
                            </button>
                            <button type="button" class="ui mini basic button" @click="openWebhookEditor(inst)" style="margin-left: 0.5em;">
                                Webhooks
                            </button>
                            <button type="button" class="ui mini basic button" @click="openChatwootEditor(inst)" style="margin-left: 0.5em;">
                                Chatwoot
                            </button>
                            <button type="button" class="ui mini basic button" @click="openGeminiEditor(inst)" style="margin-left: 0.5em;">
                                IA Assistant
                            </button>
                        </td>
                    </tr>
                    </tbody>
                </table>
            </div>
            <div v-else>
                <p>No instances yet. Create one to get a token.</p>
            </div>
        </div>
    `,
};
