import WorkspaceEditor from "./WorkspaceEditor.js";
import ChannelManager from "./ChannelManager.js";

export default {
    name: 'WorkspaceManager',
    components: { WorkspaceEditor, ChannelManager },
    data() {
        return {
            workspaces: [],
            loading: false,
            showEditor: false,
            showChannels: false,
            selectedWorkspace: null
        }
    },
    methods: {
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
        openCreate() {
            this.selectedWorkspace = null;
            this.showEditor = true;
        },
        openEdit(ws) {
            this.selectedWorkspace = ws;
            this.showEditor = true;
        },
        openChannels(ws) {
            this.selectedWorkspace = ws;
            this.showChannels = true;
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
        onEditorSaved() {
            this.loadWorkspaces();
        }
    },
    mounted() {
        this.loadWorkspaces();
    },
    template: `
    <div class="ui card fluid">
        <div class="content">
            <div class="right floated meta">
                <button class="ui primary mini button" @click="openCreate">
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
                                <i class="building icon"></i> <b>{{ ws.name }}</b>
                                <div class="ui label mini">{{ ws.id }}</div>
                            </td>
                            <td>{{ ws.description }}</td>
                            <td>{{ ws.owner_id }}</td>
                            <td>
                                <div class="ui label green" v-if="ws.enabled">Active</div>
                                <div class="ui label red" v-else>Disabled</div>
                            </td>
                            <td>
                                <div class="ui icon buttons mini">
                                    <button class="ui button" @click="openEdit(ws)" title="Edit">
                                        <i class="edit icon"></i>
                                    </button>
                                    <button class="ui teal button" @click="openChannels(ws)" title="Manage Channels">
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

        <!-- Modals -->
        <workspace-editor 
            v-if="showEditor" 
            :workspace="selectedWorkspace" 
            @saved="onEditorSaved" 
            @close="showEditor = false">
        </workspace-editor>

        <channel-manager 
            v-if="showChannels" 
            :workspace-id="selectedWorkspace.id" 
            @close="showChannels = false">
        </channel-manager>
    </div>
    `
}
