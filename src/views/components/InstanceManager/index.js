import GlobalIASettings from './GlobalIASettings.js';
import CredentialManager from './CredentialManager.js';
import NewInstanceForm from './NewInstanceForm.js';
import BotManager from './BotManager.js';
import InstanceList from './InstanceList.js';
import WebhookEditor from './WebhookEditor.js';
import ChatwootEditor from './ChatwootEditor.js';
import GeminiEditor from './GeminiEditor.js';

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
            
            // Datos compartidos
            credentials: [],
            bots: [],
        };
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
        handleCredentialsLoaded(credentials) {
            this.credentials = credentials;
        },
        handleBotsLoaded(bots) {
            this.bots = bots;
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
                            @credentials-loaded="handleCredentialsLoaded"
                        />

                        <!-- Formulario de nueva instancia -->
                        <NewInstanceForm 
                            @refresh-instances="handleRefreshInstances"
                            @set-active-token="handleSetActiveToken"
                        />

                        <!-- Gestión de Bots AI -->
                        <BotManager 
                            :credentials="credentials"
                            @bots-loaded="handleBotsLoaded"
                        />
                    </div>

                    <!-- Lista de instancias existentes -->
                    <InstanceList 
                        :instances="instances"
                        :selectedToken="selectedToken"
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
