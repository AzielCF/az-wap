export default {
    name: 'ChannelManager',
    props: {
        workspaceId: {
            type: String,
            required: true
        }
    },
    emits: ['close'],
    data() {
        return {
            channels: [],
            loading: false,
            newChannel: {
                name: '',
                type: 'whatsapp'
            },
            creating: false
        }
    },
    methods: {
        async loadChannels() {
            this.loading = true;
            try {
                const { data } = await window.http.get(`/workspaces/${this.workspaceId}/channels`);
                this.channels = data || [];
            } catch (err) {
                window.showErrorInfo('Failed to load channels: ' + err.message);
            } finally {
                this.loading = false;
            }
        },
        async createChannel() {
            if (!this.newChannel.name) return;
            this.creating = true;
            try {
                await window.http.post(`/workspaces/${this.workspaceId}/channels`, this.newChannel);
                window.showSuccessInfo('Channel created');
                this.newChannel.name = '';
                await this.loadChannels();
            } catch (err) {
                window.showErrorInfo(err.response?.data?.error || err.message);
            } finally {
                this.creating = false;
            }
        },
        async toggleChannel(channel) {
            const action = channel.enabled ? 'disable' : 'enable';
            try {
                await window.http.post(`/workspaces/${this.workspaceId}/channels/${channel.id}/${action}`);
                channel.enabled = !channel.enabled;
                window.showSuccessInfo(`Channel ${action}d`);
            } catch (err) {
                window.showErrorInfo(`Failed to ${action} channel: ` + err.message);
            }
        },
        async deleteChannel(channel) {
            if (!confirm('Are you sure you want to delete this channel?')) return;
            try {
                await window.http.delete(`/workspaces/${this.workspaceId}/channels/${channel.id}`);
                window.showSuccessInfo('Channel deleted');
                await this.loadChannels();
            } catch (err) {
                window.showErrorInfo('Failed to delete channel: ' + err.message);
            }
        },
        close() {
            this.$emit('close');
        }
    },
    template: `
    <div class="ui modal" id="modalChannelManager">
        <div class="header">Manage Channels</div>
        <div class="content">
            <!-- Create New -->
            <div class="ui form segment">
                <div class="inline fields">
                    <div class="field">
                        <label>Type</label>
                        <select v-model="newChannel.type" class="ui dropdown">
                            <option value="whatsapp">WhatsApp</option>
                            <option value="telegram" disabled>Telegram (Soon)</option>
                        </select>
                    </div>
                    <div class="eight wide field">
                        <label>Name</label>
                        <input type="text" v-model="newChannel.name" placeholder="Support Line">
                    </div>
                    <div class="field">
                        <button class="ui primary button" :class="{loading: creating}" @click="createChannel">
                            <i class="plus icon"></i> Add
                        </button>
                    </div>
                </div>
            </div>

            <!-- List -->
            <div class="ui segment" :class="{loading: loading}">
                <div class="ui relaxed divided list">
                    <div class="item" v-for="ch in channels" :key="ch.id">
                        <div class="right floated content">
                            <button class="ui icon button" :class="ch.enabled ? 'orange' : 'green'" @click="toggleChannel(ch)" :title="ch.enabled ? 'Disable' : 'Enable'">
                                <i :class="ch.enabled ? 'pause icon' : 'play icon'"></i>
                            </button>
                            <button class="ui icon red button" @click="deleteChannel(ch)">
                                <i class="trash icon"></i>
                            </button>
                        </div>
                        <i class="large middle aligned icon" :class="ch.type === 'whatsapp' ? 'whatsapp green' : 'globe'"></i>
                        <div class="content">
                            <div class="header">[[ ch.name ]]</div>
                            <div class="description">
                                Status: <b>[[ ch.status ]]</b> | ID: [[ ch.channel_id || ch.id ]]
                                <div class="ui label mini" v-if="ch.config && ch.config.settings && ch.config.settings.instance_id">
                                    Linked: [[ ch.config.settings.instance_id ]]
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="item" v-if="channels.length === 0">
                        <div class="content">No channels found.</div>
                    </div>
                </div>
            </div>
        </div>
        <div class="actions">
            <div class="ui button" @click="close">Close</div>
        </div>
    </div>
    `,
    mounted() {
        $('#modalChannelManager').modal({
            closable: false,
            onHidden: () => {
                this.$emit('close');
            }
        }).modal('show');
        this.loadChannels();
    },
    unmounted() {
        $('#modalChannelManager').modal('hide');
    }
}
