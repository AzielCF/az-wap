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
            chatwootEnabledInput: true,
            chatwootCredentialIdInput: '',
            savingChatwoot: false,

            geminiEditingInstanceId: null,
            geminiEnabledInput: false,
            geminiAPIKeyInput: '',
            geminiModelInput: '',
            geminiSystemPromptInput: '',
            geminiKnowledgeBaseInput: '',
            geminiTimezoneInput: '',
            geminiAudioEnabledInput: false,
            geminiImageEnabledInput: false,
            geminiMemoryEnabledInput: false,
            savingGemini: false,
            clearingGeminiMemory: false,

            instanceBotIdInput: '',

            bots: [],
            loadingBots: false,
            showBotSection: false,
            editingBotId: null,
            botNameInput: '',
            botDescriptionInput: '',
            botProviderInput: 'gemini',
            botAPIKeyInput: '',
            botModelInput: '',
            botSystemPromptInput: '',
            botKnowledgeBaseInput: '',
            botTimezoneInput: '',
            botAudioEnabledInput: true,
            botImageEnabledInput: true,
            botMemoryEnabledInput: true,
            botCredentialIdInput: '',
            botChatwootCredentialIdInput: '',
            botChatwootBotTokenInput: '',
            savingBot: false,

            // Credential manager
            credentials: [],
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

            showBotModal: false,

            globalGeminiPrompt: '',
            globalGeminiTimezone: '',
            loadingGlobalGeminiPrompt: false,
            savingGlobalGeminiPrompt: false,
            showGlobalIASettings: false,
            showNewInstanceSection: false,
        };
    },
    created() {
        this.loadGlobalGeminiPrompt();
        this.loadBots();
        this.loadCredentials();
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
            // Solo un panel de configuración de instancia visible a la vez
            this.chatwootEditingInstanceId = null;
            this.geminiEditingInstanceId = null;
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
            // Solo un panel de configuración de instancia visible a la vez
            this.editingInstanceId = null;
            this.geminiEditingInstanceId = null;
            this.chatwootEditingInstanceId = inst.id;
            this.chatwootBaseUrlInput = inst.chatwoot_base_url || '';
            this.chatwootAccountTokenInput = inst.chatwoot_account_token || '';
            this.chatwootBotTokenInput = inst.chatwoot_bot_token || '';
            this.chatwootAccountIdInput = inst.chatwoot_account_id || '';
            this.chatwootInboxIdInput = inst.chatwoot_inbox_id || '';
            this.chatwootInboxIdentifierInput = inst.chatwoot_inbox_identifier || '';
            this.chatwootEnabledInput = inst.chatwoot_enabled !== false;
            this.chatwootCredentialIdInput = inst.chatwoot_credential_id || '';
            this.savingChatwoot = false;
        },
        cancelChatwootEditor() {
            this.chatwootEditingInstanceId = null;
            this.chatwootBaseUrlInput = '';
            this.chatwootAccountTokenInput = '';
            this.chatwootBotTokenInput = '';
            this.chatwootAccountIdInput = '';
            this.chatwootInboxIdInput = '';
            this.chatwootInboxIdentifierInput = '';
            this.chatwootEnabledInput = true;
        },
        async saveChatwootConfig() {
            if (!this.chatwootEditingInstanceId || this.savingChatwoot) return;
            try {
                this.savingChatwoot = true;
                await window.http.put(`/instances/${this.chatwootEditingInstanceId}/chatwoot`, {
                    base_url: this.chatwootBaseUrlInput || '',
                    account_id: this.chatwootAccountIdInput || '',
                    inbox_id: this.chatwootInboxIdInput || '',
                    inbox_identifier: this.chatwootInboxIdentifierInput || '',
                    account_token: this.chatwootAccountTokenInput || '',
                    bot_token: this.chatwootBotTokenInput || '',
                    credential_id: this.chatwootCredentialIdInput || '',
                    enabled: !!this.chatwootEnabledInput,
                });
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Chatwoot configuration updated.');
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
        openGeminiEditor(inst) {
            if (!inst || !inst.id) return;
            // Solo un panel de configuración de instancia visible a la vez
            this.editingInstanceId = null;
            this.chatwootEditingInstanceId = null;
            this.geminiEditingInstanceId = inst.id;
            this.geminiEnabledInput = !!inst.gemini_enabled;
            this.geminiAPIKeyInput = inst.gemini_api_key || '';
            this.geminiModelInput = inst.gemini_model || '';
            this.geminiSystemPromptInput = inst.gemini_system_prompt || '';
            this.geminiKnowledgeBaseInput = inst.gemini_knowledge_base || '';
            this.geminiTimezoneInput = inst.gemini_timezone || '';
            this.geminiAudioEnabledInput = !!inst.gemini_audio_enabled;
            this.geminiImageEnabledInput = !!inst.gemini_image_enabled;
            this.geminiMemoryEnabledInput = !!inst.gemini_memory_enabled;
            this.instanceBotIdInput = inst.bot_id || '';
        },
        cancelGeminiEditor() {
            this.geminiEditingInstanceId = null;
            this.geminiEnabledInput = false;
            this.geminiAPIKeyInput = '';
            this.geminiModelInput = '';
            this.geminiSystemPromptInput = '';
            this.geminiKnowledgeBaseInput = '';
            this.geminiTimezoneInput = '';
            this.geminiAudioEnabledInput = false;
            this.geminiImageEnabledInput = false;
            this.geminiMemoryEnabledInput = false;
            this.clearingGeminiMemory = false;
            this.instanceBotIdInput = '';
        },
        async saveGeminiConfig() {
            if (!this.geminiEditingInstanceId) return;
            try {
                this.savingGemini = true;
                await window.http.put(`/instances/${this.geminiEditingInstanceId}/gemini`, {
                    enabled: !!this.geminiEnabledInput,
                    api_key: this.geminiAPIKeyInput || '',
                    model: this.geminiModelInput || '',
                    system_prompt: this.geminiSystemPromptInput || '',
                    knowledge_base: this.geminiKnowledgeBaseInput || '',
                    timezone: this.geminiTimezoneInput || '',
                    audio_enabled: !!this.geminiAudioEnabledInput,
                    image_enabled: !!this.geminiImageEnabledInput,
                    memory_enabled: !!this.geminiMemoryEnabledInput,
                });
                await window.http.put(`/instances/${this.geminiEditingInstanceId}/bot`, {
                    bot_id: this.instanceBotIdInput || '',
                });
                this.$emit('refresh-instances');
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Gemini configuration updated for instance.');
                }
                this.cancelGeminiEditor();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to update Gemini configuration');
                }
            } finally {
                this.savingGemini = false;
            }
        },
        async clearGeminiMemory() {
            if (!this.geminiEditingInstanceId || this.clearingGeminiMemory) return;
            try {
                this.clearingGeminiMemory = true;
                await window.http.post(`/instances/${this.geminiEditingInstanceId}/gemini/memory/clear`, {});
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('IA memory cleared for this instance.');
                }
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to clear IA memory');
                }
            } finally {
                this.clearingGeminiMemory = false;
            }
        },
        async loadGlobalGeminiPrompt() {
            try {
                this.loadingGlobalGeminiPrompt = true;
                const { data } = await window.http.get('/settings/gemini');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to load global IA prompt');
                }
            } finally {
                this.loadingGlobalGeminiPrompt = false;
            }
        },
        async saveGlobalGeminiPrompt() {
            try {
                this.savingGlobalGeminiPrompt = true;
                await window.http.put('/settings/gemini', {
                    global_system_prompt: this.globalGeminiPrompt || '',
                    timezone: this.globalGeminiTimezone || '',
                });
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Global IA prompt updated.');
                }
                const { data } = await window.http.get('/settings/gemini');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to update global IA prompt');
                }
            } finally {
                this.savingGlobalGeminiPrompt = false;
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
        async loadBots() {
            try {
                this.loadingBots = true;
                const { data } = await window.http.get('/bots');
                const results = data?.results || [];
                this.bots = Array.isArray(results) ? results : [];
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to load Bot AI list');
                }
            } finally {
                this.loadingBots = false;
            }
        },
        async loadCredentials() {
            try {
                this.loadingCredentials = true;
                const { data } = await window.http.get('/credentials');
                const results = data?.results || [];
                this.credentials = Array.isArray(results) ? results : [];
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to load IA credentials');
                }
            } finally {
                this.loadingCredentials = false;
            }
        },
        geminiCredentials() {
            if (!Array.isArray(this.credentials)) return [];
            return this.credentials.filter((c) => c && c.kind === 'gemini');
        },
        chatwootCredentials() {
            if (!Array.isArray(this.credentials)) return [];
            return this.credentials.filter((c) => c && c.kind === 'chatwoot');
        },
        botWebhookUrl(bot) {
            if (!bot || !bot.id) return '';
            const base = window.location ? `${window.location.protocol}//${window.location.host}` : '';
            const basePath = window.AppBasePath || '';
            const normalizedBasePath = basePath && basePath !== '/' ? basePath.replace(/\/$/, '') : '';
            return `${base}${normalizedBasePath}/bots/${bot.id}/webhook`;
        },
        copyBotWebhookUrl(bot) {
            const url = this.botWebhookUrl(bot);
            if (!url) return;
            if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(url).then(() => {
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Bot webhook URL copied to clipboard.');
                    }
                }).catch((err) => {
                    if (typeof window.showErrorInfo === 'function') {
                        window.showErrorInfo(err?.message || 'Failed to copy Bot webhook URL');
                    }
                });
            }
        },
        openNewBotModal() {
            this.openNewBotForm();
            this.showBotModal = true;
        },
        openNewBotForm() {
            this.editingBotId = null;
            this.botNameInput = '';
            this.botDescriptionInput = '';
            this.botProviderInput = 'gemini';
            this.botAPIKeyInput = '';
            this.botModelInput = '';
            this.botSystemPromptInput = '';
            this.botKnowledgeBaseInput = '';
            this.botTimezoneInput = '';
            this.botAudioEnabledInput = true;
            this.botImageEnabledInput = true;
            this.botMemoryEnabledInput = true;
            this.botCredentialIdInput = '';
            this.botChatwootCredentialIdInput = '';
            this.botChatwootBotTokenInput = '';
        },
        openBotEditor(bot) {
            if (!bot || !bot.id) return;
            this.editingBotId = bot.id;
            this.botNameInput = bot.name || '';
            this.botDescriptionInput = bot.description || '';
            this.botProviderInput = bot.provider || 'gemini';
            this.botAPIKeyInput = bot.api_key || '';
            this.botModelInput = bot.model || '';
            this.botSystemPromptInput = bot.system_prompt || '';
            this.botKnowledgeBaseInput = bot.knowledge_base || '';
            this.botTimezoneInput = bot.timezone || '';
            this.botAudioEnabledInput = !!bot.audio_enabled;
            this.botImageEnabledInput = !!bot.image_enabled;
            this.botMemoryEnabledInput = !!bot.memory_enabled;
            this.botCredentialIdInput = bot.credential_id || '';
            this.botChatwootCredentialIdInput = bot.chatwoot_credential_id || '';
            this.botChatwootBotTokenInput = bot.chatwoot_bot_token || '';
            this.showBotModal = true;
        },
        cancelBotEditor() {
            this.showBotModal = false;
            this.openNewBotForm();
        },
        async saveBot() {
            if (!this.botNameInput || this.savingBot) return;
            const payload = {
                name: this.botNameInput,
                description: this.botDescriptionInput || '',
                provider: this.botProviderInput || 'gemini',
                api_key: this.botAPIKeyInput || '',
                model: this.botModelInput || '',
                system_prompt: this.botSystemPromptInput || '',
                knowledge_base: this.botKnowledgeBaseInput || '',
                timezone: this.botTimezoneInput || '',
                audio_enabled: !!this.botAudioEnabledInput,
                image_enabled: !!this.botImageEnabledInput,
                memory_enabled: !!this.botMemoryEnabledInput,
                credential_id: this.botCredentialIdInput || '',
                chatwoot_credential_id: this.botChatwootCredentialIdInput || '',
                chatwoot_bot_token: this.botChatwootBotTokenInput || '',
            };
            try {
                this.savingBot = true;
                if (this.editingBotId) {
                    await window.http.put(`/bots/${this.editingBotId}`, payload);
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Bot AI updated.');
                    }
                } else {
                    await window.http.post('/bots', payload);
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Bot AI created.');
                    }
                }
                await this.loadBots();
                this.showBotModal = false;
                this.openNewBotForm();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to save Bot AI');
                }
            } finally {
                this.savingBot = false;
            }
        },
        async deleteBot(bot) {
            if (!bot || !bot.id) return;
            if (!window.confirm(`Delete Bot AI "${bot.name}"? This cannot be undone.`)) {
                return;
            }
            try {
                await window.http.delete(`/bots/${bot.id}`);
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Bot AI deleted.');
                }
                await this.loadBots();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to delete Bot AI');
                }
            }
        },
        async clearBotMemory(bot) {
            if (!bot || !bot.id) return;
            if (!window.confirm(`Clear IA memory for Bot AI "${bot.name}"?`)) {
                return;
            }
            try {
                await window.http.post(`/bots/${bot.id}/memory/clear`, {});
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Bot AI memory cleared.');
                }
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to clear Bot AI memory');
                }
            }
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
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Credential updated.');
                    }
                } else {
                    await window.http.post('/credentials', payload);
                    if (typeof window.showSuccessInfo === 'function') {
                        window.showSuccessInfo('Credential created.');
                    }
                }
                await this.loadCredentials();
                this.cancelCredentialEditor();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to save credential');
                }
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
                if (typeof window.showSuccessInfo === 'function') {
                    window.showSuccessInfo('Credential deleted.');
                }
                await this.loadCredentials();
            } catch (err) {
                if (typeof window.showErrorInfo === 'function') {
                    window.showErrorInfo(err?.response?.data?.message || err.message || 'Failed to delete credential');
                }
            }
        },
    },
    template: `
    <div class="instance-manager">
        <div class="green card" style="cursor: default; width: 100%;">
            <div class="content">
            <a class="ui teal right ribbon label">Instances</a>
            <div class="header">Instance Manager</div>
            <div class="description">
                <div class="ui form">
                    <div class="field">
                        <div class="ui segment">
                            <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                                <div>
                                    <h4 class="ui header" style="margin-bottom: 0;">Global IA settings</h4>
                                    <div style="font-size: 0.9em; opacity: 0.7;">Applies to all IA assistants in all instances.</div>
                                </div>
                                <button type="button" class="ui mini button" @click="showGlobalIASettings = !showGlobalIASettings">
                                    {{ showGlobalIASettings ? 'Hide' : 'Show' }}
                                </button>
                            </div>
                            <div v-if="showGlobalIASettings" style="margin-top: 0.75rem;">
                                <label>Global IA system prompt</label>
                                <textarea rows="4" v-model="globalGeminiPrompt" placeholder="Global rules for all Gemini assistants"></textarea>
                                <div class="field" style="margin-top: 0.5rem;">
                                    <label>IA timezone (IANA)</label>
                                    <select class="ui dropdown" v-model="globalGeminiTimezone">
                                        <option value="">(Use server default / UTC)</option>
                                        <option value="UTC">UTC</option>
                                        <option value="America/Bogota">America/Bogota</option>
                                        <option value="America/Lima">America/Lima</option>
                                        <option value="America/Mexico_City">America/Mexico_City</option>
                                        <option value="America/Santo_Domingo">America/Santo_Domingo (República Dominicana)</option>
                                        <option value="America/Santiago">America/Santiago</option>
                                        <option value="America/Argentina/Buenos_Aires">America/Argentina/Buenos_Aires</option>
                                        <option value="America/Los_Angeles">America/Los_Angeles</option>
                                        <option value="America/New_York">America/New_York</option>
                                        <option value="Europe/Madrid">Europe/Madrid</option>
                                        <option value="Europe/London">Europe/London</option>
                                    </select>
                                </div>
                                </div>
                                <button type="button" class="ui button" :class="{ loading: savingGlobalGeminiPrompt }" @click="saveGlobalGeminiPrompt" :disabled="savingGlobalGeminiPrompt">
                                    Save global settings
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="field" style="margin-top: 1rem;">
                        <div class="ui segment">
                            <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                                <div>
                                    <h4 class="ui header" style="margin-bottom: 0;">IA Credentials</h4>
                                    <div style="font-size: 0.9em; opacity: 0.7;">Manage reusable Gemini and Chatwoot credentials.</div>
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
                                            <td>{{ cred.kind }}</td>
                                            <td>
                                                <span v-if="cred.kind === 'gemini'">Gemini API key stored</span>
                                                <span v-else-if="cred.kind === 'chatwoot'">
                                                    {{ cred.chatwoot_base_url || 'Chatwoot base URL configured' }}
                                                </span>
                                            </td>
                                            <td style="text-align: right;">
                                                <button type="button" class="ui mini basic button" @click="openCredentialEditor(cred)">
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
                    </div>
                    <div class="field" style="margin-top: 1rem;">
                        <div class="ui segment">
                            <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                                <div>
                                    <h4 class="ui header" style="margin-bottom: 0;">New instance</h4>
                                    <div style="font-size: 0.9em; opacity: 0.7;">Create a new WhatsApp instance with its own token.</div>
                                </div>
                                <button type="button" class="ui mini button" @click="showNewInstanceSection = !showNewInstanceSection">
                                    {{ showNewInstanceSection ? 'Hide' : 'Show' }}
                                </button>
                            </div>
                            <div v-if="showNewInstanceSection" style="margin-top: 0.75rem;">
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
                            </div>
                        </div>
                    </div>
                    <div class="field" style="margin-top: 1rem;">
                        <div class="ui segment">
                            <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                                <div>
                                    <h4 class="ui header" style="margin-bottom: 0;">Reusable Bot AI</h4>
                                    <div style="font-size: 0.9em; opacity: 0.7;">Create shared IA assistants and reuse them across instances.</div>
                                </div>
                                <button type="button" class="ui mini button" @click="showBotSection = !showBotSection">
                                    {{ showBotSection ? 'Hide' : 'Show' }}
                                </button>
                            </div>
                            <div v-if="showBotSection" style="margin-top: 0.75rem;">
                                <div class="field" style="margin-bottom: 0.75rem;">
                                    <button type="button" class="ui primary mini button" @click="openNewBotModal">
                                        <i class="plus icon"></i>
                                        Create Bot AI
                                    </button>
                                </div>
                                <div class="field" v-if="bots && bots.length">
                                    <label>Existing Bot AI assistants</label>
                                    <table class="ui very basic compact table">
                                        <thead>
                                        <tr>
                                            <th>Name</th>
                                            <th>Provider</th>
                                            <th>Webhook</th>
                                            <th>Audio</th>
                                            <th>Image</th>
                                            <th>Memory</th>
                                            <th></th>
                                        </tr>
                                        </thead>
                                        <tbody>
                                        <tr v-for="bot in bots" :key="bot.id">
                                            <td>{{ bot.name }}</td>
                                            <td>{{ bot.provider }}</td>
                                            <td>
                                                <div class="ui action input" style="max-width: 260px;">
                                                    <input type="text" :value="botWebhookUrl(bot)" readonly>
                                                    <button type="button" class="ui mini button" @click="copyBotWebhookUrl(bot)">
                                                        Copy
                                                    </button>
                                                </div>
                                            </td>
                                            <td>{{ bot.audio_enabled ? 'Yes' : 'No' }}</td>
                                            <td>{{ bot.image_enabled ? 'Yes' : 'No' }}</td>
                                            <td>{{ bot.memory_enabled ? 'Yes' : 'No' }}</td>
                                            <td style="text-align: right;">
                                                <button type="button" class="ui mini basic button" @click="openBotEditor(bot)">
                                                    Edit
                                                </button>
                                                <button
                                                    v-if="bot.memory_enabled"
                                                    type="button"
                                                    class="ui mini orange basic button"
                                                    style="margin-left: 0.5em;"
                                                    @click="clearBotMemory(bot)"
                                                >
                                                    Clear memory
                                                </button>
                                                <button type="button" class="ui mini red basic button" style="margin-left: 0.5em;" @click="deleteBot(bot)">
                                                    Delete
                                                </button>
                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>
                                </div>
                                <div class="field" v-else>
                                    <div class="ui message">
                                        <div class="header">No Bot AI assistants yet</div>
                                        <p>Click "Create Bot AI" to add your first reusable assistant.</p>
                                    </div>
                                </div>
                            </div>
                        </div>
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
                                <button type="button" class="ui mini basic button" @click="openGeminiEditor(inst)" style="margin-left: 0.5em;">
                                    IA Assistant
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
                    <div v-if="geminiEditingInstanceId" class="ui segment" style="margin-top: 1em;">
                        <h4 class="ui header">IA Assistant configuration</h4>
                        <div class="ui form">
                            <div class="field">
                                <div class="ui checkbox">
                                    <input type="checkbox" v-model="geminiEnabledInput" id="gemini-enabled-toggle">
                                    <label for="gemini-enabled-toggle">Enable IA Assistant (Gemini) for this instance</label>
                                </div>
                            </div>
                            <div v-if="geminiEnabledInput">
                                <div class="field" v-if="bots && bots.length">
                                    <label>Reusable Bot AI (optional)</label>
                                    <select class="ui dropdown" v-model="instanceBotIdInput">
                                        <option :value="''">(Use instance-specific IA config)</option>
                                        <option v-for="bot in bots" :key="bot.id" :value="bot.id">
                                            {{ bot.name }} - {{ bot.provider }}
                                        </option>
                                    </select>
                                    <div style="font-size: 0.85em; opacity: 0.7; margin-top: 0.25rem;">
                                        When a Bot AI is selected, this instance will use its configuration (API key, model, prompts, etc.).
                                    </div>
                                </div>
                                <div v-if="!instanceBotIdInput">
                                <div class="field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="geminiMemoryEnabledInput" id="gemini-memory-enabled-toggle">
                                        <label for="gemini-memory-enabled-toggle">Use chat memory (in-memory only)</label>
                                    </div>
                                </div>
                                <div class="field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="geminiAudioEnabledInput" id="gemini-audio-enabled-toggle">
                                        <label for="gemini-audio-enabled-toggle">Let IA understand voice messages (audio)</label>
                                    </div>
                                </div>
                                <div class="field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="geminiImageEnabledInput" id="gemini-image-enabled-toggle">
                                        <label for="gemini-image-enabled-toggle">Let IA understand and describe images (image messages)</label>
                                    </div>
                                </div>
                                <div class="field">
                                    <label>Gemini API key</label>
                                    <input type="password" v-model="geminiAPIKeyInput" placeholder="API key from Google AI Studio">
                                </div>
                                <div class="field">
                                    <label>Gemini model</label>
                                    <input type="text" v-model="geminiModelInput" placeholder="e.g. gemini-2.5-flash">
                                </div>
                                <div class="field">
                                    <label>System prompt</label>
                                    <textarea rows="3" v-model="geminiSystemPromptInput" placeholder="High-level instructions for the assistant"></textarea>
                                </div>
                                <div class="field">
                                    <label>Knowledge base (instance-specific)</label>
                                    <textarea rows="4" v-model="geminiKnowledgeBaseInput" placeholder="General knowledge and FAQs for this instance"></textarea>
                                </div>
                                <div class="field">
                                    <label>IA timezone (IANA)</label>
                                    <select class="ui dropdown" v-model="geminiTimezoneInput">
                                        <option value="">(Use global / server default)</option>
                                        <option value="UTC">UTC</option>
                                        <option value="America/Bogota">America/Bogota</option>
                                        <option value="America/Lima">America/Lima</option>
                                        <option value="America/Mexico_City">America/Mexico_City</option>
                                        <option value="America/Santo_Domingo">America/Santo_Domingo (República Dominicana)</option>
                                        <option value="America/Santiago">America/Santiago</option>
                                        <option value="America/Argentina/Buenos_Aires">America/Argentina/Buenos_Aires</option>
                                        <option value="America/Los_Angeles">America/Los_Angeles</option>
                                        <option value="America/New_York">America/New_York</option>
                                        <option value="Europe/Madrid">Europe/Madrid</option>
                                        <option value="Europe/London">Europe/London</option>
                                    </select>
                                </div>
                                <div class="field">
                                    <button type="button" class="ui button" :class="{ loading: clearingGeminiMemory }" @click="clearGeminiMemory" :disabled="clearingGeminiMemory">
                                        Clear IA memory for this instance
                                    </button>
                                </div>
                            </div>
                            <div class="ui buttons">
                                <button type="button" class="ui primary button" :class="{ loading: savingGemini }" @click="saveGeminiConfig" :disabled="savingGemini">
                                    Save
                                </button>
                                <div class="or"></div>
                                <button type="button" class="ui button" @click="cancelGeminiEditor" :disabled="savingGemini">
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

            <!-- Modal Chatwoot configuración de instancia -->
            <div v-if="chatwootEditingInstanceId" class="modal-overlay">
                <div class="ui segment modal-panel modal-panel--wide">
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
                        <div class="field" v-if="chatwootCredentials().length">
                            <label>Chatwoot credential (optional)</label>
                            <select class="ui dropdown" v-model="chatwootCredentialIdInput">
                                <option value="">(Use direct base URL and account token below)</option>
                                <option v-for="cred in chatwootCredentials()" :key="cred.id" :value="cred.id">
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

            <div v-if="showBotModal" class="modal-overlay">
            <div class="ui segment modal-panel modal-panel--wide">
                <h4 class="ui header">{{ editingBotId ? 'Edit Bot AI' : 'New Bot AI' }}</h4>
                <div class="ui form">
                            <div class="fields">
                                <div class="eight wide field">
                                    <label>Name</label>
                                    <input type="text" v-model="botNameInput" placeholder="e.g. ventas-es" :disabled="savingBot">
                                </div>
                                <div class="eight wide field">
                                    <label>Provider</label>
                                    <select class="ui dropdown" v-model="botProviderInput" :disabled="savingBot">
                                        <option value="gemini">Gemini</option>
                                    </select>
                                </div>
                            </div>
                            <div class="field" v-if="geminiCredentials().length">
                                <label>Gemini credential (optional)</label>
                                <select class="ui dropdown" v-model="botCredentialIdInput" :disabled="savingBot">
                                    <option value="">(Use direct API key below)</option>
                                    <option v-for="cred in geminiCredentials()" :key="cred.id" :value="cred.id">
                                        {{ cred.name }}
                                    </option>
                                </select>
                                <div style="font-size: 0.8em; opacity: 0.7; margin-top: 0.25rem;">
                                    When a credential is selected, this Bot AI will use its Gemini API key.
                                </div>
                            </div>
                            <div class="field">
                                <label>Description</label>
                                <input type="text" v-model="botDescriptionInput" placeholder="Short description" :disabled="savingBot">
                            </div>
                            <div class="field" v-if="chatwootCredentials().length">
                                <label>Chatwoot credential (optional)</label>
                                <select class="ui dropdown" v-model="botChatwootCredentialIdInput" :disabled="savingBot">
                                    <option value="">(No Chatwoot credential)</option>
                                    <option v-for="cred in chatwootCredentials()" :key="cred.id" :value="cred.id">
                                        {{ cred.name }} - {{ cred.chatwoot_base_url || 'no base URL' }}
                                    </option>
                                </select>
                                <div style="font-size: 0.8em; opacity: 0.7; margin-top: 0.25rem;">
                                    When a Chatwoot credential is selected, this Bot AI will be able to use its Chatwoot base URL and account token.
                                </div>
                            </div>
                            <div class="field">
                                <label>Chatwoot Bot token</label>
                                <input type="text" v-model="botChatwootBotTokenInput" placeholder="Chatwoot bot access token for this Bot AI" :disabled="savingBot">
                            </div>
                            <div class="field" v-if="!botCredentialIdInput">
                                <label>API key</label>
                                <input type="password" v-model="botAPIKeyInput" placeholder="API key for this Bot AI" :disabled="savingBot">
                            </div>
                            <div class="field">
                                <label>Model</label>
                                <input type="text" v-model="botModelInput" placeholder="e.g. gemini-2.5-flash" :disabled="savingBot">
                            </div>
                            <div class="field">
                                <label>System prompt</label>
                                <textarea rows="3" v-model="botSystemPromptInput" placeholder="High-level instructions for this Bot AI" :disabled="savingBot"></textarea>
                            </div>
                            <div class="field">
                                <label>Knowledge base</label>
                                <textarea rows="3" v-model="botKnowledgeBaseInput" placeholder="Domain knowledge and FAQs for this Bot AI" :disabled="savingBot"></textarea>
                            </div>
                            <div class="field">
                                <label>IA timezone (IANA)</label>
                                <select class="ui dropdown" v-model="botTimezoneInput" :disabled="savingBot">
                                    <option value="">(Use global / server default)</option>
                                    <option value="UTC">UTC</option>
                                    <option value="America/Bogota">America/Bogota</option>
                                    <option value="America/Lima">America/Lima</option>
                                    <option value="America/Mexico_City">America/Mexico_City</option>
                                    <option value="America/Santo_Domingo">America/Santo_Domingo (República Dominicana)</option>
                                    <option value="America/Santiago">America/Santiago</option>
                                    <option value="America/Argentina/Buenos_Aires">America/Argentina/Buenos_Aires</option>
                                    <option value="America/Los_Angeles">America/Los_Angeles</option>
                                    <option value="America/New_York">America/New_York</option>
                                    <option value="Europe/Madrid">Europe/Madrid</option>
                                    <option value="Europe/London">Europe/London</option>
                                </select>
                            </div>
                            <div class="fields">
                                <div class="four wide field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="botAudioEnabledInput" id="bot-audio-enabled-toggle" :disabled="savingBot">
                                        <label for="bot-audio-enabled-toggle">Audio</label>
                                    </div>
                                </div>
                                <div class="four wide field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="botImageEnabledInput" id="bot-image-enabled-toggle" :disabled="savingBot">
                                        <label for="bot-image-enabled-toggle">Image</label>
                                    </div>
                                </div>
                                <div class="four wide field">
                                    <div class="ui checkbox">
                                        <input type="checkbox" v-model="botMemoryEnabledInput" id="bot-memory-enabled-toggle" :disabled="savingBot">
                                        <label for="bot-memory-enabled-toggle">Memory</label>
                                    </div>
                                </div>
                            </div>
                            <div class="ui buttons">
                                <button type="button" class="ui primary button" :class="{ loading: savingBot }" @click="saveBot" :disabled="savingBot || !botNameInput">
                                    Save Bot AI
                                </button>
                                <div class="or"></div>
                                <button type="button" class="ui button" @click="cancelBotEditor" :disabled="savingBot">
                                    Cancel
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
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
