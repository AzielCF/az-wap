import { showSuccessInfo, handleApiError, chatwootWebhookUrl, copyToClipboard } from './utils.js';

export default {
    name: 'ChatwootEditor',
    props: {
        instanceId: {
            type: [String, Number],
            default: null,
        },
        instance: {
            type: Object,
            default: () => ({}),
        },
        credentials: {
            type: Array,
            default: () => [],
        },
    },
    data() {
        return {
            chatwootBaseUrlInput: '',
            chatwootAccountTokenInput: '',
            chatwootBotTokenInput: '',
            chatwootAccountIdInput: '',
            chatwootInboxIdInput: '',
            chatwootInboxIdentifierInput: '',
            chatwootEnabledInput: true,
            chatwootCredentialIdInput: '',
            savingChatwoot: false,
        };
    },
    computed: {
        chatwootCredentials() {
            return this.credentials.filter((c) => c && c.kind === 'chatwoot');
        },
    },
    watch: {
        instanceId: {
            immediate: true,
            handler(val) {
                if (val && this.instance) {
                    this.loadData();
                }
            },
        },
    },
    methods: {
        loadData() {
            this.chatwootBaseUrlInput = this.instance.chatwoot_base_url || '';
            this.chatwootAccountTokenInput = this.instance.chatwoot_account_token || '';
            this.chatwootBotTokenInput = this.instance.chatwoot_bot_token || '';
            this.chatwootAccountIdInput = this.instance.chatwoot_account_id || '';
            this.chatwootInboxIdInput = this.instance.chatwoot_inbox_id || '';
            this.chatwootInboxIdentifierInput = this.instance.chatwoot_inbox_identifier || '';
            this.chatwootEnabledInput = this.instance.chatwoot_enabled !== false;
            this.chatwootCredentialIdInput = this.instance.chatwoot_credential_id || '';
        },
        getChatwootWebhookUrl() {
            return chatwootWebhookUrl(this.instanceId);
        },
        copyChatwootWebhookUrl() {
            const url = this.getChatwootWebhookUrl();
            copyToClipboard(url, 'Chatwoot webhook URL copied to clipboard.');
        },
        cancel() {
            this.$emit('cancel');
        },
        async save() {
            if (!this.instanceId || this.savingChatwoot) return;
            try {
                this.savingChatwoot = true;
                await window.http.put(`/instances/${this.instanceId}/chatwoot`, {
                    base_url: this.chatwootBaseUrlInput || '',
                    account_id: this.chatwootAccountIdInput || '',
                    inbox_id: this.chatwootInboxIdInput || '',
                    inbox_identifier: this.chatwootInboxIdentifierInput || '',
                    account_token: this.chatwootAccountTokenInput || '',
                    bot_token: this.chatwootBotTokenInput || '',
                    credential_id: this.chatwootCredentialIdInput || '',
                    enabled: !!this.chatwootEnabledInput,
                });
                showSuccessInfo('Chatwoot configuration updated.');
                this.cancel();
            } catch (err) {
                handleApiError(err, 'Failed to update Chatwoot configuration');
            } finally {
                this.savingChatwoot = false;
            }
        },
    },
    template: `
        <div v-if="instanceId" class="modal-overlay">
            <div class="ui segment modal-panel modal-panel--wide">
                <h4 class="ui header">Chatwoot configuration</h4>
                <div class="ui form">
                    <div class="field">
                        <label>Chatwoot webhook URL for this instance</label>
                        <div class="ui fluid action input">
                            <input type="text" :value="getChatwootWebhookUrl()" readonly>
                            <button type="button" class="ui button" @click="copyChatwootWebhookUrl">
                                Copy
                            </button>
                        </div>
                    </div>
                    <div class="field" v-if="chatwootCredentials.length">
                        <label>Chatwoot credential (optional)</label>
                        <select class="ui dropdown" v-model="chatwootCredentialIdInput">
                            <option value="">(Use direct base URL and account token below)</option>
                            <option v-for="cred in chatwootCredentials" :key="cred.id" :value="cred.id">
                                {{ cred.name }} - {{ cred.chatwoot_base_url || 'no base URL' }}
                            </option>
                        </select>
                        <div style="font-size: 0.8em; opacity: 0.7; margin-top: 0.25rem;">
                            When a credential is selected, this instance will use its Chatwoot base URL and account token.
                        </div>
                    </div>
                    <div class="field" v-if="!chatwootCredentialIdInput">
                        <label>Chatwoot API base URL</label>
                        <input type="text" v-model="chatwootBaseUrlInput" placeholder="https://chatwoot.example.com">
                    </div>
                    <div class="field" v-if="!chatwootCredentialIdInput">
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
                    <div class="field">
                        <div class="ui checkbox">
                            <input type="checkbox" v-model="chatwootEnabledInput" id="chatwoot-enabled-toggle">
                            <label for="chatwoot-enabled-toggle">Enable Chatwoot integration for this instance</label>
                        </div>
                    </div>
                    <div class="ui buttons">
                        <button type="button" class="ui primary button" :class="{ loading: savingChatwoot }" @click="save" :disabled="savingChatwoot">
                            Save
                        </button>
                        <div class="or"></div>
                        <button type="button" class="ui button" @click="cancel" :disabled="savingChatwoot">
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `,
};
