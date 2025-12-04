export default {
    name: 'InstanceManager',
    props: {
        instances: {
            type: Array,
            default: () => [],
        },
        selectedToken: {
            type: String,
            default: '',
        },
    },
    data() {
        return {
            newName: '',
            creating: false,
            lastToken: null,
            tokenInput: '',

            editingInstanceId: null,
            webhookUrlsInput: '',
            webhookSecretInput: '',
            webhookInsecureInput: false,
            savingWebhook: false,

            chatwootEditingInstanceId: null,
            chatwootBaseUrlInput: '',
            chatwootAccountTokenInput: '',
            chatwootBotTokenInput: '',
            chatwootAccountIdInput: '',
            chatwootInboxIdInput: '',
            chatwootInboxIdentifierInput: '',
            savingChatwoot: false,
        };
    },
    watch: {
        selectedToken: {
            immediate: true,
            handler(val) {
                this.tokenInput = val || '';
            },
        },
    },
    methods: {
        async createInstance() {
            if (!this.newName || this.creating) return;
            try {
                this.creating = true;
                const { data } = await window.http.post('/instances', {
                    name: this.newName,
                });
                const results = data.results || {};
                this.lastToken = results.token || null;
                this.newName = '';
                this.$emit('refresh-instances');
                if (this.lastToken) {
                    this.tokenInput = this.lastToken;
                    this.$emit('set-active-token', this.lastToken);
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Instance created. Token set as active for this UI session.');
                    }
                }
            } catch (err) {
                if (err.response && err.response.data && err.response.data.message) {
                    if (typeof window.showErrorInfo === 'function') {
                        window.showErrorInfo(err.response.data.message);
                    }
                } else if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err.message || 'Failed to create instance');
                }
            } finally {
                this.creating = false;
            }
        },
        applyToken() {
            const token = (this.tokenInput || '').trim();
            if (!token) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo('You must select an instance token; global session is disabled.');
                }
                return;
            }
            this.$emit('set-active-token', token);
            if (typeof window.showSuccessInfo === 'function') {
                window.showSuccessInfo('Active instance token set for this UI session.');
            }
        },
        useInstance(inst) {
            if (!inst || !inst.token) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo('Selected instance has no token available.');
                }
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
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Instance deleted.');
                }
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to delete instance');
                }
            }
        },
        openWebhookEditor(inst) {
            if (!inst || !inst.id) return;
            this.editingInstanceId = inst.id;
            const urls = Array.isArray(inst.webhook_urls) ? inst.webhook_urls : [];
            this.webhookUrlsInput = urls.join('\n');
            this.webhookSecretInput = inst.webhook_secret || '';
            this.webhookInsecureInput = !!inst.webhook_insecure_skip_verify;
        },
        cancelWebhookEditor() {
            this.editingInstanceId = null;
            this.webhookUrlsInput = '';
            this.webhookSecretInput = '';
            this.webhookInsecureInput = false;
        },
        async saveWebhookConfig() {
            if (!this.editingInstanceId) return;
            const raw = this.webhookUrlsInput || '';
            const urls = raw
                .split('\n')
                .map((u) => u.trim())
                .filter((u) => u.length > 0);

            try {
                this.savingWebhook = true;
                await window.http.put(`/instances/${this.editingInstanceId}/webhook`, {
                    urls,
                    secret: this.webhookSecretInput || '',
                    insecure: !!this.webhookInsecureInput,
                });
                this.$emit('refresh-instances');
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Webhook configuration updated for instance.');
                }
                this.cancelWebhookEditor();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to update webhook configuration');
                }
            } finally {
                this.savingWebhook = false;
            }
        },
        openChatwootEditor(inst) {
            if (!inst || !inst.id) return;
            this.chatwootEditingInstanceId = inst.id;
            this.chatwootBaseUrlInput = inst.chatwoot_base_url || '';
            this.chatwootAccountTokenInput = inst.chatwoot_account_token || '';
            this.chatwootBotTokenInput = inst.chatwoot_bot_token || '';
            this.chatwootAccountIdInput = inst.chatwoot_account_id || '';
            this.chatwootInboxIdInput = inst.chatwoot_inbox_id || '';
            this.chatwootInboxIdentifierInput = inst.chatwoot_inbox_identifier || '';
        },
        cancelChatwootEditor() {
            this.chatwootEditingInstanceId = null;
            this.chatwootBaseUrlInput = '';
            this.chatwootAccountTokenInput = '';
            this.chatwootBotTokenInput = '';
            this.chatwootAccountIdInput = '';
            this.chatwootInboxIdInput = '';
            this.chatwootInboxIdentifierInput = '';
        },
        async saveChatwootConfig() {
            if (!this.chatwootEditingInstanceId) return;
            try {
                this.savingChatwoot = true;
                await window.http.put(`/instances/${this.chatwootEditingInstanceId}/chatwoot`, {
                    base_url: this.chatwootBaseUrlInput || '',
                    account_id: this.chatwootAccountIdInput || '',
                    inbox_id: this.chatwootInboxIdInput || '',
                    inbox_identifier: this.chatwootInboxIdentifierInput || '',
                    account_token: this.chatwootAccountTokenInput || '',
                    bot_token: this.chatwootBotTokenInput || '',
                });
                this.$emit('refresh-instances');
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Chatwoot configuration updated for instance.');
                }
                this.cancelChatwootEditor();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to update Chatwoot configuration');
                }
            } finally {
                this.savingChatwoot = false;
            }
        },
        chatwootWebhookUrl() {
            const id = this.chatwootEditingInstanceId;
            if (!id) return '';
            const base = window.location ? `${window.location.protocol}//${window.location.host}` : '';
            const basePath = window.AppBasePath || '';
            const normalizedBasePath = basePath && basePath !== '/' ? basePath.replace(/\/$/, '') : '';
            return `${base}${normalizedBasePath}/instances/${id}/chatwoot/webhook`;
        },
        copyChatwootWebhookUrl() {
            const url = this.chatwootWebhookUrl();
            if (!url) return;
            if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(url).then(() => {
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Chatwoot webhook URL copied to clipboard.');
                    }
                }).catch((err) => {
                    if (typeof window.showErrorInfo === 'function') {
                        window.showErrorInfo(err?.message || 'Failed to copy Chatwoot webhook URL');
                    }
                });
            }
        },
    },
    template: `
    <div class="green card" style="cursor: default">
        <div class="content">
            <a class="ui teal right ribbon label">Instances</a>
            <div class="header">Instance Manager</div>
            <div class="description">
                <div class="ui form">
                    <div class="fields">
                        <div class="ten wide field">
                            <label>New instance name</label>
                            <input type="text" v-model="newName" placeholder="e.g. my-bot-ventas" :disabled="creating"
                                   @keyup.enter="createInstance">
                        </div>
                        <div class="six wide field" style="padding-top: 23px;">
                            <button class="ui primary button" type="button" @click="createInstance" :disabled="creating || !newName">
                                <i class="plus icon"></i>
                                Create instance
                            </button>
                        </div>
                    </div>
                    <div class="field" v-if="lastToken">
                        <label>Last created instance token (copy and keep it safe)</label>
                        <div class="ui fluid action input">
                            <input type="text" :value="lastToken" readonly>
                            <button type="button" class="ui button" @click="() => navigator.clipboard && navigator.clipboard.writeText(lastToken)">
                                Copy
                            </button>
                        </div>
                    </div>
                    <div class="field">
                        <label>Active token for this UI (used in X-Instance-Token header)</label>
                        <div class="ui fluid action input">
                            <input type="text" v-model="tokenInput" placeholder="Paste or type instance token">
                            <button type="button" class="ui button" @click="applyToken">
                                Use token
                            </button>
                        </div>
                        <small v-if="!selectedToken">No active instance selected. Some actions will fail until you choose an instance.</small>
                        <small v-else>Token is active for all API calls from this UI.</small>
                    </div>
                </div>

                <div class="ui divider"></div>

                <div v-if="instances && instances.length">
                    <h4 class="ui header">Existing instances</h4>
                    <table class="ui very basic compact table">
                        <thead>
                        <tr>
                            <th>ID</th>
                            <th>Name</th>
                            <th>Status</th>
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
                            </td>
                        </tr>
                        </tbody>
                    </table>
                    <div v-if="editingInstanceId" class="ui segment" style="margin-top: 1em;">
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
                                <button type="button" class="ui primary button" :class="{ loading: savingWebhook }" @click="saveWebhookConfig" :disabled="savingWebhook">
                                    Save
                                </button>
                                <div class="or"></div>
                                <button type="button" class="ui button" @click="cancelWebhookEditor" :disabled="savingWebhook">
                                    Cancel
                                </button>
                            </div>
                        </div>
                    </div>
                    <div v-if="chatwootEditingInstanceId" class="ui segment" style="margin-top: 1em;">
                        <h4 class="ui header">Chatwoot configuration</h4>
                        <div class="ui form">
                            <div class="field">
                                <label>Chatwoot webhook URL for this instance</label>
                                <div class="ui fluid action input">
                                    <input type="text" :value="chatwootWebhookUrl()" readonly>
                                    <button type="button" class="ui button" @click="copyChatwootWebhookUrl">
                                        Copy
                                    </button>
                                </div>
                            </div>
                            <div class="field">
                                <label>Chatwoot API base URL</label>
                                <input type="text" v-model="chatwootBaseUrlInput" placeholder="https://chatwoot.example.com">
                            </div>
                            <div class="field">
                                <label>Account access token</label>
                                <input type="text" v-model="chatwootAccountTokenInput" placeholder="Account API token">
                            </div>
                            <div class="field">
                                <label>Bot token</label>
                                <input type="text" v-model="chatwootBotTokenInput" placeholder="Bot token for this instance">
                            </div>
                            <div class="field">
                                <label>Account ID</label>
                                <input type="text" v-model="chatwootAccountIdInput" placeholder="Chatwoot account ID">
                            </div>
                            <div class="field">
                                <label>Inbox ID</label>
                                <input type="text" v-model="chatwootInboxIdInput" placeholder="Chatwoot inbox ID for this instance">
                            </div>
                            <div class="field">
                                <label>Inbox Identifier (API channel identifier)</label>
                                <input type="text" v-model="chatwootInboxIdentifierInput" placeholder="chatwoot.inboxIdentifier from your API channel">
                            </div>
                            <div class="ui buttons">
                                <button type="button" class="ui primary button" :class="{ loading: savingChatwoot }" @click="saveChatwootConfig" :disabled="savingChatwoot">
                                    Save
                                </button>
                                <div class="or"></div>
                                <button type="button" class="ui button" @click="cancelChatwootEditor" :disabled="savingChatwoot">
                                    Cancel
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
                <div v-else>
                    <p>No instances yet. Create one to get a token.</p>
                </div>
            </div>
        </div>
    </div>
    `,
};
