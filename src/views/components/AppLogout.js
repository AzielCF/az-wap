export default {
    name: 'AppLogout',
    methods: {
        async handleSubmit() {
            try {
                // Validar que haya una instancia activa seleccionada en la UI
                if (!window.ACTIVE_INSTANCE_TOKEN) {
                    showErrorInfo('You must select an instance in the Instance Manager (Use in UI) before logging out.');
                    return;
                }

                await this.submitApi()
                showSuccessInfo("Logout success")

                // fetch devices
                this.$emit('reload-devices')

                // Forzar también la recarga de instancias desde el root,
                // para actualizar inmediatamente las etiquetas ONLINE/OFFLINE.
                if (this.$root && typeof this.$root.loadInstances === 'function') {
                    this.$root.loadInstances()
                }

            } catch (err) {
                showErrorInfo(err)
            }
        },

        async submitApi() {
            try {
                await window.http.get(`/app/logout`)
            } catch (error) {
                if (error.response) {
                    throw Error(error.response.data.message)
                }
                throw Error(error.message)
            }

        }
    },
    template: `
    <div class="green card" @click="handleSubmit" style="cursor: pointer">
        <div class="content">
            <a class="ui teal right ribbon label">App</a>
            <div class="header">Logout</div>
            <div class="description">
                Remove your login session in application
            </div>
        </div>
    </div>
    `
}