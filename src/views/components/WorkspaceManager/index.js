export default {
    name: 'WorkspaceManager',
    delimiters: ['[[', ']]'],
    data() {
        return {
            workspaces: [],
            loading: false,
            
            // Workspace Editor State
            selectedWorkspace: null,
            workspaceForm: {
                name: '',
                description: '',
                owner_id: ''
            },
            workspaceLoading: false,
            workspaceError: null,

            // Channel Manager State
            currentWorkspaceId: '',
            channels: [],
            bots: [],
            channelsLoading: false,
            newChannel: {
                name: '',
                type: 'whatsapp'
            },
            creatingChannel: false,

            // Channel Config State
            selectedChannel: null,
            configForm: {
                webhook_url: '',
                webhook_secret: '',
                bot_id: '',
                chatwoot: {
                    enabled: false,
                    account_id: 0,
                    inbox_id: 0,
                    token: '',
                    url: '',
                    bot_token: '',
                    inbox_identifier: '',
                    credential_id: '',
                    webhook_url: ''
                },
                skip_tls_verification: false,
                auto_reconnect: true
            },
            configLoading: false,
            credentials: [],

            // WhatsApp Control State
            waStatus: {
                loading: false,
                connected: false,
                loggedIn: false,
                qr: null
            }
        }
    },
    methods: {
        // --- WORKSPACE METHODS ---
        async loadWorkspaces() {
            this.loading = true;
            try {
                const { data } = await window.http.get('/workspaces');
                this.workspaces = data || [];
            } catch (err) {
                console.error(err);
                window.showErrorInfo('Failed to load workspaces');
            } finally {
                this.loading = false;
            }
        },
        openWorkspaceModal(ws = null) {
            this.selectedWorkspace = ws;
            if (ws) {
                this.workspaceForm = {
                    name: ws.name,
                    description: ws.description || '',
                    owner_id: ws.owner_id || ''
                };
            } else {
                this.workspaceForm = { name: '', description: '', owner_id: '' };
            }
            this.workspaceError = null;
            $('#modalWorkspaceEditor').modal('show');
        },
        async saveWorkspace() {
            this.workspaceLoading = true;
            this.workspaceError = null;
            try {
                if (this.selectedWorkspace) {
                    await window.http.put(`/workspaces/${this.selectedWorkspace.id}`, this.workspaceForm);
                } else {
                    await window.http.post('/workspaces', this.workspaceForm);
                }
                window.showSuccessInfo('Workspace saved');
                $('#modalWorkspaceEditor').modal('hide');
                this.loadWorkspaces();
            } catch (err) {
                this.workspaceError = err.response?.data?.error || err.message;
            } finally {
                this.workspaceLoading = false;
            }
        },
        async deleteWorkspace(ws) {
            if (!confirm(`Are you sure you want to delete workspace "${ws.name}"?`)) return;
            try {
                await window.http.delete(`/workspaces/${ws.id}`);
                window.showSuccessInfo('Workspace deleted');
                this.loadWorkspaces();
            } catch (err) {
                window.showErrorInfo('Failed to delete workspace: ' + err.message);
            }
        },

        // --- CHANNEL METHODS ---
        async openChannelManager(ws) {
            this.currentWorkspaceId = ws.id;
            this.channels = [];
            this.loadChannels();
            this.loadBots();
            this.loadCredentials();
            $('#modalChannelManager').modal('show');
        },
        async loadCredentials() {
            try {
                const { data } = await window.http.get('/credentials');
                this.credentials = data?.results || [];
            } catch (err) {}
        },
        async loadChannels() {
            this.channelsLoading = true;
            try {
                const { data } = await window.http.get(`/workspaces/${this.currentWorkspaceId}/channels`);
                this.channels = data || [];
            } catch (err) {
                window.showErrorInfo('Failed to load channels');
            } finally {
                this.channelsLoading = false;
            }
        },
        async loadBots() {
            try {
                const { data } = await window.http.get('/bots');
                this.bots = data?.results || [];
            } catch (err) {}
        },
        async createChannel() {
            if (!this.newChannel.name) return;
            this.creatingChannel = true;
            try {
                await window.http.post(`/workspaces/${this.currentWorkspaceId}/channels`, this.newChannel);
                this.newChannel.name = '';
                this.loadChannels();
            } catch (err) {
                window.showErrorInfo(err.response?.data?.error || err.message);
            } finally {
                this.creatingChannel = false;
            }
        },
        async toggleChannel(ch) {
            const action = ch.enabled ? 'disable' : 'enable';
            try {
                await window.http.post(`/workspaces/${this.currentWorkspaceId}/channels/${ch.id}/${action}`);
                ch.enabled = !ch.enabled;
            } catch (err) {
                window.showErrorInfo(`Failed to ${action} channel`);
            }
        },
        async deleteChannel(ch) {
            if (!confirm('Delete this channel?')) return;
            try {
                await window.http.delete(`/workspaces/${this.currentWorkspaceId}/channels/${ch.id}`);
                this.loadChannels();
            } catch (err) {
                window.showErrorInfo('Failed to delete channel');
            }
        },

        // --- CHANNEL CONFIG METHODS ---
        openChannelConfig(ch) {
            this.selectedChannel = ch;
            if (ch.config) {
                this.configForm = JSON.parse(JSON.stringify(ch.config));
                if (!this.configForm.chatwoot) {
                    this.configForm.chatwoot = { 
                        enabled: false, 
                        account_id: 0, 
                        inbox_id: 0, 
                        token: '', 
                        url: '',
                        bot_token: '',
                        inbox_identifier: '',
                        credential_id: '',
                        webhook_url: ''
                    };
                }
            } else {
                this.configForm = {
                    webhook_url: '',
                    webhook_secret: '',
                    bot_id: '',
                    skip_tls_verification: false,
                    auto_reconnect: true,
                    chatwoot: { 
                        enabled: false, 
                        account_id: 0, 
                        inbox_id: 0, 
                        token: '', 
                        url: '',
                        bot_token: '',
                        inbox_identifier: '',
                        credential_id: '',
                        webhook_url: ''
                    }
                };
            }
            
            // Generate read-only webhook URL
            if (this.configForm.chatwoot) {
                const apiBase = window.http.defaults.baseURL || window.location.origin;
                this.configForm.chatwoot.webhook_url = `${apiBase}/workspaces/${this.currentWorkspaceId}/channels/${ch.id}/chatwoot/webhook`;
            }
            $('#modalChannelConfig').modal('show');
        },
        async saveChannelConfig() {
            this.configLoading = true;
            try {
                const configToSave = JSON.parse(JSON.stringify(this.configForm));
                if (configToSave.chatwoot) delete configToSave.chatwoot.webhook_url;

                await window.http.put(`/workspaces/${this.currentWorkspaceId}/channels/${this.selectedChannel.id}/config`, configToSave);
                window.showSuccessInfo('Configuration saved');
                $('#modalChannelConfig').modal('hide');
                this.loadChannels();
            } catch (err) {
                window.showErrorInfo('Failed to save configuration');
            } finally {
                this.configLoading = false;
            }
        },
        copyToClipboard(text) {
            if (!text) return;
            navigator.clipboard.writeText(text).then(() => {
                window.showSuccessInfo('URL copied to clipboard');
            }).catch(err => {
                window.showErrorInfo('Failed to copy URL');
            });
        },

        // --- WHATSAPP CONTROL METHODS ---
        async openWhatsAppControl(ch) {
            this.selectedChannel = ch;
            this.waStatus = { loading: true, qr: null, connected: false, loggedIn: false };
            $('#modalWhatsAppControl').modal('show');
            await this.loadWhatsAppStatus();
        },
        async loadWhatsAppStatus() {
            try {
                const { data } = await window.http.get(`/workspaces/${this.currentWorkspaceId}/channels/${this.selectedChannel.id}/whatsapp/status`);
                this.waStatus.connected = data.is_connected;
                this.waStatus.loggedIn = data.is_logged_in;
                this.waStatus.loading = false;
            } catch (err) {
                console.error(err);
            }
        },
        async whatsappLogin() {
            this.waStatus.loading = true;
            this.waStatus.qr = null;
            try {
                const { data } = await window.http.get(`/workspaces/${this.currentWorkspaceId}/channels/${this.selectedChannel.id}/whatsapp/login`);
                if (data.results && data.results.qr_link) {
                    this.waStatus.qr = data.results.qr_link;
                }
            } catch (err) {
                window.showErrorInfo(err.response?.data?.error || 'Failed to get QR');
            } finally {
                this.waStatus.loading = false;
            }
        },
        async whatsappLogout() {
            if (!confirm('Logout from WhatsApp?')) return;
            try {
                await window.http.get(`/workspaces/${this.currentWorkspaceId}/channels/${this.selectedChannel.id}/whatsapp/logout`);
                window.showSuccessInfo('Logged out');
                this.loadWhatsAppStatus();
            } catch (err) {
                window.showErrorInfo('Failed to logout');
            }
        },
        async whatsappReconnect() {
            try {
                await window.http.get(`/workspaces/${this.currentWorkspaceId}/channels/${this.selectedChannel.id}/whatsapp/reconnect`);
                window.showSuccessInfo('Reconnect signal sent');
                setTimeout(() => this.loadWhatsAppStatus(), 2000);
            } catch (err) {
                window.showErrorInfo('Failed to reconnect');
            }
        }
    },
    mounted() {
        this.loadWorkspaces();
    },
    template: `
    <div class="ui card fluid" id="workspace-manager-card">
        <div class="content">
            <div class="right floated meta">
                <button class="ui primary mini button" @click="openWorkspaceModal()">
                    <i class="plus icon"></i> New Workspace
                </button>
                <button class="ui icon mini button" @click="loadWorkspaces" :class="{loading: loading}">
                    <i class="sync icon"></i>
                </button>
            </div>
            <div class="header">Workspaces</div>
            <div class="meta">Manage multi-tenant environments</div>
            
            <div class="description" style="margin-top: 1em;">
                <table class="ui celled table">
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Description</th>
                            <th>Owner</th>
                            <th>Status</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="ws in workspaces" :key="ws.id">
                            <td>
                                <i class="building icon"></i> <b>[[ ws.name ]]</b>
                                <div class="ui label mini">[[ ws.id ]]</div>
                            </td>
                            <td>[[ ws.description ]]</td>
                            <td>[[ ws.owner_id ]]</td>
                            <td>
                                <div class="ui label green" v-if="ws.enabled">Active</div>
                                <div class="ui label red" v-else>Disabled</div>
                            </td>
                            <td>
                                <div class="ui icon buttons mini">
                                    <button class="ui button" @click="openWorkspaceModal(ws)" title="Edit">
                                        <i class="edit icon"></i>
                                    </button>
                                    <button class="ui teal button" @click="openChannelManager(ws)" title="Manage Channels">
                                        <i class="sitemap icon"></i>
                                    </button>
                                    <button class="ui red button" @click="deleteWorkspace(ws)" title="Delete">
                                        <i class="trash icon"></i>
                                    </button>
                                </div>
                            </td>
                        </tr>
                        <tr v-if="workspaces.length === 0">
                            <td colspan="5" class="center aligned">No workspaces found.</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- Modal: Workspace Editor -->
    <div class="ui modal" id="modalWorkspaceEditor">
        <i class="close icon"></i>
        <div class="header">[[ selectedWorkspace ? 'Edit Workspace' : 'New Workspace' ]]</div>
        <div class="content">
            <div class="ui form" :class="{loading: workspaceLoading}">
                <div class="field">
                    <label>Name</label>
                    <input type="text" v-model="workspaceForm.name" placeholder="My Workspace">
                </div>
                <div class="field">
                    <label>Description</label>
                    <input type="text" v-model="workspaceForm.description" placeholder="Optional description">
                </div>
                <div class="field">
                    <label>Owner ID</label>
                    <input type="text" v-model="workspaceForm.owner_id" placeholder="Owner User ID">
                </div>
                <div class="ui error message" v-if="workspaceError" style="display:block">[[ workspaceError ]]</div>
            </div>
        </div>
        <div class="actions">
            <div class="ui black deny button">Cancel</div>
            <div class="ui positive right labeled icon button" @click="saveWorkspace">
                Save
                <i class="checkmark icon"></i>
            </div>
        </div>
    </div>

    <!-- Modal: Channel Manager -->
    <div class="ui scrolling modal" id="modalChannelManager">
        <i class="close icon"></i>
        <div class="header">Manage Channels</div>
        <div class="scrolling content">
            <div class="ui form segment">
                <div class="inline fields">
                    <div class="field">
                        <label>Type</label>
                        <select v-model="newChannel.type" class="ui dropdown">
                            <option value="whatsapp">WhatsApp</option>
                        </select>
                    </div>
                    <div class="field">
                        <label>Name</label>
                        <input type="text" v-model="newChannel.name" placeholder="Channel Name">
                    </div>
                    <div class="field">
                        <button class="ui primary button" :class="{loading: creatingChannel}" @click="createChannel">
                            <i class="plus icon"></i> Add
                        </button>
                    </div>
                </div>
            </div>

            <div class="ui segment" :class="{loading: channelsLoading}">
                <div class="ui relaxed divided list">
                    <div class="item" v-for="ch in channels" :key="ch.id">
                        <div class="right floated content">
                            <div class="ui icon buttons mini">
                                <button class="ui button" @click="openChannelConfig(ch)" title="Settings">
                                    <i class="cog icon"></i>
                                </button>
                                <button class="ui icon green button" v-if="ch.type === 'whatsapp'" @click="openWhatsAppControl(ch)" title="WhatsApp Control">
                                    <i class="whatsapp icon"></i>
                                </button>
                                <button class="ui icon button" :class="ch.enabled ? 'orange' : 'green'" @click="toggleChannel(ch)">
                                    <i :class="ch.enabled ? 'pause icon' : 'play icon'"></i>
                                </button>
                                <button class="ui icon red button" @click="deleteChannel(ch)">
                                    <i class="trash icon"></i>
                                </button>
                            </div>
                        </div>
                        <i class="large middle aligned whatsapp green icon"></i>
                        <div class="content">
                            <div class="header">[[ ch.name ]]</div>
                            <div class="description">
                                Status: <b>[[ ch.status ]]</b> | ID: [[ ch.channel_id || ch.id ]]
                                <div class="ui label mini" v-if="ch.config && ch.config.bot_id">
                                    Bot: [[ ch.config.bot_id ]]
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="item" v-if="channels.length === 0">No channels found.</div>
                </div>
            </div>
        </div>
    </div>

    <!-- Modal: Channel Config -->
    <div class="ui scrolling modal" id="modalChannelConfig">
        <i class="close icon"></i>
        <div class="header">Configure Channel: [[ selectedChannel ? selectedChannel.name : '' ]]</div>
        <div class="scrolling content">
            <div class="ui form" :class="{loading: configLoading}">
                <h4 class="ui dividing header">Bot Assignment</h4>
                <div class="two fields">
                    <div class="field">
                        <label>Assigned Bot</label>
                        <select v-model="configForm.bot_id" class="ui dropdown">
                            <option value="">None</option>
                            <option v-for="bot in bots" :key="bot.id" :value="bot.id">[[ bot.name ]] ([[ bot.id ]])</option>
                        </select>
                    </div>
                    <div class="field" style="padding-top: 25px;">
                        <div class="ui toggle checkbox">
                            <input type="checkbox" v-model="configForm.skip_tls_verification">
                            <label>Skip TLS Verification</label>
                        </div>
                        <div class="ui toggle checkbox" style="margin-left: 15px;">
                            <input type="checkbox" v-model="configForm.auto_reconnect">
                            <label>Auto Reconnect</label>
                        </div>
                    </div>
                </div>

                <h4 class="ui dividing header">Webhook Configuration</h4>
                <div class="field">
                    <label>Webhook URL</label>
                    <input type="text" v-model="configForm.webhook_url">
                </div>
                <div class="field">
                    <label>Webhook Secret</label>
                    <input type="password" v-model="configForm.webhook_secret">
                </div>

                <h4 class="ui dividing header">Chatwoot Integration</h4>
                <div class="field">
                    <div class="ui toggle checkbox">
                        <input type="checkbox" v-model="configForm.chatwoot.enabled">
                        <label>Enable Chatwoot</label>
                    </div>
                </div>
                <template v-if="configForm.chatwoot.enabled">
                    <div class="field">
                        <label>Chatwoot Credential (Optional)</label>
                        <select v-model="configForm.chatwoot.credential_id" class="ui dropdown">
                            <option value="">(Use direct base URL and account token below)</option>
                            <option v-for="cred in credentials" :key="cred.id" :value="cred.id">
                                [[ cred.name ]] - [[ cred.chatwoot_base_url || 'no base URL' ]]
                            </option>
                        </select>
                    </div>
                    <div class="two fields" v-if="!configForm.chatwoot.credential_id">
                        <div class="field"><label>API Token</label><input type="text" v-model="configForm.chatwoot.token"></div>
                        <div class="field"><label>Chatwoot URL</label><input type="text" v-model="configForm.chatwoot.url"></div>
                    </div>
                    <div class="two fields">
                        <div class="field"><label>Account ID</label><input type="number" v-model.number="configForm.chatwoot.account_id"></div>
                        <div class="field"><label>Inbox ID</label><input type="number" v-model.number="configForm.chatwoot.inbox_id"></div>
                    </div>
                    <div class="two fields">
                        <div class="field"><label>Bot Token</label><input type="text" v-model="configForm.chatwoot.bot_token"></div>
                        <div class="field"><label>Inbox Identifier</label><input type="text" v-model="configForm.chatwoot.inbox_identifier"></div>
                    </div>
                    <div class="field">
                        <label>Chatwoot Webhook URL (Read-only)</label>
                        <div class="ui action input">
                            <input type="text" :value="configForm.chatwoot.webhook_url" readonly>
                            <button class="ui teal icon button" @click="copyToClipboard(configForm.chatwoot.webhook_url)">
                                <i class="copy icon"></i>
                            </button>
                        </div>
                        <small>Copy this URL and paste it in your Chatwoot Inbox settings (Webhook URL).</small>
                    </div>
                </template>
            </div>
        </div>
        <div class="actions">
            <div class="ui black deny button">Cancel</div>
            <div class="ui positive button" @click="saveChannelConfig">Save</div>
        </div>
    </div>

    <!-- Modal: WhatsApp Control -->
    <div class="ui modal" id="modalWhatsAppControl">
        <i class="close icon"></i>
        <div class="header">WhatsApp Control: [[ selectedChannel ? selectedChannel.name : '' ]]</div>
        <div class="content">
            <div class="ui segment" :class="{loading: waStatus.loading}">
                <div class="ui two column grid">
                    <div class="column center aligned borderless">
                        <h4>Status</h4>
                        <div class="ui statistic mini">
                            <div class="value">
                                <i class="circle icon" :class="waStatus.connected ? 'green' : 'red'"></i>
                            </div>
                            <div class="label">[[ waStatus.connected ? 'Connected' : 'Disconnected' ]]</div>
                        </div>
                        <div class="ui statistic mini" style="margin-left: 20px;">
                            <div class="value">
                                <i class="user icon" :class="waStatus.loggedIn ? 'green' : 'grey'"></i>
                            </div>
                            <div class="label">[[ waStatus.loggedIn ? 'Logged In' : 'Logged Out' ]]</div>
                        </div>
                    </div>
                    <div class="column center aligned borderless" style="border-left: 1px solid #ddd;">
                        <h4>Action</h4>
                        <button class="ui primary fluid button" v-if="!waStatus.loggedIn" @click="whatsappLogin">
                            <i class="qrcode icon"></i> Login (Show QR)
                        </button>
                        <button class="ui orange fluid button" v-if="waStatus.loggedIn" @click="whatsappLogout">
                            <i class="sign out icon"></i> Logout
                        </button>
                        <button class="ui grey fluid button" style="margin-top: 10px;" @click="whatsappReconnect">
                            <i class="sync icon"></i> Force Reconnect
                        </button>
                        <button class="ui icon mini button fluid" style="margin-top: 10px;" @click="loadWhatsAppStatus">
                            <i class="sync icon"></i> Refresh Status
                        </button>
                    </div>
                </div>

                <div v-if="waStatus.qr" class="ui center aligned segment" style="margin-top: 20px;">
                    <h4>Scan QR Code</h4>
                    <img :src="waStatus.qr" style="max-width: 250px; margin: 0 auto; display: block; background: #fff; padding: 10px; border: 1px solid #eee;">
                    <p style="margin-top: 10px;">The QR code will expire in a few minutes.</p>
                </div>
            </div>
        </div>
        <div class="actions">
            <div class="ui black deny button">Close</div>
        </div>
    </div>
    `
}
