export default {
    name: 'WorkspaceEditor',
    props: {
        workspace: {
            type: Object,
            default: null
        }
    },
    emits: ['saved', 'close'],
    data() {
        return {
            form: {
                name: '',
                description: '',
                owner_id: 'admin' // Default owner
            },
            loading: false,
            error: null
        }
    },
    watch: {
        workspace: {
            immediate: true,
            handler(val) {
                if (val) {
                    this.form = { ...val };
                } else {
                    this.form = {
                        name: '',
                        description: '',
                        owner_id: 'admin'
                    };
                }
            }
        }
    },
    methods: {
        async save() {
            this.loading = true;
            this.error = null;
            try {
                if (this.workspace && this.workspace.id) {
                    await window.http.put(`/workspaces/${this.workspace.id}`, this.form);
                } else {
                    await window.http.post('/workspaces', this.form);
                }
                window.showSuccessInfo('Workspace saved successfully');
                this.$emit('saved');
                this.close();
            } catch (err) {
                console.error(err);
                this.error = err.response?.data?.error || err.message;
                window.showErrorInfo(this.error);
            } finally {
                this.loading = false;
            }
        },
        close() {
            this.$emit('close');
        }
    },
    template: `
    <div class="ui modal" id="modalWorkspaceEditor">
        <div class="header">{{ workspace ? 'Edit Workspace' : 'New Workspace' }}</div>
        <div class="content">
            <div class="ui form" :class="{loading: loading}">
                <div class="field">
                    <label>Name</label>
                    <input type="text" v-model="form.name" placeholder="My Workspace">
                </div>
                <div class="field">
                    <label>Description</label>
                    <input type="text" v-model="form.description" placeholder="Optional description">
                </div>
                <div class="field">
                    <label>Owner ID</label>
                    <input type="text" v-model="form.owner_id" placeholder="Owner User ID">
                </div>
                <div class="ui error message" v-if="error" style="display:block">{{ error }}</div>
            </div>
        </div>
        <div class="actions">
            <div class="ui black deny button" @click="close">Cancel</div>
            <div class="ui positive right labeled icon button" @click="save">
                Save
                <i class="checkmark icon"></i>
            </div>
        </div>
    </div>
    `,
    mounted() {
        $('#modalWorkspaceEditor').modal({
            onHidden: () => {
                this.$emit('close');
            }
        }).modal('show');
    },
    unmounted() {
        $('#modalWorkspaceEditor').modal('hide');
    }
}
