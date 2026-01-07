export default {
    name: 'MCPManager',
    data() {
        return {
            servers: [],
            loading: false,
            showAddModal: false,
            newServer: {
                name: '',
                description: '',
                type: 'sse',
                url: '',
                command: '',
                args: [],
            },
            editingServer: null,
            tools: [],
            loadingTools: false,
            selectedServerTools: null,
            // Helper for UI
            argsString: '',
        };
    },
    mounted() {
        this.fetchServers();
    },
    methods: {
        async fetchServers() {
            this.loading = true;
            try {
                const response = await fetch('/api/mcp/servers');
                const data = await response.json();
                if (data.status === 200) {
                    this.servers = data.results || [];
                }
            } catch (err) {
                console.error('Failed to fetch MCP servers:', err);
            } finally {
                this.loading = false;
            }
        },
        async saveServer() {
            try {
                const url = this.editingServer ? `/api/mcp/servers/${this.editingServer.id}` : '/api/mcp/servers';
                const method = this.editingServer ? 'PUT' : 'POST';
                const target = this.editingServer || this.newServer;
                
                // Parse args from string if in stdio mode
                if (target.type === 'stdio') {
                    // Split by newline and filter empty lines
                    target.args = this.argsString.split('\n').map(s => s.trim()).filter(s => s !== '');
                    target.url = ''; // reset url for stdio
                } else {
                    target.args = [];
                    target.command = ''; // reset command for sse
                }

                const response = await fetch(url, {
                    method: method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(target),
                });

                if (response.ok) {
                    this.showAddModal = false;
                    this.editingServer = null;
                    this.resetForm();
                    this.fetchServers();
                    this.$emit('mcp-servers-updated');
                } else {
                    const error = await response.json();
                    alert(`Error: ${error.message}`);
                }
            } catch (err) {
                alert(`Error saving server: ${err.message}`);
            }
        },
        resetForm() {
            this.newServer = { name: '', description: '', type: 'sse', url: '', command: '', args: [] };
            this.argsString = '';
        },
        async deleteServer(id) {
            if (!confirm('Are you sure you want to delete this MCP server?')) return;
            try {
                const response = await fetch(`/api/mcp/servers/${id}`, { method: 'DELETE' });
                if (response.ok) {
                    this.fetchServers();
                    this.$emit('mcp-servers-updated');
                }
            } catch (err) {
                console.error('Failed to delete server:', err);
            }
        },
        async viewTools(server) {
            this.selectedServerTools = server;
            this.loadingTools = true;
            this.tools = [];
            try {
                const response = await fetch(`/api/mcp/servers/${server.id}/tools`);
                const data = await response.json();
                if (data.status === 200) {
                    this.tools = data.results || [];
                } else {
                    alert(`Error: ${data.message}`);
                }
            } catch (err) {
                alert(`Error fetching tools: ${err.message}`);
            } finally {
                this.loadingTools = false;
            }
        },
        editServer(server) {
            this.editingServer = { ...server };
            if (this.editingServer.args) {
                this.argsString = this.editingServer.args.join('\n');
            } else {
                this.argsString = '';
            }
            this.showAddModal = true;
        },
    },
    template: `
    <div class="mcp-manager" style="margin-top: 20px;">
        <div class="ui divider"></div>
        <div class="ui flex-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
            <h4 class="ui header" style="margin: 0;">
                <i class="tools icon"></i>
                <div class="content">
                    MCP Tool Servers
                    <div class="sub header">External tools for your AI Bots</div>
                </div>
            </h4>
            <button class="ui tiny teal button" @click="showAddModal = true; editingServer = null; resetForm();">
                <i class="plus icon"></i> Add Server
            </button>
        </div>

        <div v-if="loading" class="ui centered inline loader active"></div>
        
        <div v-else-if="servers.length === 0" class="ui placeholder segment" style="min-height: 100px;">
            <div class="ui icon header">
                <i class="server icon"></i>
                No MCP servers configured yet.
            </div>
        </div>

        <div v-else class="ui middle aligned divided list">
            <div v-for="srv in servers" :key="srv.id" class="item" style="padding: 10px 0;">
                <div class="right floated content">
                    <button class="ui tiny compact icon button" @click="viewTools(srv)" title="View Tools">
                        <i class="eye icon"></i>
                    </button>
                    <button class="ui tiny compact icon button" @click="editServer(srv)" title="Edit">
                        <i class="edit icon"></i>
                    </button>
                    <button class="ui tiny compact icon red button" @click="deleteServer(srv.id)" title="Delete">
                        <i class="trash icon"></i>
                    </button>
                </div>
                <i class="large server icon" :class="srv.enabled ? 'green' : 'grey'"></i>
                <div class="content">
                    <span class="header">{{ srv.name }}</span>
                    <div class="description">
                        <div class="ui horizontal label" :class="srv.type === 'sse' ? 'blue' : 'orange'">{{ srv.type.toUpperCase() }}</div>
                        <code v-if="srv.type === 'stdio'">{{ srv.command }} {{ (srv.args || []).join(' ') }}</code>
                        <span v-else>{{ srv.url }}</span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Add/Edit Modal -->
        <div v-if="showAddModal" class="ui modal active" style="top: 5%; display: block !important; max-height: 90vh; overflow-y: auto;">
            <div class="header">{{ editingServer ? 'Edit' : 'Add New' }} MCP Server</div>
            <div class="content">
                <div class="ui form">
                    <div class="field">
                        <label>Name</label>
                        <input type="text" v-model="(editingServer || newServer).name" placeholder="E.g. NocoDB Travelmika">
                    </div>
                    <div class="field">
                        <label>Description</label>
                        <textarea rows="2" v-model="(editingServer || newServer).description" placeholder="What this server provides..."></textarea>
                    </div>
                    <div class="field">
                        <label>Connection Type</label>
                        <select v-model="(editingServer || newServer).type" class="ui dropdown">
                            <option value="sse">SSE (HTTP/HTTPS Remote)</option>
                            <option value="stdio">Stdio (Local Command / Proxy)</option>
                        </select>
                    </div>

                    <div v-if="(editingServer || newServer).type === 'sse'" class="field">
                        <label>URL</label>
                        <input type="text" v-model="(editingServer || newServer).url" placeholder="https://mcp.api.com/events">
                        <div class="ui blue message small" style="margin-top: 5px;">
                            <i class="info icon"></i> Direct SSE connection is recommended for remote servers.
                        </div>
                    </div>

                    <div v-if="(editingServer || newServer).type === 'stdio'">
                        <div class="field">
                            <label>Command</label>
                            <input type="text" v-model="(editingServer || newServer).command" placeholder="e.g. npx, node, python">
                        </div>
                        <div class="field">
                            <label>Arguments (One per line)</label>
                            <textarea rows="4" v-model="argsString" placeholder="e.g.&#10;mcp-remote&#10;https://my-server.com&#10;--header&#10;auth-token: value"></textarea>
                        </div>
                        <div class="ui orange message small" style="margin-top: 5px;">
                            <i class="warning icon"></i> <b>Stdio</b> executes local commands. Use with caution. 
                            For <code>mcp-remote</code>, ensure <code>npx</code> is installed in the global path.
                        </div>
                    </div>
                </div>
            </div>
            <div class="actions">
                <button class="ui black deny button" @click="showAddModal = false">Cancel</button>
                <button class="ui positive right labeled icon button" @click="saveServer">
                    Save <i class="checkmark icon"></i>
                </button>
            </div>
        </div>

        <!-- Tools Viewer Modal -->
        <div v-if="selectedServerTools" class="ui modal active" style="top: 10%; display: block !important;">
            <div class="header">Tools: {{ selectedServerTools.name }}</div>
            <div class="content" style="max-height: 400px; overflow-y: auto;">
                <div v-if="loadingTools" class="ui centered inline loader active"></div>
                <div v-else-if="tools.length === 0" class="ui message">No tools found on this server.</div>
                <div v-else class="ui divided list">
                    <div v-for="tool in tools" :key="tool.name" class="item" style="padding: 10px 0;">
                        <i class="wrench icon teal"></i>
                        <div class="content">
                            <div class="header">{{ tool.name }}</div>
                            <div class="description">{{ tool.description }}</div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="actions">
                <button class="ui blue button" @click="selectedServerTools = null">Close</button>
            </div>
        </div>
        
        <div v-if="showAddModal || selectedServerTools" class="ui dimmer modals page transition visible active" style="display: block !important;"></div>
    </div>
    `
};
