import GlobalIASettings from './GlobalIASettings.js';
import CredentialManager from './CredentialManager.js';
import NewInstanceForm from './NewInstanceForm.js';
import BotManager from './BotManager.js';
import InstanceList from './InstanceList.js';
import WebhookEditor from './WebhookEditor.js';
import ChatwootEditor from './ChatwootEditor.js';
import GeminiEditor from './GeminiEditor.js';
import MCPManager from './MCPManager.js';

export default {
    name: 'InstanceManager',
    components: {
        GlobalIASettings,
        CredentialManager,
        NewInstanceForm,
        BotManager,
        InstanceList,
        WebhookEditor,
        ChatwootEditor,
        GeminiEditor,
        MCPManager,
    },
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
            // Estado de editores
            editingInstanceId: null,
            chatwootEditingInstanceId: null,
            geminiEditingInstanceId: null,
            
            // Datos compartidos centralizados
            credentials: [],
            bots: [],
            mcpServers: [],
            healthStatus: {},
            loadingData: false,
            pollTimer: null,
        };
    },
    created() {
        this.handleDiscovery();
        // Arrancamos el polling para reactividad automática (cada 10 seg)
        this.pollTimer = setInterval(() => {
            this.handleDiscovery();
        }, 10000);
    },
    beforeUnmount() {
        if (this.pollTimer) {
            clearInterval(this.pollTimer);
        }
    },
    computed: {
        currentEditingInstance() {
            if (this.chatwootEditingInstanceId) {
                return this.instances.find(i => i.id === this.chatwootEditingInstanceId) || {};
            }
            if (this.geminiEditingInstanceId) {
                return this.instances.find(i => i.id === this.geminiEditingInstanceId) || {};
            }
            if (this.editingInstanceId) {
                return this.instances.find(i => i.id === this.editingInstanceId) || {};
            }
            return {};
        },
        webhookUrls() {
            return this.currentEditingInstance.webhook_urls || [];
        },
        webhookSecret() {
            return this.currentEditingInstance.webhook_secret || '';
        },
        webhookInsecure() {
            return this.currentEditingInstance.webhook_insecure_skip_verify || false;
        },
    },
    methods: {
        handleSetActiveToken(token) {
            this.$emit('set-active-token', token);
        },
        handleRefreshInstances() {
            this.$emit('refresh-instances');
        },
        async handleDiscovery() {
            if (this.loadingData) return;
            this.loadingData = true;
            try {
                const [credRes, botRes, mcpRes, healthRes] = await Promise.all([
                    window.http.get('/credentials'),
                    window.http.get('/bots'),
                    window.http.get('/api/mcp/servers'),
                    window.http.get('/api/health/status')
                ]);

                this.credentials = credRes.data?.results || [];
                this.bots = botRes.data?.results || [];
                this.mcpServers = mcpRes.data?.results || [];
                
                const statusMap = {};
                (healthRes.data?.results || []).forEach(r => {
                    statusMap[`${r.entity_type}:${r.entity_id}`] = r;
                });
                this.healthStatus = statusMap;
            } catch (err) {
                console.error('Discovery failed:', err);
            } finally {
                this.loadingData = false;
            }
        },
        handleCredentialsUpdated() {
            this.handleDiscovery();
        },
        handleBotsUpdated() {
            this.handleDiscovery();
        },
        handleMCPServersUpdated() {
            this.handleDiscovery();
        },
        openWebhookEditor(inst) {
            if (!inst || !inst.id) return;
            // Solo un panel de configuración de instancia visible a la vez
            this.chatwootEditingInstanceId = null;
            this.geminiEditingInstanceId = null;
            this.editingInstanceId = inst.id;
        },
        cancelWebhookEditor() {
            this.editingInstanceId = null;
        },
        openChatwootEditor(inst) {
            if (!inst || !inst.id) return;
            // Solo un panel de configuración de instancia visible a la vez
            this.editingInstanceId = null;
            this.geminiEditingInstanceId = null;
            this.chatwootEditingInstanceId = inst.id;
        },
        cancelChatwootEditor() {
            this.chatwootEditingInstanceId = null;
        },
        openGeminiEditor(inst) {
            if (!inst || !inst.id) return;
            // Solo un panel de configuración de instancia visible a la vez
            this.editingInstanceId = null;
            this.chatwootEditingInstanceId = null;
            this.geminiEditingInstanceId = inst.id;
        },
        cancelGeminiEditor() {
            this.geminiEditingInstanceId = null;
        },
    },
    template: `
    <div class="instance-manager">
        <div class="green card" style="cursor: default; width: 100%;">
            <div class="content">
                <a class="ui teal right ribbon label">Instance Manager</a>
                <div class="description">
                    <div class="ui form">
                        <!-- Configuración global de IA -->
                        <GlobalIASettings />

                        <!-- Gestión de credenciales -->
                        <CredentialManager 
                            :credentials="credentials"
                            :health-status="healthStatus"
                            @credentials-updated="handleCredentialsUpdated"
                        />

                        <!-- Formulario de nueva instancia -->
                        <NewInstanceForm 
                            @refresh-instances="handleRefreshInstances"
                            @set-active-token="handleSetActiveToken"
                        />

                        <!-- Gestión de Bots AI -->
                        <BotManager 
                            :bots="bots"
                            :credentials="credentials"
                            :health-status="healthStatus"
                            @bots-updated="handleBotsUpdated"
                        />

                        <!-- Gestión de MCP -->
                        <MCPManager 
                            :servers="mcpServers"
                            :health-status="healthStatus"
                            @mcp-servers-updated="handleMCPServersUpdated"
                        />
                    </div>

                    <!-- Lista de instancias existentes -->
                    <InstanceList 
                        :instances="instances"
                        :selectedToken="selectedToken"
                        :health-status="healthStatus"
                        @set-active-token="handleSetActiveToken"
                        @refresh-instances="handleRefreshInstances"
                        @open-webhook-editor="openWebhookEditor"
                        @open-chatwoot-editor="openChatwootEditor"
                        @open-gemini-editor="openGeminiEditor"
                    />

                    <!-- Editor de Webhooks -->
                    <WebhookEditor 
                        :instanceId="editingInstanceId"
                        :webhookUrls="webhookUrls"
                        :webhookSecret="webhookSecret"
                        :webhookInsecure="webhookInsecure"
                        @cancel="cancelWebhookEditor"
                        @refresh-instances="handleRefreshInstances"
                    />

                    <!-- Editor de Gemini/IA -->
                    <GeminiEditor 
                        :instanceId="geminiEditingInstanceId"
                        :instance="currentEditingInstance"
                        :bots="bots"
                        @cancel="cancelGeminiEditor"
                        @refresh-instances="handleRefreshInstances"
                    />
                </div>

                <!-- Modal de Chatwoot -->
                <ChatwootEditor 
                    :instanceId="chatwootEditingInstanceId"
                    :instance="currentEditingInstance"
                    :credentials="credentials"
                    @cancel="cancelChatwootEditor"
                />
            </div>
        </div>
    </div>
    `,
};
