import { showSuccessInfo, handleApiError } from './utils.js';

export default {
    name: 'WebhookEditor',
    props: {
        instanceId: {
            type: [String, Number],
            default: null,
        },
        webhookUrls: {
            type: Array,
            default: () => [],
        },
        webhookSecret: {
            type: String,
            default: '',
        },
        webhookInsecure: {
            type: Boolean,
            default: false,
        },
        autoReconnect: {
            type: Boolean,
            default: true,
        },
    },
    data() {
        return {
            webhookUrlsInput: '',
            webhookSecretInput: '',
            webhookInsecureInput: false,
            autoReconnectInput: true,
            savingSettings: false,
        };
    },
    watch: {
        instanceId: {
            immediate: true,
            handler(val) {
                if (val) {
                    this.loadData();
                }
            },
        },
    },
    methods: {
        loadData() {
            const urls = Array.isArray(this.webhookUrls) ? this.webhookUrls : [];
            this.webhookUrlsInput = urls.join('\n');
            this.webhookSecretInput = this.webhookSecret || '';
            this.webhookInsecureInput = !!this.webhookInsecure;
            this.autoReconnectInput = this.autoReconnect !== false;
        },
        cancel() {
            this.$emit('cancel');
        },
        async save() {
            if (!this.instanceId) return;
            const raw = this.webhookUrlsInput || '';
            const urls = raw
                .split('\n')
                .map((u) => u.trim())
                .filter((u) => u.length > 0);

            try {
                this.savingSettings = true;
                
                // Save Webhooks
                await window.http.put(`/instances/${this.instanceId}/webhook`, {
                    urls,
                    secret: this.webhookSecretInput || '',
                    insecure: !!this.webhookInsecureInput,
                });

                // Save AutoReconnect
                await window.http.put(`/instances/${this.instanceId}/auto-reconnect`, {
                    enabled: !!this.autoReconnectInput,
                });

                this.$emit('refresh-instances');
                showSuccessInfo('Instance settings updated.');
                this.cancel();
            } catch (err) {
                handleApiError(err, 'Failed to update instance settings');
            } finally {
                this.savingSettings = false;
            }
        },
    },
    template: `
        <div v-if="instanceId" class="ui segment" style="margin-top: 1em;">
            <h4 class="ui header"><i class="cog icon"></i> Instance Settings</h4>
            
            <div class="ui form">
                <div class="ui top attached tabular menu">
                    <a class="active item">General & Webhooks</a>
                </div>
                <div class="ui bottom attached active tab segment">
                    <div class="field">
                        <div class="ui toggle checkbox">
                            <input type="checkbox" v-model="autoReconnectInput" id="auto-reconnect-toggle">
                            <label for="auto-reconnect-toggle">
                                <strong>Auto-reconnect</strong> 
                                <small>(Maintain session alive automatically)</small>
                            </label>
                        </div>
                    </div>

                    <div class="ui divider"></div>

                    <div class="field">
                        <label>Webhook URLs (one per line)</label>
                        <textarea rows="3" v-model="webhookUrlsInput" placeholder="https://example.com/webhook"></textarea>
                    </div>
                    <div class="field">
                        <label>Webhook secret</label>
                        <input type="text" v-model="webhookSecretInput" placeholder="Optional shared secret for signature">
                    </div>
                    <div class="field">
                        <div class="ui checkbox">
                            <input type="checkbox" v-model="webhookInsecureInput" id="webhook-insecure-toggle">
                            <label for="webhook-insecure-toggle">Skip TLS verification (development / self-signed)</label>
                        </div>
                    </div>
                </div>

                <div style="margin-top: 1em;">
                    <div class="ui buttons">
                        <button type="button" class="ui primary button" :class="{ loading: savingSettings }" @click="save" :disabled="savingSettings">
                            Save Settings
                        </button>
                        <div class="or"></div>
                        <button type="button" class="ui button" @click="cancel" :disabled="savingSettings">
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `,
};
