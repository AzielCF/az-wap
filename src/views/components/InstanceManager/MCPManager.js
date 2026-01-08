export default {
    name: 'MCPManager',
    props: {
        servers: {
            type: Array,
            default: () => [],
        },
        healthStatus: {
            type: Object,
            default: () => ({}),
        },
    },
    data() {
        return {
            loading: false,
            showAddModal: false,
            newServer: {
                name: '',
                description: '',
                type: 'sse',
                url: '',
                command: '',
                args: [],
                headers: {},
                is_template: false,
                template_config: {}, // Key: HelperText
            },
            templateHeaders: [], // Array of { key: '', help: '' } for UI editing
            editingServer: null,
            tools: [],
            loadingTools: false,
            selectedServerTools: null,
            argsString: '',
            headersString: '', // Now for custom headers
            checkingHealth: {}, // Map of entity_id -> boolean
        };
    },
    computed: {
        target() {
            return this.editingServer || this.newServer;
        }
    },
    mounted() {
        // Managed by parent
    },
    methods: {
        async checkServerHealth(srv) {
            this.checkingHealth[srv.id] = true;
            try {
                const { data } = await window.http.post(`/api/health/mcp/${srv.id}/check`);
                if (data.status === 200 || data.code === 'SUCCESS') {
                    this.$emit('mcp-servers-updated');
                    if (data.results.status === 'ERROR') {
                        window.showErrorInfo(`Health check failed: ${data.results.last_message}`);
                    } else {
                        window.showSuccessInfo('Server is healthy');
                    }
                }
            } catch (err) {
                window.showErrorInfo('Failed to perform health check');
            } finally {
                this.checkingHealth[srv.id] = false;
            }
        },
        getHealthLabel(srvId) {
            const h = this.healthStatus[`mcp_server:${srvId}`];
            if (!h) return { text: 'UNKNOWN', color: '' };
            return {
                text: h.status,
                color: h.status === 'OK' ? 'green' : (h.status === 'ERROR' ? 'red' : 'yellow'),
                message: h.last_message,
                lastChecked: h.last_checked
            };
        },
        async fetchServers() {
            this.$emit('mcp-servers-updated');
        },
        async saveServer() {
            if (!this.target.name) {
                window.showErrorInfo('Server name is required');
                return;
            }
            if (this.target.type === 'stdio' && !this.target.command) {
                window.showErrorInfo('Command is required for STDIO servers');
                return;
            }
            if ((this.target.type === 'sse' || this.target.type === 'http') && !this.target.url) {
                window.showErrorInfo('URL is required for this connection type');
                return;
            }

            try {
                const url = this.editingServer ? `/api/mcp/servers/${this.editingServer.id}` : '/api/mcp/servers';
                const method = this.editingServer ? 'put' : 'post';
                
                // Parse headers (Key: Value)
                const headers = {};
                (this.headersString || '').split('\n').forEach(line => {
                    const idx = line.indexOf(':');
                    if (idx > 0) {
                        const key = line.substring(0, idx).trim();
                        const val = line.substring(idx + 1).trim();
                        if (key) headers[key] = val;
                    }
                });
                this.target.headers = headers;

                if (this.target.type === 'stdio') {
                    this.target.args = (this.argsString || '').split('\n').map(s => s.trim()).filter(s => s !== '');
                    this.target.url = '';
                    this.target.command = '';
                }

                // Prepare template config
                const tplConfig = {};
                if (this.target.is_template) {
                    this.templateHeaders.forEach(h => {
                        if (h.key) tplConfig[h.key] = h.help || '';
                    });
                }
                this.target.template_config = tplConfig;

                const { data } = await window.http[method](url, this.target);

                if (data.status === 200 || data.status === 201 || data.code === 'SUCCESS') {
                    this.closeModal();
                    this.$emit('mcp-servers-updated');
                    window.showSuccessInfo('MCP Server saved successfully');
                } else {
                    window.showErrorInfo(data.message || 'Error saving server');
                }
            } catch (err) {
                window.showErrorInfo(err.response?.data?.message || err.message);
            }
        },
        resetForm() {
            this.newServer = { name: '', description: '', type: 'http', url: '', command: '', args: [], headers: {} };
            this.argsString = '';
            this.argsString = '';
            this.headersString = '';
            this.templateHeaders = [];
        },
        addTemplateHeader() {
            this.templateHeaders.push({ key: '', help: '' });
        },
        removeTemplateHeader(index) {
            this.templateHeaders.splice(index, 1);
        },
        openModal() {
            this.showAddModal = true;
            this.$nextTick(() => {
                $('#mcpServerModal').modal({
                    closable: true,
                    onHidden: () => {
                        this.showAddModal = false;
                        this.editingServer = null;
                        this.resetForm();
                    }
                }).modal('show');
            });
        },
        closeModal() {
            $('#mcpServerModal').modal('hide');
            this.showAddModal = false;
            this.editingServer = null;
            this.resetForm();
        },
        async deleteServer(id) {
            if (!confirm('Are you sure you want to delete this MCP server?')) return;
            try {
                const { data } = await window.http.delete(`/api/mcp/servers/${id}`);
                if (data.status === 200 || data.code === 'SUCCESS') {
                    this.$emit('mcp-servers-updated');
                    window.showSuccessInfo('Server deleted successfully');
                }
            } catch (err) {
                window.showErrorInfo('Error deleting server');
            }
        },
        async viewTools(server, forceRefresh = false) {
            this.selectedServerTools = server;
            this.loadingTools = true;
            this.tools = [];
            
            // Show modal if not already visible
            if (!forceRefresh) {
                this.$nextTick(() => {
                    $('#mcpToolsModal').modal({
                        closable: true,
                        onHidden: () => {
                            this.selectedServerTools = null;
                            this.tools = [];
                        }
                    }).modal('show');
                });
            }

            try {
                // If it's a force refresh, we could pass a query param or just rely on the backend 
                // but the backend already refreshes the cache when ListTools is called.
                const { data } = await window.http.get(`/api/mcp/servers/${server.id}/tools`);
                if (data.status === 200 || data.code === 'SUCCESS') {
                    this.tools = data.results || [];
                } else {
                    window.showErrorInfo(data.message || 'Error loading tools');
                }
            } catch (err) {
                window.showErrorInfo(err.response?.data?.message || 'Error loading tools');
            } finally {
                this.loadingTools = false;
            }
        },
        editServer(server) {
            this.editingServer = JSON.parse(JSON.stringify(server));
            if (this.editingServer.args) {
                this.argsString = this.editingServer.args.join('\n');
            } else {
                this.argsString = '';
            }
            if (this.editingServer.headers) {
                this.headersString = Object.entries(this.editingServer.headers)
                    .map(([k, v]) => `${k}: ${v}`)
                    .join('\n');
            } else {
                this.headersString = '';
            }
            if (this.editingServer.template_config) {
                this.templateHeaders = Object.entries(this.editingServer.template_config).map(([k, v]) => ({
                    key: k,
                    help: v
                }));
            } else {
                this.templateHeaders = [];
            }
            this.openModal();
        },
    },
    template: `
    <div class="mcp-manager">
        <div class="ui divider"></div>
        
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 25px;">
            <h3 class="ui header" style="margin: 0;">
                <i class="plug icon blue"></i>
                <div class="content">
                    MCP Tool Servers
                    <div class="sub header">Connect external tools to your AI Bots</div>
                </div>
            </h3>
            <button class="ui primary button" @click="openModal">
                <i class="plus icon"></i> Connect Server
            </button>
        </div>

        <div v-if="loading" class="ui active centered inline loader"></div>
        
        <div v-else-if="!servers || servers.length === 0" class="ui placeholder segment">
            <div class="ui icon header">
                <i class="server icon"></i>
                No MCP servers configured
                <div class="sub header">Add one to expand your bots' capabilities</div>
            </div>
        </div>

        <div v-else class="ui three stackable cards">
            <div v-for="srv in servers" :key="srv.id" class="ui card">
                <div class="content">
                    <div class="right floated">
                        <div class="ui label" :class="srv.type === 'sse' ? 'blue' : (srv.type === 'http' ? 'teal' : 'orange')">
                            {{ srv.type.toUpperCase() }}
                        </div>
                    </div>
                    <div class="header">
                        {{ srv.name }}
                        <div v-if="srv.is_template" class="ui mini label violet" style="margin-left: 5px;">Template</div>
                        <div v-if="getHealthLabel(srv.id).text !== 'UNKNOWN'" 
                             class="ui mini label" 
                             :class="getHealthLabel(srv.id).color" 
                             style="margin-left: 5px;"
                             :title="getHealthLabel(srv.id).message">
                            {{ getHealthLabel(srv.id).text }}
                        </div>
                    </div>
                    <div class="meta">
                        {{ srv.description || 'No description' }}
                        <div v-if="srv.tools && srv.tools.length" style="margin-top: 5px; color: #21ba45; font-weight: bold; font-size: 0.85rem;">
                            <i class="check circle icon"></i> {{ srv.tools.length }} tools loaded
                        </div>
                    </div>
                </div>
                <div class="content">
                    <div class="ui small message">
                        <code style="word-break: break-all;">{{ srv.url || (srv.command + ' ' + (srv.args || []).join(' ')) }}</code>
                    </div>
                </div>
                <div class="extra content">
                    <div class="ui four tiny basic buttons">
                        <button class="ui button" @click="checkServerHealth(srv)" :class="{ 'loading': checkingHealth[srv.id] }" title="Check health now">
                            <i class="heartbeat icon" :class="getHealthLabel(srv.id).color"></i>
                        </button>
                        <button class="ui button" @click="viewTools(srv)">
                            <i class="eye icon"></i> Tools
                        </button>
                        <button class="ui button" @click="editServer(srv)">
                            <i class="edit icon"></i>
                        </button>
                        <button class="ui button" @click="deleteServer(srv.id)">
                            <i class="trash icon red"></i>
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Add/Edit Modal -->
        <div id="mcpServerModal" class="ui modal">
            <div class="header">
                <i class="plug icon"></i>
                {{ editingServer ? 'Edit' : 'Connect' }} MCP Server
            </div>
            <div class="content">
                <div class="ui form">
                    <div class="field">
                        <label>Server Name</label>
                        <input type="text" v-model="target.name" placeholder="e.g. My NocoDB Database">
                    </div>

                    <div class="field">
                        <label>Description (optional)</label>
                        <input type="text" v-model="target.description" placeholder="Describe what this server is for">
                    </div>

                    <div class="field">
                        <label>Transport Method</label>
                        <div class="ui two column grid">
                            <div class="column">
                                <div class="ui fluid button" :class="{ 'primary': target.type === 'http' }" @click="target.type = 'http'">
                                    <i class="globe icon"></i><br>
                                    HTTP POST
                                </div>
                            </div>
                            <div class="column">
                                <div class="ui fluid button" :class="{ 'primary': target.type === 'sse' }" @click="target.type = 'sse'">
                                    <i class="rss icon"></i><br>
                                    SSE
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="field">
                        <label>Remote Server URL</label>
                        <input type="text" v-model="target.url" placeholder="https://api.example.com/mcp">
                    </div>

                    <div class="field">
                        <label>Custom Headers (optional)</label>
                        <textarea v-model="headersString" rows="2" placeholder="e.g.&#10;Authorization: Bearer your-token&#10;xc-mcp-token: your-token"></textarea>
                    </div>

                    <div class="ui info message">
                        <i class="info circle icon"></i>
                        Headers are sent with every request to the remote MCP server.
                    </div>

                    <div class="ui divider"></div>
                    
                    <div class="field">
                        <div class="ui checkbox">
                            <input type="checkbox" v-model="target.is_template" id="is_template_check">
                            <label for="is_template_check">Use as MCP Template</label>
                        </div>
                        <div style="font-size: 0.85em; color: #666; margin-top: 5px; margin-left: 20px;">
                            If checked, this server configuration will act as a template. You can define specific headers that MUST be configured 
                            individually for each Bot AI that uses this server (e.g. Auth Tokens).
                        </div>
                    </div>

                    <div v-if="target.is_template" class="ui segment secondary">
                        <h5 class="ui header">Template Configuration (Required Headers)</h5>
                        <div v-for="(th, idx) in templateHeaders" :key="idx" class="fields inline">
                            <div class="six wide field">
                                <input type="text" v-model="th.key" placeholder="Header Name (e.g. xc-token)">
                            </div>
                            <div class="eight wide field">
                                <input type="text" v-model="th.help" placeholder="Helper text (e.g. User Token)">
                            </div>
                            <div class="two wide field">
                                <button class="ui icon button red mini" @click="removeTemplateHeader(idx)">
                                    <i class="trash icon"></i>
                                </button>
                            </div>
                        </div>
                        <button class="ui button mini basic violet" @click="addTemplateHeader">
                            <i class="plus icon"></i> Add Required Header
                        </button>
                    </div>
                </div>
            </div>
            <div class="actions">
                <button class="ui cancel button" @click="closeModal">Cancel</button>
                <button class="ui primary button" @click="saveServer">
                    <i class="save icon"></i> Save
                </button>
            </div>
        </div>

        <!-- Tools Modal -->
        <div id="mcpToolsModal" class="ui modal">
            <div class="header" style="display: flex; justify-content: space-between; align-items: center;">
                <div class="title">
                    <i class="wrench icon teal"></i>
                    Available Tools
                    <span v-if="selectedServerTools" class="ui tiny teal label">{{ selectedServerTools.name }}</span>
                </div>
                <button class="ui tiny basic primary button" @click="viewTools(selectedServerTools, true)" :class="{ 'loading': loadingTools }">
                    <i class="sync icon"></i> Sync
                </button>
            </div>
            <div class="content" style="max-height: 400px; overflow-y: auto;">
                <div v-if="loadingTools" class="ui active centered inline loader"></div>
                <div v-else-if="tools.length === 0" class="ui placeholder segment">
                    <div class="ui icon header">
                        <i class="search icon"></i>
                        No tools detected
                        <div class="sub header">Make sure the server is properly configured and has tools exposed.</div>
                    </div>
                </div>
                <div v-else class="ui relaxed divided list">
                    <div v-for="tool in tools" :key="tool.name" class="item" style="padding: 12px 0;">
                        <i class="large wrench icon teal middle aligned"></i>
                        <div class="content">
                            <div class="header" style="color: #1e293b; font-weight: 700;">{{ tool.name }}</div>
                            <div class="description" style="color: #64748b; margin-top: 4px;">{{ tool.description }}</div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="actions">
                <button class="ui cancel button" @click="$('#mcpToolsModal').modal('hide')">Close</button>
            </div>
        </div>
    </div>
    `
};
