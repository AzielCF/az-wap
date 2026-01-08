import { showSuccessInfo, handleApiError, botWebhookUrl, copyToClipboard } from './utils.js';

export default {
    name: 'BotManager',
    props: {
        credentials: {
            type: Array,
            default: () => [],
        },
    },
    data() {
        return {
            bots: [],
            loadingBots: false,
            showBotSection: false,
            showBotModal: false,
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
            botMCPServers: [],
            loadingBotMCPs: false,
        };
    },
    created() {
        this.loadBots();
    },
    computed: {
        geminiCredentials() {
            return this.credentials.filter((c) => c && c.kind === 'gemini');
        },
        chatwootCredentials() {
            return this.credentials.filter((c) => c && c.kind === 'chatwoot');
        },
    },
    watch: {
        showBotModal(newVal) {
            if (newVal) {
                // Inicializar dropdowns cuando se abre el modal
                this.$nextTick(() => {
                    // Configurar dropdowns SIN clearable para que la opción vacía sea visible
                    $('#bot-provider-dropdown').dropdown();
                    $('#bot-credential-dropdown').dropdown();
                    $('#bot-chatwoot-credential-dropdown').dropdown();
                    $('#bot-timezone-dropdown').dropdown();
                });
            }
        },
    },
    methods: {
        async loadBots() {
            try {
                this.loadingBots = true;
                const { data } = await window.http.get('/bots');
                const results = data?.results || [];
                this.bots = Array.isArray(results) ? results : [];
                this.$emit('bots-loaded', this.bots);
            } catch (err) {
                handleApiError(err, 'Failed to load Bot AI list');
            } finally {
                this.loadingBots = false;
            }
        },
        getBotWebhookUrl(bot) {
            return botWebhookUrl(bot.id);
        },
        copyBotWebhookUrl(bot) {
            const url = this.getBotWebhookUrl(bot);
            copyToClipboard(url, 'Bot webhook URL copied to clipboard.');
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
            this.botTimezoneInput = '__NONE__';
            this.botAudioEnabledInput = true;
            this.botImageEnabledInput = true;
            this.botMemoryEnabledInput = true;
            this.botCredentialIdInput = '__NONE__';
            this.botChatwootCredentialIdInput = '__NONE__';
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
            this.botTimezoneInput = bot.timezone || '__NONE__';
            this.botAudioEnabledInput = !!bot.audio_enabled;
            this.botImageEnabledInput = !!bot.image_enabled;
            this.botMemoryEnabledInput = !!bot.memory_enabled;
            this.botCredentialIdInput = bot.credential_id || '__NONE__';
            this.botChatwootCredentialIdInput = bot.chatwoot_credential_id || '__NONE__';
            this.botChatwootBotTokenInput = bot.chatwoot_bot_token || '';
            this.showBotModal = true;
            
            // Actualizar los valores de los dropdowns después de que el modal se renderice
            this.$nextTick(() => {
                // Establecer los valores de los dropdowns usando Semantic UI por ID
                // Ahora la opción vacía es visible, así que podemos seleccionarla directamente
                $('#bot-provider-dropdown').dropdown('set selected', this.botProviderInput);
                $('#bot-credential-dropdown').dropdown('set selected', this.botCredentialIdInput);
                $('#bot-chatwoot-credential-dropdown').dropdown('set selected', this.botChatwootCredentialIdInput);
                $('#bot-timezone-dropdown').dropdown('set selected', this.botTimezoneInput);
            });

            this.loadBotMCPs(bot.id);
        },
        async loadBotMCPs(botId) {
            this.loadingBotMCPs = true;
            try {
                const { data } = await window.http.get(`/bots/${botId}/mcp`);
                const results = data?.results || [];
                // Add headersString to each server for editing
                // Prepare headersMap for each server based on template or existing custom_headers
                this.botMCPServers = results.map(srv => {
                    let headersMap = {};
                    let headersString = '';

                    // If it has custom_headers saved, use them
                    if (srv.custom_headers) {
                        headersMap = { ...srv.custom_headers };
                        // Also build string for non-template mode fallback
                        headersString = Object.entries(srv.custom_headers)
                            .map(([k, v]) => `${k}: ${v}`)
                            .join('\n');
                    }

                    // If it is a template, ensure all keys exist in headersMap
                    if (srv.is_template && srv.template_config) {
                        Object.keys(srv.template_config).forEach(k => {
                            if (!headersMap[k]) headersMap[k] = '';
                        });
                    }

                    return { ...srv, headersString, headersMap };
                });
            } catch (err) {
                console.error('Failed to load bot MCPs:', err);
            } finally {
                this.loadingBotMCPs = false;
            }
        },
        async saveMCPPreference(server) {
            if (!this.editingBotId) return;

            let headers = {};

            // If template, construct from headersMap
            if (server.is_template) {
                headers = { ...server.headersMap };
            } else {
                // Legacy / Non-template mode: Parse headers (Key: Value) from textarea
                (server.headersString || '').split('\n').forEach(line => {
                    const idx = line.indexOf(':');
                    if (idx > 0) {
                        const key = line.substring(0, idx).trim();
                        const val = line.substring(idx + 1).trim();
                        if (key) headers[key] = val;
                    }
                });
            }

            try {
                await window.http.put(`/bots/${this.editingBotId}/mcp/${server.id}`, {
                    enabled: server.enabled,
                    disabled_tools: server.disabled_tools || [],
                    custom_headers: headers
                });
                return true;
            } catch (err) {
                handleApiError(err, 'Failed to update MCP preference');
                return false;
            }
        },
        async toggleMCPForBot(server) {
            // If enabling a template, don't auto-save immediately to enabled=true.
            // Just expand the section and let user fill info. 
            // BUT for UX simplification as per request: "if not template, activate. if template, ask."
            
            if (server.is_template && !server.enabled) {
                // Expanding will happen automatically because we are not toggling 'enabled' yet?
                // Actually the checkbox is bound to :checked="srv.enabled". 
                // We need to allow the toggle to happen visually but maybe not save 'enabled=true' if validation fails?
                // Let's just toggle 'enabled' state locally.
                server.enabled = !server.enabled;

                // If now enabled, check if we need to show validation error or just save
                // We save immediately to 'enabled=true', user then fills headers.
                await this.saveMCPPreference(server);
            } else {
                server.enabled = !server.enabled;
                await this.saveMCPPreference(server);
            }
            // Reload to refresh state
            this.loadBotMCPs(this.editingBotId);
        },
        getTemplateConfig(server) {
            return server.template_config || {};
        },
        isToolDisabled(server, toolName) {
            if (!server.disabled_tools) return false;
            return server.disabled_tools.includes(toolName);
        },
        async toggleToolForBot(server, toolName) {
            if (!this.editingBotId) return;
            
            let disabledTools = [...(server.disabled_tools || [])];
            if (disabledTools.includes(toolName)) {
                disabledTools = disabledTools.filter(t => t !== toolName);
            } else {
                disabledTools.push(toolName);
            }
            server.disabled_tools = disabledTools;

            await this.saveMCPPreference(server);
        },
        async saveBotMCPHeaders(server) {
            const ok = await this.saveMCPPreference(server);
            if (ok) showSuccessInfo(`MCP headers updated for ${server.name}`);
        },
        cancelBotEditor() {
            this.showBotModal = false;
            this.openNewBotForm();
        },
        async saveBot() {
            if (!this.botNameInput || this.savingBot) return;
            
            // Convertir '__NONE__' a '' para las credenciales
            const credentialId = this.botCredentialIdInput === '__NONE__' ? '' : this.botCredentialIdInput;
            const chatwootCredentialId = this.botChatwootCredentialIdInput === '__NONE__' ? '' : this.botChatwootCredentialIdInput;
            
            const payload = {
                name: this.botNameInput,
                description: this.botDescriptionInput || '',
                provider: this.botProviderInput || 'gemini',
                api_key: this.botAPIKeyInput || '',
                model: this.botModelInput || '',
                system_prompt: this.botSystemPromptInput || '',
                knowledge_base: this.botKnowledgeBaseInput || '',
                timezone: this.botTimezoneInput === '__NONE__' ? '' : this.botTimezoneInput,
                audio_enabled: !!this.botAudioEnabledInput,
                image_enabled: !!this.botImageEnabledInput,
                memory_enabled: !!this.botMemoryEnabledInput,
                credential_id: credentialId,
                chatwoot_credential_id: chatwootCredentialId,
                chatwoot_bot_token: this.botChatwootBotTokenInput || '',
            };
            try {
                this.savingBot = true;
                if (this.editingBotId) {
                    await window.http.put(`/bots/${this.editingBotId}`, payload);
                    showSuccessInfo('Bot AI updated.');
                } else {
                    await window.http.post('/bots', payload);
                    showSuccessInfo('Bot AI created.');
                }
                await this.loadBots();
                this.showBotModal = false;
                this.openNewBotForm();
            } catch (err) {
                handleApiError(err, 'Failed to save Bot AI');
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
                showSuccessInfo('Bot AI deleted.');
                await this.loadBots();
            } catch (err) {
                handleApiError(err, 'Failed to delete Bot AI');
            }
        },
        async clearBotMemory(bot) {
            if (!bot || !bot.id) return;
            if (!window.confirm(`Clear IA memory for Bot AI "${bot.name}"?`)) {
                return;
            }
            try {
                await window.http.post(`/bots/${bot.id}/memory/clear`, {});
                showSuccessInfo('Bot AI memory cleared.');
            } catch (err) {
                handleApiError(err, 'Failed to clear Bot AI memory');
            }
        },
    },
    template: `
        <div class="field" style="margin-top: 1rem;">
            <div class="ui segment">
                <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                    <div>
                        <h3 class="ui header" style="margin-bottom: 0;">
                            <i class="robot icon blue"></i>
                            <div class="content">
                                Reusable Bot AI
                                <div class="sub header">Create shared AI assistants and reuse them across instances</div>
                            </div>
                        </h3>
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
                                        <input type="text" :value="getBotWebhookUrl(bot)" readonly>
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

            <!-- Modal de Bot AI -->
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
                                <select id="bot-provider-dropdown" class="ui dropdown" v-model="botProviderInput" :disabled="savingBot">
                                    <option value="gemini">Gemini</option>
                                </select>
                            </div>
                        </div>
                        <div class="field">
                            <label>Gemini credential (optional)</label>
                            <select id="bot-credential-dropdown" class="ui dropdown" v-model="botCredentialIdInput" :disabled="savingBot">
                                <option value="__NONE__">(No credential - use direct API key below)</option>
                                <option v-if="!geminiCredentials.length" disabled>── No credentials available ──</option>
                                <option v-for="cred in geminiCredentials" :key="cred.id" :value="cred.id">
                                    {{ cred.name }}
                                </option>
                            </select>
                            <div style="font-size: 0.8em; opacity: 0.7; margin-top: 0.25rem;">
                                Select a credential to reuse an existing API key, or leave empty to enter a direct API key below.
                            </div>
                        </div>
                        <div class="field">
                            <label>Description</label>
                            <input type="text" v-model="botDescriptionInput" placeholder="Short description" :disabled="savingBot">
                        </div>
                        <div class="field">
                            <label>Chatwoot credential (optional)</label>
                            <select id="bot-chatwoot-credential-dropdown" class="ui dropdown" v-model="botChatwootCredentialIdInput" :disabled="savingBot">
                                <option value="__NONE__">(No Chatwoot credential)</option>
                                <option v-if="!chatwootCredentials.length" disabled>── No credentials available ──</option>
                                <option v-for="cred in chatwootCredentials" :key="cred.id" :value="cred.id">
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
                        <div class="field" v-if="!botCredentialIdInput || botCredentialIdInput === '__NONE__'">
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
                            <label>AI timezone (IANA)</label>
                            <select id="bot-timezone-dropdown" class="ui dropdown" v-model="botTimezoneInput" :disabled="savingBot">
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
                        </div>

                        <!-- MCP Tools for this bot -->
                        <div v-if="editingBotId" class="ui segment">
                            <h5 class="ui header">
                                <i class="tools icon"></i>
                                Available MCP Tools
                            </h5>
                            <div v-if="loadingBotMCPs" class="ui centered inline loader active"></div>
                            <div v-else-if="botMCPServers.length === 0" class="ui message small">
                                No MCP servers available. Configure them in the Instance Manager first.
                            </div>
                            <div v-else class="ui middle aligned divided list">
                                <div v-for="srv in botMCPServers" :key="srv.id" class="item">
                                    <div class="right floated content">
                                        <div class="ui toggle checkbox">
                                            <input type="checkbox" :checked="srv.enabled" @change="toggleMCPForBot(srv)">
                                            <label></label>
                                        </div>
                                    </div>
                                    <div class="content">
                                        <div class="header" style="font-size: 0.95em;">
                                            {{ srv.name }}
                                            <span v-if="srv.tools && srv.tools.length" class="ui tiny label">{{ srv.tools.length }} tools</span>
                                        </div>
                                        <div class="description" style="font-size: 0.85em;">{{ srv.description || srv.url }}</div>
                                        
                                        <!-- Tool detail / selection -->
                                        <div v-if="srv.enabled" style="margin-top: 10px; padding-left: 20px; border-left: 2px solid #eee;">
                                            <div v-if="srv.is_template" class="field" style="margin-bottom: 1rem;">
                                                <label style="font-size: 0.85em; font-weight: bold; display: block; margin-bottom: 5px;">
                                                    Required Configuration
                                                </label>

                                                <!-- Template Mode -->
                                                <div v-if="srv.template_config" class="ui form tiny">
                                                    <div v-for="(help, key) in srv.template_config" :key="key" class="field required">
                                                        <label style="font-size: 0.8em;">{{ key }}</label>
                                                        <div class="ui input">
                                                            <input type="text" v-model="srv.headersMap[key]" :placeholder="help || 'Value required'">
                                                        </div>
                                                        <div v-if="help" style="font-size: 0.75em; opacity: 0.6; margin-top:2px;">{{ help }}</div>
                                                    </div>
                                                    <div style="text-align: right; margin-top: 5px;">
                                                        <button type="button" class="ui icon button teal tiny" @click="saveBotMCPHeaders(srv)">
                                                            <i class="save icon"></i> Save Configuration
                                                        </button>
                                                    </div>
                                                </div>
                                            </div>

                                            <div v-if="!srv.tools || !srv.tools.length" class="ui tiny orange message">
                                                No cached tools found for this server. Go to MCP Manager to sync.
                                            </div>
                                            <div v-else class="ui list">
                                                <div v-for="tool in srv.tools" :key="tool.name" class="item" style="padding: 4px 0;">
                                                    <div class="ui checkbox tiny">
                                                        <input type="checkbox" 
                                                               :id="'tool-' + srv.id + '-' + tool.name"
                                                               :checked="!isToolDisabled(srv, tool.name)"
                                                               @change="toggleToolForBot(srv, tool.name)">
                                                        <label :for="'tool-' + srv.id + '-' + tool.name" style="cursor:pointer">
                                                            <strong>{{ tool.name }}</strong>
                                                            <div style="font-size:0.85em; opacity:0.7">{{ tool.description }}</div>
                                                        </label>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
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
    `,
};
