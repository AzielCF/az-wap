import { showSuccessInfo, showErrorInfo, handleApiError } from './utils.js';

export default {
    name: 'GlobalIASettings',
    data() {
        return {
            globalGeminiPrompt: '',
            globalGeminiTimezone: '',
            aiDebounceMs: 3000,
            aiWaitContactIdleMs: 10000,
            aiTypingEnabled: true,
            loadingGlobalGeminiPrompt: false,
            savingGlobalGeminiPrompt: false,
            showGlobalIASettings: false,
        };
    },
    created() {
        this.loadGlobalGeminiPrompt();
    },
    methods: {
        async loadGlobalGeminiPrompt() {
            try {
                this.loadingGlobalGeminiPrompt = true;
                const { data } = await window.http.get('/settings/ai');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
                this.aiDebounceMs = typeof results.debounce_ms === 'number' ? results.debounce_ms : 0;
                this.aiWaitContactIdleMs = typeof results.wait_contact_idle_ms === 'number' ? results.wait_contact_idle_ms : 0;
                this.aiTypingEnabled = typeof results.typing_enabled === 'boolean' ? results.typing_enabled : true;
            } catch (err) {
                handleApiError(err, 'Failed to load global IA prompt');
            } finally {
                this.loadingGlobalGeminiPrompt = false;
            }
        },
        async saveGlobalGeminiPrompt() {
            try {
                this.savingGlobalGeminiPrompt = true;
                await window.http.put('/settings/ai', {
                    global_system_prompt: this.globalGeminiPrompt || '',
                    timezone: this.globalGeminiTimezone || '',
                    debounce_ms: Number.isFinite(this.aiDebounceMs) ? this.aiDebounceMs : 0,
                    wait_contact_idle_ms: Number.isFinite(this.aiWaitContactIdleMs) ? this.aiWaitContactIdleMs : 0,
                    typing_enabled: !!this.aiTypingEnabled,
                });
                showSuccessInfo('Global IA prompt updated.');
                const { data } = await window.http.get('/settings/ai');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
                this.aiDebounceMs = typeof results.debounce_ms === 'number' ? results.debounce_ms : 0;
                this.aiWaitContactIdleMs = typeof results.wait_contact_idle_ms === 'number' ? results.wait_contact_idle_ms : 0;
                this.aiTypingEnabled = typeof results.typing_enabled === 'boolean' ? results.typing_enabled : true;
            } catch (err) {
                handleApiError(err, 'Failed to update global IA prompt');
            } finally {
                this.savingGlobalGeminiPrompt = false;
            }
        },
    },
    template: `
        <div class="field">
            <div class="ui segment">
                <div style="display: flex; align-items: center; justify-content: space-between; gap: 0.75rem;">
                    <div>
                        <h3 class="ui header" style="margin-bottom: 0;">
                            <i class="cog icon blue"></i>
                            <div class="content">
                                Global AI settings
                                <div class="sub header">Applies to all AI assistants in all instances</div>
                            </div>
                        </h3>
                    </div>
                    <button type="button" class="ui mini button" @click="showGlobalIASettings = !showGlobalIASettings">
                        {{ showGlobalIASettings ? 'Hide' : 'Show' }}
                    </button>
                </div>
                <div v-if="showGlobalIASettings" style="margin-top: 0.75rem;">
                    <label>Global AI system prompt</label>
                    <textarea rows="4" v-model="globalGeminiPrompt" placeholder="Global rules for all AI assistants"></textarea>
                    <div class="field" style="margin-top: 0.5rem;">
                        <label>AI timezone (IANA)</label>
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
                    <div class="field" style="margin-top: 0.5rem;">
                        <label>AI reply debounce (ms)</label>
                        <input type="number" min="0" step="100" v-model.number="aiDebounceMs" placeholder="0" />
                    </div>
                    <div class="field" style="margin-top: 0.5rem;">
                        <label>Wait contact idle before AI reply (ms)</label>
                        <input type="number" min="0" step="250" v-model.number="aiWaitContactIdleMs" placeholder="0" />
                    </div>
                    <div class="field" style="margin-top: 0.5rem;">
                        <div class="ui checkbox">
                            <input type="checkbox" v-model="aiTypingEnabled" />
                            <label>Simulate human typing before AI reply</label>
                        </div>
                    </div>
                    <button type="button" class="ui button" :class="{ loading: savingGlobalGeminiPrompt }" @click="saveGlobalGeminiPrompt" :disabled="savingGlobalGeminiPrompt">
                        Save global settings
                    </button>
                </div>
            </div>
        </div>
    `,
};
