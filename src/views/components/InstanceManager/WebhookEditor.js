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
    },
    data() {
        return {
            webhookUrlsInput: '',
            webhookSecretInput: '',
            webhookInsecureInput: false,
            savingWebhook: false,
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
                this.savingWebhook = true;
                await window.http.put(`/instances/${this.instanceId}/webhook`, {
                    urls,
                    secret: this.webhookSecretInput || '',
                    insecure: !!this.webhookInsecureInput,
                });
                this.$emit('refresh-instances');
                showSuccessInfo('Webhook configuration updated for instance.');
                this.cancel();
            } catch (err) {
                handleApiError(err, 'Failed to update webhook configuration');
            } finally {
                this.savingWebhook = false;
            }
        },
    },
    template: `
        <div v-if="instanceId" class="ui segment" style="margin-top: 1em;">
            <h4 class="ui header">Webhook configuration</h4>
            <div class="ui form">
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
                <div class="ui buttons">
                    <button type="button" class="ui primary button" :class="{ loading: savingWebhook }" @click="save" :disabled="savingWebhook">
                        Save
                    </button>
                    <div class="or"></div>
                    <button type="button" class="ui button" @click="cancel" :disabled="savingWebhook">
                        Cancel
                    </button>
                </div>
            </div>
        </div>
    `,
};
