import { showSuccessInfo, handleApiError } from './utils.js';

export default {
    name: 'GeminiEditor',
    props: {
        instanceId: {
            type: [String, Number],
            default: null,
        },
        instance: {
            type: Object,
            default: () => ({}),
        },
        bots: {
            type: Array,
            default: () => [],
        },
    },
    data() {
        return {
            geminiEnabledInput: false,
            geminiAPIKeyInput: '',
            geminiModelInput: '',
            geminiSystemPromptInput: '',
            geminiKnowledgeBaseInput: '',
            geminiTimezoneInput: '',
            geminiAudioEnabledInput: false,
            geminiImageEnabledInput: false,
            geminiMemoryEnabledInput: false,
            instanceBotIdInput: '',
            savingGemini: false,
            clearingGeminiMemory: false,
        };
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
            this.geminiEnabledInput = !!this.instance.gemini_enabled;
            this.geminiAPIKeyInput = this.instance.gemini_api_key || '';
            this.geminiModelInput = this.instance.gemini_model || '';
            this.geminiSystemPromptInput = this.instance.gemini_system_prompt || '';
            this.geminiKnowledgeBaseInput = this.instance.gemini_knowledge_base || '';
            this.geminiTimezoneInput = this.instance.gemini_timezone || '';
            this.geminiAudioEnabledInput = !!this.instance.gemini_audio_enabled;
            this.geminiImageEnabledInput = !!this.instance.gemini_image_enabled;
            this.geminiMemoryEnabledInput = !!this.instance.gemini_memory_enabled;
            this.instanceBotIdInput = this.instance.bot_id || '';
        },
        cancel() {
            this.$emit('cancel');
        },
        async save() {
            if (!this.instanceId) return;
            try {
                this.savingGemini = true;
                await window.http.put(`/instances/${this.instanceId}/gemini`, {
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
                await window.http.put(`/instances/${this.instanceId}/bot`, {
                    bot_id: this.instanceBotIdInput || '',
                });
                this.$emit('refresh-instances');
                showSuccessInfo('Gemini configuration updated for instance.');
                this.cancel();
            } catch (err) {
                handleApiError(err, 'Failed to update Gemini configuration');
            } finally {
                this.savingGemini = false;
            }
        },
        async clearMemory() {
            if (!this.instanceId || this.clearingGeminiMemory) return;
            try {
                this.clearingGeminiMemory = true;
                await window.http.post(`/instances/${this.instanceId}/gemini/memory/clear`, {});
                showSuccessInfo('IA memory cleared for this instance.');
            } catch (err) {
                handleApiError(err, 'Failed to clear IA memory');
            } finally {
                this.clearingGeminiMemory = false;
            }
        },
    },
    template: `
        <div v-if="instanceId" class="ui segment" style="margin-top: 1em;">
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
                            <button type="button" class="ui button" :class="{ loading: clearingGeminiMemory }" @click="clearMemory" :disabled="clearingGeminiMemory">
                                Clear IA memory for this instance
                            </button>
                        </div>
                    </div>
                </div>
                <div class="ui buttons">
                    <button type="button" class="ui primary button" :class="{ loading: savingGemini }" @click="save" :disabled="savingGemini">
                        Save
                    </button>
                    <div class="or"></div>
                    <button type="button" class="ui button" @click="cancel" :disabled="savingGemini">
                        Cancel
                    </button>
                </div>
            </div>
        </div>
    `,
};
