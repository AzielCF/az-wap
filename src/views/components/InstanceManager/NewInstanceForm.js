import { showSuccessInfo, handleApiError } from './utils.js';

export default {
    name: 'NewInstanceForm',
    data() {
        return {
            newName: '',
            creating: false,
            lastToken: null,
            showNewInstanceSection: false,
        };
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
                    this.$emit('set-active-token', this.lastToken);
                    showSuccessInfo('Instance created. Token set as active for this UI session.');
                }
            } catch (err) {
                handleApiError(err, 'Failed to create instance');
            } finally {
                this.creating = false;
            }
        },
    },
    template: `
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
                    <div class="ui form">
                        <div class="field">
                            <label>Instance name</label>
                            <input type="text" v-model="newName" placeholder="e.g. my-bot-ventas" :disabled="creating"
                                   @keyup.enter="createInstance">
                        </div>
                        <div class="field" style="padding-top: 23px;">
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
    `,
};
