<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'
import AppModal from '@/components/AppModal.vue'
import { Layers, RefreshCw, Plus, Edit3, Trash2, CheckCircle2, MoreVertical } from 'lucide-vue-next'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'

const router = useRouter()
const api = useApi()
const workspaces = ref<any[]>([])
const loading = ref(false)

const showModal = ref(false)
const showEditModal = ref(false)
const newWs = ref({ name: '', description: '', owner_id: '' })
const editWs = ref({ id: '', name: '', description: '', owner_id: '' })
const confirmDialog = ref({
    show: false,
    title: '',
    message: '',
    type: 'danger' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

async function loadWorkspaces() {
  loading.value = true
  try {
    const data = await api.get('/workspaces')
    workspaces.value = data || []
  } catch (err) {
    console.error('Failed to load workspaces:', err)
  } finally {
    loading.value = false
  }
}

async function createWorkspace() {
  if (!newWs.value.name) return
  try {
    await api.post('/workspaces', newWs.value)
    newWs.value = { name: '', description: '', owner_id: '' }
    showModal.value = false
    await loadWorkspaces()
  } catch (err) {
    alert('Failed to create workspace')
  }
}

function openEdit(ws: any) {
  editWs.value = { 
    id: ws.id, 
    name: ws.name, 
    description: ws.description || '',
    owner_id: ws.owner_id || ''
  }
  showEditModal.value = true
}

async function updateWorkspace() {
  try {
    await api.put(`/workspaces/${editWs.value.id}`, editWs.value)
    showEditModal.value = false
    await loadWorkspaces()
  } catch (err) {
    alert('Failed to update workspace')
  }
}

async function deleteWorkspace(ws: any) {
    // Phase 1: Initial Warning
    confirmDialog.value = {
        show: true,
        title: 'Delete Node?',
        message: `Are you sure you want to delete workspace "${ws.name}"? This will require a second confirmation.`,
        type: 'warning',
        confirmText: 'Proceed',
        onConfirm: () => {
             // Close first dialog
             confirmDialog.value.show = false;
             // Wait for animation loop then trigger phase 2
             setTimeout(() => {
                 triggerFinalDelete(ws)
             }, 200)
        }
    }
}

function triggerFinalDelete(ws: any) {
    confirmDialog.value = {
        show: true,
        title: 'IRREVERSIBLE ACTION',
        message: 'This will permanently destroy all channels, logs, and configurations within this node. There is no undo. Confirm destruction?',
        type: 'danger',
        confirmText: 'DESTROY NODE',
        onConfirm: async () => {
            try {
                await api.delete(`/workspaces/${ws.id}`)
                await loadWorkspaces()
                confirmDialog.value.show = false
            } catch (err) {
                confirmDialog.value.show = false
                alert('Failed to delete workspace')
            }
        }
    }
}

onMounted(loadWorkspaces)
</script>

<template>
  <div class="space-y-10 max-w-[1500px] mx-auto pb-20 animate-in fade-in duration-500">
    <!-- Professional Header -->
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-10 border-b border-white/5 mx-6 lg:mx-0">
      <div class="space-y-6 flex-1">
        <div class="flex items-center gap-3">
          <Layers class="w-4 h-4 text-primary" />
          <span class="section-title-premium py-0 border-none pl-0 text-primary">Multi-Tenant Core</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-[10px] font-black uppercase tracking-[0.25em] text-slate-500">Infrastructure Nodes</span>
        </div>
        <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none">Workspaces</h2>
      </div>
      
      <div class="flex flex-col lg:flex-row gap-4">
        <button class="btn-premium btn-premium-ghost px-10 h-14 w-full lg:w-auto" @click="loadWorkspaces" :class="{ loading: loading }">
            <RefreshCw v-if="!loading" class="w-4 h-4 mr-2" />
            Synchronize
        </button>
        <button class="btn-premium btn-premium-primary px-16 h-14 w-full lg:w-auto" @click="showModal = true">
            <Plus class="w-5 h-5 mr-2" />
            New Environment
        </button>
      </div>
    </div>

    <!-- Workspaces Table (Serious Manager Style) -->
    <div class="px-6 lg:px-0">
        <div class="bg-[#161a23]/40 border border-white/5 rounded-[2rem] shadow-2xl backdrop-blur-xl relative z-10">
            <div class="overflow-x-auto lg:overflow-visible">
                <table class="table w-full border-collapse">
                    <thead>
                        <tr class="text-[10px] text-slate-500 uppercase tracking-widest border-b border-white/5 bg-white/[0.02]">
                            <th class="py-6 pl-10 font-black">Node Identification</th>
                            <th class="font-black">Operational Context</th>
                            <th class="font-black">Owner ID</th>
                            <th class="font-black">System Status</th>
                            <th class="pr-10 text-right font-black">Control Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="ws in workspaces" :key="ws.id" class="hover:bg-white/[0.03] transition-all border-b border-white/5 group">
                            <td class="py-10 pl-10">
                                <div class="flex items-center gap-6">
                                    <div class="icon-box-premium w-14 h-14" :class="'icon-box-primary'">
                                        <Layers class="w-7 h-7" />
                                    </div>
                                    <div>
                                        <div class="text-lg font-black text-white uppercase tracking-tighter leading-none mb-1 group-hover:text-primary transition-colors cursor-pointer" @click="router.push(`/workspaces/${ws.id}`)">{{ ws.name }}</div>
                                        <div class="text-[9px] font-mono text-slate-600 uppercase tracking-widest select-all">{{ ws.id }}</div>
                                    </div>
                                </div>
                            </td>
                            <td class="text-xs text-slate-500 max-w-xs truncate font-medium uppercase italic">{{ ws.description || 'No operational manifest defined.' }}</td>
                            <td class="text-[10px] font-black text-slate-600 uppercase tracking-widest font-mono">{{ ws.owner_id || 'SYSTEM_CORE' }}</td>
                            <td>
                                <div class="flex items-center gap-2">
                                    <div class="w-2 h-2 rounded-full bg-success shadow-[0_0_8px_rgba(var(--su),0.5)] animate-pulse"></div>
                                    <span class="text-[10px] font-black text-slate-300 uppercase tracking-widest">ACTIVE</span>
                                </div>
                            </td>
                            <td class="pr-10 text-right">
                                <div class="flex justify-end gap-3 items-center">
                                    <button @click="openEdit(ws)" class="btn-premium btn-premium-square btn-premium-sm btn-premium-ghost border border-white/10 lg:hidden" title="Edit Metadata">
                                        <Edit3 class="w-4 h-4 text-slate-400 group-hover:text-primary transition-colors" />
                                    </button>
                                     <button @click="router.push(`/workspaces/${ws.id}`)" class="btn-premium btn-premium-primary px-6 h-10 text-[10px] shadow-lg shadow-primary/20">
                                        Open Cluster
                                    </button>
                                    <div class="dropdown dropdown-end">
                                        <button tabindex="0" class="btn-premium btn-premium-square btn-premium-sm btn-premium-ghost border border-white/10">
                                            <MoreVertical class="w-4 h-4 text-slate-400" />
                                        </button>
                                        <ul tabindex="0" class="dropdown-content z-[10] menu p-2 shadow-2xl bg-[#0b0e14] rounded-xl border border-white/10 w-52 mt-2">
                                            <li>
                                                <button @click="openEdit(ws)" class="flex items-center gap-3 text-xs font-bold uppercase tracking-wider text-slate-300 hover:text-white hover:bg-white/5 py-3">
                                                    <Edit3 class="w-4 h-4" /> Edit Metadata
                                                </button>
                                            </li>
                                             <div class="h-px bg-white/5 my-1"></div>
                                            <li>
                                                <button @click="deleteWorkspace(ws)" class="flex items-center gap-3 text-xs font-bold uppercase tracking-wider text-red-400 hover:text-red-500 hover:bg-red-500/10 py-3">
                                                    <Trash2 class="w-4 h-4" /> Delete Node
                                                </button>
                                            </li>
                                        </ul>
                                    </div>
                                </div>
                            </td>
                        </tr>
                        <tr v-if="workspaces.length === 0">
                            <td colspan="5" class="py-32 text-center bg-black/5">
                                <div class="flex flex-col items-center gap-4 opacity-20">
                                    <Layers class="w-12 h-12 text-slate-500" />
                                    <div class="text-[11px] font-black uppercase tracking-[0.4em] text-slate-300">Central Gateway: No nodes found in cluster.</div>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- Create/Edit Modal -->
    <AppModal v-model="showModal" title="Initialize New Environment" maxWidth="max-w-xl" noPadding noScroll>
      <div class="flex flex-col h-full overflow-hidden">
          <div class="flex-1 p-6 sm:p-12 space-y-8 overflow-y-auto custom-scrollbar bg-[#0b0e14]">
              <div class="section-title-premium text-primary/60">Node Definition</div>
              <div class="space-y-8">
                  <div class="form-control w-full">
                      <label class="label-premium">Environment Name</label>
                      <input v-model="newWs.name" type="text" class="input-premium h-14 w-full text-lg font-black" />
                  </div>
                  <div class="form-control w-full">
                      <label class="label-premium">Operational Manifest</label>
                      <textarea v-model="newWs.description" class="input-premium h-32 py-5 resize-none w-full"></textarea>
                  </div>
                  <div class="form-control w-full">
                      <label class="label-premium">Owner Identifier</label>
                      <input v-model="newWs.owner_id" type="text" class="input-premium h-14 w-full font-mono text-sm" />
                  </div>
              </div>
          </div>
          <div class="flex-none p-6 sm:p-8 border-t border-white/5 bg-[#0b0e14] flex flex-col-reverse sm:flex-row justify-end gap-4 sm:gap-6">
              <button class="btn-premium btn-premium-ghost px-10 h-14 w-full sm:w-auto text-sm" @click="showModal = false">Discard</button>
              <button class="btn-premium btn-premium-primary px-16 h-14 w-full sm:w-auto text-sm" @click="createWorkspace" :disabled="!newWs.name || loading">
                  <CheckCircle2 class="w-4 h-4 mr-2" />
                  Create Node
              </button>
          </div>
      </div>
    </AppModal>

    <AppModal v-model="showEditModal" title="Edit Infrastructure Metadata" maxWidth="max-w-xl" noPadding noScroll>
      <div class="flex flex-col h-full overflow-hidden">
          <div class="flex-1 p-6 sm:p-12 space-y-8 overflow-y-auto custom-scrollbar bg-[#0b0e14]">
              <div class="section-title-premium text-primary/60">Node Updates</div>
              <div class="space-y-8">
                  <div class="form-control w-full">
                      <label class="label-premium">Environment Name</label>
                      <input v-model="editWs.name" type="text" class="input-premium h-14 w-full text-lg font-black" />
                  </div>
                  <div class="form-control w-full">
                      <label class="label-premium">Operational Manifest</label>
                      <textarea v-model="editWs.description" class="input-premium h-32 py-5 resize-none w-full"></textarea>
                  </div>
                  <div class="form-control w-full">
                      <label class="label-premium">Owner Identifier</label>
                      <input v-model="editWs.owner_id" type="text" class="input-premium h-14 w-full font-mono text-sm" />
                  </div>
              </div>
          </div>

          <!-- Responsive Footer -->
          <div class="flex-none p-6 sm:p-8 border-t border-white/5 bg-[#0b0e14] flex flex-col-reverse sm:flex-row justify-end gap-4 sm:gap-6">
            <button class="btn-premium btn-premium-ghost px-10 h-14 w-full sm:w-auto text-sm" @click="showEditModal = false">Discard</button>
            <button class="btn-premium btn-premium-primary px-16 h-14 w-full sm:w-auto text-sm" @click="updateWorkspace" :disabled="!editWs.name || loading">
                <CheckCircle2 class="w-4 h-4 mr-2" />
                Update Node
            </button>
          </div>
      </div>
    </AppModal>

    <ConfirmationDialog 
        v-model="confirmDialog.show" 
        :title="confirmDialog.title" 
        :message="confirmDialog.message" 
        :type="confirmDialog.type" 
        :confirmText="confirmDialog.confirmText"
        @confirm="confirmDialog.onConfirm()" 
        @cancel="confirmDialog.show = false" 
    />
  </div>
</template>

<style scoped>
.table :where(thead, tfoot) :where(th, td) { background-color: transparent !important; color: inherit; font-size: 11px; font-weight: bold; border: none; }
</style>
