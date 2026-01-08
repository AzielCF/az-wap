import { showSuccessInfo, showErrorInfo, handleApiError, filterCredentialsByKind } from './utils.js';

export default {
    name: 'CredentialManager',
    props: {
        credentials: {
            type: Array,
            default: () => [],
        },
        healthStatus: {
            type: Object,
            default: () => ({}),
        },
    },
    data() {
        return {
            loadingCredentials: false,
            showCredentialSection: false,
            showCredentialModal: false,
            credentialEditingId: null,
            credentialNameInput: '',
            credentialKindInput: 'gemini',
            credentialGeminiAPIKeyInput: '',
            credentialChatwootBaseUrlInput: '',
            credentialChatwootAccountTokenInput: '',
            savingCredential: false,
            checkingHealth: {}, // Map of entity_id -> boolean
        };
    },
    created() {
        // Managed by parent
    },
    methods: {
        async checkCredentialHealth(cred) {
            this.checkingHealth[cred.id] = true;
            try {
                const { data } = await window.http.post(`/api/health/credentials/${cred.id}/check`);
                if (data.status === 200 || data.code === 'SUCCESS') {
                    // Update the centralized status via parent if necessary, 
                    // or just show local result if parent doesn't need to sync immediately
                    this.$emit('credentials-updated'); 
                    if (data.results.status === 'ERROR') {
                        window.showErrorInfo(`Health check failed: ${data.results.last_message}`);
                    } else {
                        window.showSuccessInfo('Credential is valid');
                    }
                }
            } catch (err) {
                window.showErrorInfo('Failed to perform health check');
            } finally {
                this.checkingHealth[cred.id] = false;
            }
        },
        getHealthLabel(credId) {
            const h = this.healthStatus[`ia_credential:${credId}`];
            if (!h) return { text: 'UNKNOWN', color: '' };
            return {
                text: h.status,
                color: h.status === 'OK' ? 'green' : (h.status === 'ERROR' ? 'red' : 'yellow'),
                message: h.last_message
            };
        },
        geminiCredentials() {
            return filterCredentialsByKind(this.credentials, 'gemini');
        },
        chatwootCredentials() {
            return filterCredentialsByKind(this.credentials, 'chatwoot');
        },
        openNewCredentialModal(kind = 'gemini') {
            this.credentialEditingId = null;
            this.credentialNameInput = '';
            this.credentialKindInput = kind || 'gemini';
            this.credentialGeminiAPIKeyInput = '';
            this.credentialChatwootBaseUrlInput = '';
            this.credentialChatwootAccountTokenInput = '';
            this.showCredentialModal = true;
        },
        openCredentialEditor(cred) {
            if (!cred || !cred.id) return;
            this.credentialEditingId = cred.id;
            this.credentialNameInput = cred.name || '';
            this.credentialKindInput = cred.kind || 'gemini';
            this.credentialGeminiAPIKeyInput = cred.gemini_api_key || '';
            this.credentialChatwootBaseUrlInput = cred.chatwoot_base_url || '';
            this.credentialChatwootAccountTokenInput = cred.chatwoot_account_token || '';
            this.showCredentialModal = true;
        },
        cancelCredentialEditor() {
            this.showCredentialModal = false;
            this.credentialEditingId = null;
            this.credentialNameInput = '';
            this.credentialKindInput = 'gemini';
            this.credentialGeminiAPIKeyInput = '';
            this.credentialChatwootBaseUrlInput = '';
            this.credentialChatwootAccountTokenInput = '';
        },
        async saveCredential() {
            if (!this.credentialNameInput || this.savingCredential) return;
            const kind = this.credentialKindInput || 'gemini';
            const payload = {
                name: this.credentialNameInput,
                kind,
                gemini_api_key: kind === 'gemini' ? (this.credentialGeminiAPIKeyInput || '') : '',
                chatwoot_base_url: kind === 'chatwoot' ? (this.credentialChatwootBaseUrlInput || '') : '',
                chatwoot_account_token: kind === 'chatwoot' ? (this.credentialChatwootAccountTokenInput || '') : '',
                chatwoot_bot_token: '',
            };
            try {
                this.savingCredential = true;
                if (this.credentialEditingId) {
                    await window.http.put(`/credentials/${this.credentialEditingId}`, payload);
                    showSuccessInfo('Credential updated.');
                } else {
                    await window.http.post('/credentials', payload);
                    showSuccessInfo('Credential created.');
                }
                this.$emit('credentials-updated');
                this.cancelCredentialEditor();
            } catch (err) {
                handleApiError(err, 'Failed to save credential');
            } finally {
                this.savingCredential = false;
            }
        },
        async deleteCredential(cred) {
            if (!cred || !cred.id) return;
            if (!window.confirm(`Delete credential "${cred.name}"? This cannot be undone.`)) {
                return;
            }
            try {
                await window.http.delete(`/credentials/${cred.id}`);
                showSuccessInfo('Credential deleted.');
                this.$emit('credentials-updated');
            } catch (err) {
                handleApiError(err, 'Failed to delete credential');
            }
        },
    },
    template: `
        <div class="field" style="margin-top: 1rem;">
            <div class="ui segment">
                <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                    <div>
                        <h3 class="ui header" style="margin-bottom: 0;">
                            <i class="key icon blue"></i>
                            <div class="content">
                                AI Credentials
                                <div class="sub header">Manage reusable Gemini and Chatwoot credentials</div>
                            </div>
                        </h3>
                    </div>
                    <button type="button" class="ui mini button" @click="showCredentialSection = !showCredentialSection">
                        {{ showCredentialSection ? 'Hide' : 'Show' }}
                    </button>
                </div>
                <div v-if="showCredentialSection" style="margin-top: 0.75rem;">
                    <div class="field" style="margin-bottom: 0.75rem;">
                        <button type="button" class="ui primary mini button" @click="openNewCredentialModal('gemini')" :disabled="loadingCredentials">
                            <i class="plus icon"></i>
                            Create Gemini credential
                        </button>
                        <button type="button" class="ui mini button" style="margin-left: 0.5em;" @click="openNewCredentialModal('chatwoot')" :disabled="loadingCredentials">
                            <i class="plus icon"></i>
                            Create Chatwoot credential
                        </button>
                    </div>
                    <div class="field" v-if="credentials && credentials.length">
                        <label>Existing credentials</label>
                        <table class="ui very basic compact table">
                            <thead>
                            <tr>
                                <th>Name</th>
                                <th>Kind</th>
                                <th>Details</th>
                                <th></th>
                            </tr>
                            </thead>
                            <tbody>
                            <tr v-for="cred in credentials" :key="cred.id">
                                <td>{{ cred.name }}</td>
                                <td>
                                    {{ cred.kind }}
                                    <div v-if="getHealthLabel(cred.id).text !== 'UNKNOWN'" 
                                         class="ui mini label" 
                                         :class="getHealthLabel(cred.id).color" 
                                         style="margin-left: 5px;"
                                         :title="getHealthLabel(cred.id).message">
                                        {{ getHealthLabel(cred.id).text }}
                                    </div>
                                </td>
                                <td>
                                    <span v-if="cred.kind === 'gemini'">Gemini API key stored</span>
                                    <span v-else-if="cred.kind === 'chatwoot'">
                                        {{ cred.chatwoot_base_url || 'Chatwoot base URL configured' }}
                                    </span>
                                </td>
                                <td style="text-align: right;">
                                    <button type="button" class="ui mini basic icon button" @click="checkCredentialHealth(cred)" :class="{ 'loading': checkingHealth[cred.id] }" title="Check health now">
                                        <i class="heartbeat icon" :class="getHealthLabel(cred.id).color"></i>
                                    </button>
                                    <button type="button" class="ui mini basic button" style="margin-left: 0.5em;" @click="openCredentialEditor(cred)">
                                        Edit
                                    </button>
                                    <button type="button" class="ui mini red basic button" style="margin-left: 0.5em;" @click="deleteCredential(cred)">
                                        Delete
                                    </button>
                                </td>
                            </tr>
                            </tbody>
                        </table>
                    </div>
                    <div class="field" v-else>
                        <div class="ui message">
                            <div class="header">No credentials yet</div>
                            <p>Use the buttons above to create Gemini or Chatwoot credentials.</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Modal de credenciales -->
            <div v-if="showCredentialModal" class="modal-overlay">
                <div class="ui segment modal-panel modal-panel--narrow">
                    <h4 class="ui header">{{ credentialEditingId ? 'Edit credential' : 'New credential' }}</h4>
                    <div class="ui form">
                        <div class="fields">
                            <div class="ten wide field">
                                <label>Name</label>
                                <input type="text" v-model="credentialNameInput" placeholder="e.g. Gemini prod" :disabled="savingCredential">
                            </div>
                            <div class="six wide field">
                                <label>Kind</label>
                                <select class="ui dropdown" v-model="credentialKindInput" :disabled="savingCredential || !!credentialEditingId">
                                    <option value="gemini">Gemini</option>
                                    <option value="chatwoot">Chatwoot</option>
                                </select>
                            </div>
                        </div>
                        <div class="field" v-if="credentialKindInput === 'gemini'">
                            <label>Gemini API key</label>
                            <input type="password" v-model="credentialGeminiAPIKeyInput" placeholder="API key from Google AI Studio" :disabled="savingCredential">
                        </div>
                        <div v-if="credentialKindInput === 'chatwoot'">
                            <div class="field">
                                <label>Chatwoot API base URL</label>
                                <input type="text" v-model="credentialChatwootBaseUrlInput" placeholder="https://chatwoot.example.com" :disabled="savingCredential">
                            </div>
                            <div class="field">
                                <label>Account access token</label>
                                <input type="text" v-model="credentialChatwootAccountTokenInput" placeholder="Account API token" :disabled="savingCredential">
                            </div>
                        </div>
                        <div class="ui buttons">
                            <button type="button" class="ui primary button" :class="{ loading: savingCredential }" @click="saveCredential" :disabled="savingCredential || !credentialNameInput">
                                Save credential
                            </button>
                            <div class="or"></div>
                            <button type="button" class="ui button" @click="cancelCredentialEditor" :disabled="savingCredential">
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `,
};
