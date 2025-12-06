import { showSuccessInfo, showErrorInfo, handleApiError } from './utils.js';

export default {
    name: 'GlobalIASettings',
    data() {
        return {
            globalGeminiPrompt: '',
            globalGeminiTimezone: '',
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
                const { data } = await window.http.get('/settings/gemini');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
            } catch (err) {
                handleApiError(err, 'Failed to load global IA prompt');
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
                showSuccessInfo('Global IA prompt updated.');
                const { data } = await window.http.get('/settings/gemini');
                const results = data?.results || {};
                this.globalGeminiPrompt = results.global_system_prompt || '';
                this.globalGeminiTimezone = results.timezone || '';
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
                    <button type="button" class="ui button" :class="{ loading: savingGlobalGeminiPrompt }" @click="saveGlobalGeminiPrompt" :disabled="savingGlobalGeminiPrompt">
                        Save global settings
                    </button>
                </div>
            </div>
        </div>
    `,
};
