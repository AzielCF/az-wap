<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'
import AppModal from '@/components/AppModal.vue'
import WhatsAppControl from '@/components/WhatsAppControl.vue'
import ChannelConfig from '@/components/ChannelConfig.vue'
import ChannelInfo from '@/components/ChannelInfo.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { Trash2, Plus, Settings, Globe, ShieldAlert, CheckCircle2, Cpu, Edit3, Info } from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const api = useApi()

const workspace = ref<any>(null)
const channels = ref<any[]>([])
const loading = ref(true)

const bots = ref<any[]>([])
const credentials = ref<any[]>([])

const showAddChannel = ref(false)
const showWhatsAppControl = ref(false)
const showConfigModal = ref(false)
const showChannelInfo = ref(false)
const showEditWorkspace = ref(false)
const selectedChannel = ref<any>(null)

const confirmDialog = ref({
    show: false,
    title: '',
    message: '',
    type: 'danger' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

const newChannel = ref({
  name: '',
  type: 'whatsapp'
})

async function loadData() {
  const wsId = route.params.id as string
  try {
    const [ws, chs, bts, creds] = await Promise.all([
        api.get(`/workspaces/${wsId}`),
        api.get(`/workspaces/${wsId}/channels`),
        api.get('/bots'),
        api.get('/credentials')
    ])
    workspace.value = ws
    channels.value = chs || []
    bots.value = bts?.results || []
    credentials.value = creds?.results || []
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function createChannel() {
  try {
    await api.post(`/workspaces/${route.params.id}/channels`, newChannel.value)
    showAddChannel.value = false
    newChannel.value = { name: '', type: 'whatsapp' }
    await loadData()
  } catch (err) {
    alert('Failed to create channel')
  }
}

async function deleteChannel(cid: string) {
    confirmDialog.value = {
        show: true,
        title: 'Terminate Instance?',
        message: 'This action cannot be undone. The instance configuration and all associated logs will be permanently deleted.',
        type: 'danger',
        confirmText: 'Terminate',
        onConfirm: async () => {
            try {
                await api.delete(`/workspaces/${route.params.id}/channels/${cid}`)
                await loadData()
            } catch (err) {
                alert('Failed to delete')
            }
        }
    }
}

async function toggleChannel(ch: any) {
  const action = ch.enabled ? 'disable' : 'enable'
  try {
    await api.post(`/workspaces/${route.params.id}/channels/${ch.id}/${action}`, {})
    await loadData()
  } catch (err) {
    alert(`Failed to ${action} instance`)
  }
}

async function updateWorkspace() {
  try {
    await api.put(`/workspaces/${route.params.id}`, workspace.value)
    showEditWorkspace.value = false
    // alert('Workspace updated.') // Removed alert for smoother UX
  } catch (err) {
    alert('Failed to update')
  }
}

function deleteWorkspace() {
    confirmDialog.value = {
        show: true,
        title: 'Delete Entire Workspace?',
        message: 'Are you absolutely sure? This will delete the workspace metadata. Channels inside must be deleted manually first for safety.',
        type: 'danger',
        confirmText: 'Obliterate',
        onConfirm: async () => {
            try {
                await api.delete(`/workspaces/${route.params.id}`)
                router.push('/workspaces')
            } catch (err) {
               alert('Failed to delete workspace')
            }
        }
    }
}

function openWhatsAppControl(ch: any) {
  selectedChannel.value = ch
  showWhatsAppControl.value = true
}

function openConfig(ch: any) {
  selectedChannel.value = ch
  showConfigModal.value = true
}

function openInfo(ch: any) {
  selectedChannel.value = ch
  showChannelInfo.value = true
}

function copyId(id: string) {
    navigator.clipboard.writeText(id)
}

onMounted(loadData)
</script>

<template>
  <div v-if="loading" class="flex justify-center py-40">
    <span class="loading loading-ring loading-lg text-primary"></span>
  </div>

  <div v-else-if="workspace" class="space-y-10 animate-in fade-in duration-700 max-w-[1600px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-10 border-b border-white/5 mx-6 lg:mx-0">
      <div class="space-y-6 flex-1 w-full">
        <div class="flex items-center gap-3">
          <RouterLink to="/workspaces" class="text-[10px] font-black text-slate-500 hover:text-primary uppercase tracking-[0.25em] transition-colors">Infrastructure</RouterLink>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-[10px] font-black uppercase tracking-[0.25em] text-primary/70">{{ workspace.id.substring(0,8) }}</span>
        </div>
        <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none break-words">{{ workspace.name }}</h2>
        <div class="text-sm text-slate-500 font-bold uppercase tracking-widest pl-1">{{ workspace.description || 'No description provided' }}</div>
      </div>
      
      <div class="flex flex-col sm:flex-row gap-4 w-full lg:w-auto">
        <button class="btn-premium btn-premium-ghost w-full sm:w-auto h-14 border border-white/10 px-8" @click="showEditWorkspace = true">
           <Settings class="w-5 h-5 mr-3" />
           Configure
        </button>
        <button class="btn-premium btn-premium-primary px-10 h-14 w-full sm:w-auto" @click="showAddChannel = true">
           <Plus class="w-5 h-5 mr-2" />
           Deploy Instance
        </button>
      </div>
    </div>

    <!-- Table List -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Active Communication Nodes</div>
        <div class="bg-[#161a23]/40 border border-white/5 rounded-[2rem] shadow-2xl backdrop-blur-xl relative z-10">
            <div class="overflow-x-auto">
                <table class="table w-full border-collapse">
                    <thead>
                        <tr class="text-[10px] text-slate-500 uppercase tracking-widest border-b border-white/5 bg-white/[0.02]">
                            <th class="py-6 pl-10 font-bold">Instance Identification</th>
                            <th class="font-bold">Operational Status</th>
                            <th class="font-bold">Logic Engine</th>
                            <th class="font-bold">CRM Connection</th>
                            <th class="pr-10 text-right font-bold">Node Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="ch in channels" :key="ch.id" class="hover:bg-white/[0.03] transition-all border-b border-white/5 group">
                            <td class="py-8 pl-10">
                                <div class="flex items-center gap-6">
                                    <div class="icon-box-premium w-14 h-14 bg-black/40 ring-1 ring-white/5" v-if="ch.type === 'whatsapp'">
                                        <svg class="w-8 h-8 text-[#25D366]" viewBox="0 0 24 24" fill="currentColor">
                                            <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z"/>
                                        </svg>
                                    </div>
                                    <div class="icon-box-premium w-14 h-14 bg-black/40 ring-1 ring-white/5 text-slate-500" v-else>
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
                                    </div>
                                    <div>
                                        <div class="flex items-center gap-3">
                                            <div class="text-lg font-black text-white uppercase tracking-tighter leading-none mb-1 group-hover:text-primary transition-colors">{{ ch.name }}</div>
                                            <span class="text-[9px] font-black text-slate-600 uppercase tracking-widest px-2 py-0.5 bg-white/5 border border-white/5 rounded">{{ ch.type }}</span>
                                        </div>
                                        <div class="flex items-center gap-2 group/id cursor-pointer select-all" @click="copyId(ch.id)">
                                            <div class="text-[9px] font-mono text-slate-500 tracking-widest uppercase">{{ ch.id.substring(0,18) }}...</div>
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 text-slate-700 group-hover/id:text-primary transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
                                        </div>
                                    </div>
                                </div>
                            </td>
                            <td>
                                <div class="flex items-center gap-3">
                                    <template v-if="ch.status === 'hibernating'">
                                        <div class="w-2.5 h-2.5 rounded-full bg-primary/40 shadow-[0_0_10px_rgba(var(--p),0.3)]"></div>
                                        <span class="text-[10px] font-black uppercase tracking-widest text-primary/60 italic">Hibernating</span>
                                    </template>
                                    <template v-else>
                                        <div class="w-2.5 h-2.5 rounded-full" :class="ch.enabled ? 'bg-success shadow-[0_0_10px_rgba(var(--su),0.5)] animate-pulse' : 'bg-warning shadow-[0_0_10px_rgba(var(--wa),0.5)]'"></div>
                                        <span class="text-[10px] font-black uppercase tracking-widest" :class="ch.enabled ? 'text-slate-200' : 'text-slate-500'">{{ ch.enabled ? 'ACTIVE' : 'ON HOLD' }}</span>
                                    </template>
                                </div>
                            </td>
                            <td>
                                <div class="flex items-center gap-2.5">
                                    <div class="w-1.5 h-1.5 rounded-full" :class="ch.config?.bot_id ? 'bg-primary' : 'bg-slate-800'"></div>
                                    <span class="text-[10px] font-black uppercase tracking-widest" :class="ch.config?.bot_id ? 'text-slate-400' : 'text-slate-700'">{{ ch.config?.bot_id ? 'LINKED' : 'UNASSIGNED' }}</span>
                                </div>
                            </td>
                            <td>
                                <div class="flex items-center gap-2.5">
                                    <div class="w-1.5 h-1.5 rounded-full" :class="ch.config?.chatwoot?.enabled ? 'bg-amber-400' : 'bg-slate-800'"></div>
                                    <span class="text-[10px] font-black uppercase tracking-widest" :class="ch.config?.chatwoot?.enabled ? 'text-slate-400' : 'text-slate-700'">{{ ch.config?.chatwoot?.enabled ? 'BRIDGED' : 'STRAY' }}</span>
                                </div>
                            </td>
                            <td class="pr-10 text-right">
                                <div class="flex justify-end gap-3 items-center">
                                    <button v-if="ch.type === 'whatsapp'" class="btn-premium btn-premium-primary px-8 h-11 text-[10px]" @click="openWhatsAppControl(ch)">
                                        Open Protocol
                                    </button>
                                    <button class="btn-premium btn-premium-square btn-premium-sm btn-premium-ghost border border-white/10" @click="openConfig(ch)" title="Config">
                                        <Settings class="w-4 h-4 text-slate-400 group-hover:text-primary transition-colors" />
                                    </button>
                                    <button class="btn-premium btn-premium-square btn-premium-sm btn-premium-ghost border border-white/10" @click="openInfo(ch)" title="Inspector">
                                        <Info class="w-4 h-4 text-slate-400 group-hover:text-blue-400 transition-colors" />
                                    </button>
                                    <div class="dropdown dropdown-left dropdown-end">
                                        <button tabindex="0" class="btn-premium btn-premium-square btn-premium-sm btn-premium-ghost p-0 border border-white/10">
                                            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 opacity-40 group-hover:opacity-100 mx-auto" viewBox="0 0 20 20" fill="currentColor"><path d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z" /></svg>
                                        </button>
                                        <ul tabindex="0" class="dropdown-content z-[10] menu p-3 shadow-2xl bg-[#0b0e14] rounded-2xl border border-white/10 w-64 animate-in fade-in zoom-in duration-200 mt-2">
                                            <li><button class="flex items-center justify-between py-4 px-5 rounded-xl hover:bg-white/5 text-slate-300 font-black uppercase text-[10px] tracking-widest" @click="toggleChannel(ch)">
                                                <span>{{ ch.enabled ? 'Pause Infrastructure' : 'Resume Protocol' }}</span>
                                                <div class="w-2.5 h-2.5 rounded-full" :class="ch.enabled ? 'bg-warning' : 'bg-success shadow-[0_0_10px_rgba(var(--su),0.5)]'"></div>
                                            </button></li>
                                            <div class="divider opacity-30 my-1"></div>
                                            <li><button class="text-error flex items-center justify-between py-4 px-5 rounded-xl hover:bg-red-500/10 font-black uppercase text-[10px] tracking-widest" @click="deleteChannel(ch.id)">
                                                <span>Terminate instance</span>
                                                <Trash2 class="w-4 h-4" />
                                            </button></li>
                                        </ul>
                                    </div>
                                </div>
                            </td>
                        </tr>
                        <tr v-if="channels.length === 0">
                            <td colspan="5" class="py-32 text-center bg-black/5">
                                <div class="flex flex-col items-center gap-4 opacity-20">
                                    <Cpu class="w-12 h-12 text-slate-500" />
                                    <div class="text-[11px] font-black uppercase tracking-[0.4em] text-slate-300">Central Cluster: No instances deployed.</div>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <!-- Modals -->
    <AppModal v-model="showAddChannel" title="Deploy New Instance" maxWidth="max-w-2xl" noPadding noScroll>
      <div class="flex flex-col h-full overflow-hidden">
          <div class="flex-1 overflow-y-auto p-12 custom-scrollbar space-y-12 bg-[#0b0e14]">
            <div class="section-title-premium text-primary/60">Logic Port Definition</div>
            
            <!-- Visual Header -->
            <div class="flex flex-col sm:flex-row items-center gap-6 sm:gap-8 p-6 sm:p-10 bg-white/[0.02] rounded-[2rem] border border-white/5 text-center sm:text-left">
                <div class="w-16 h-16 sm:w-20 sm:h-20 rounded-[1.5rem] bg-black/40 flex items-center justify-center ring-1 ring-white/10 shadow-2xl flex-none">
                    <svg class="w-8 h-8 sm:w-10 sm:h-10 text-[#25D366]" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z"/>
                    </svg>
                </div>
                <div class="flex-1">
                    <h3 class="text-xl sm:text-2xl font-black text-white uppercase tracking-tighter mb-1 leading-none">WhatsApp Web Gateway</h3>
                    <p class="text-[9px] sm:text-[10px] text-slate-600 font-bold uppercase tracking-[0.2em]">Secure protocol bridge for personal accounts</p>
                </div>
            </div>

            <!-- Form Fields -->
            <div class="space-y-8">
                <div class="form-control w-full">
                    <label class="label-premium">Infrastructure Label</label>
                    <input v-model="newChannel.name" type="text" placeholder="e.g. Sales Department Core" class="input-premium h-14 sm:h-16 w-full text-base sm:text-lg font-black" />
                    <p class="mt-3 text-[10px] text-slate-700 font-bold uppercase tracking-wider">Internal reference for the management node</p>
                </div>

                <div class="form-control w-full">
                    <label class="label-premium">Transport Protocol Selection</label>
                    <div class="p-4 sm:p-6 bg-white/[0.03] rounded-2xl border border-white/5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 group cursor-default">
                        <div class="flex items-center gap-4">
                            <div class="w-10 h-10 rounded-xl bg-[#25D366]/10 flex items-center justify-center text-[#25D366] flex-none">
                                <Globe class="w-5 h-5" />
                            </div>
                            <span class="text-xs font-black text-white uppercase tracking-widest leading-none">WhatsApp Web Native</span>
                        </div>
                        <div class="badge-premium badge-success text-success border-success/20 w-full sm:w-auto text-center justify-center">STABLE</div>
                    </div>
                </div>
            </div>

            <!-- Warning Box -->
            <div class="p-6 sm:p-8 bg-amber-500/5 border border-amber-500/10 rounded-2xl flex flex-col sm:flex-row gap-4 sm:gap-6">
                <ShieldAlert class="w-6 h-6 text-amber-500 shrink-0 mx-auto sm:mx-0" />
                <div class="text-[10px] sm:text-[11px] text-amber-500/80 leading-relaxed font-bold uppercase tracking-wide text-center sm:text-left">
                    deployment requires active QR synchronization. this implementation utilizes the whatsapp web protocol and does not require an official business API registration.
                </div>
            </div>
          </div>

          <!-- Fixed Footer -->
          <div class="flex-none p-6 sm:p-10 border-t border-white/5 bg-[#0b0e14] flex flex-col-reverse sm:flex-row justify-end gap-4 sm:gap-6 text-right">
            <button class="btn-premium btn-premium-ghost px-12 h-14" @click="showAddChannel = false">Abort</button>
            <button class="btn-premium btn-premium-primary px-20 h-14 sm:h-16" @click="createChannel" :disabled="!newChannel.name || loading">
                <CheckCircle2 class="w-5 h-5 mr-3" />
                Authorize Deployment
            </button>
          </div>
      </div>
    </AppModal>

    <ChannelConfig 
        v-if="showConfigModal && selectedChannel"
        :channel="selectedChannel" 
        :workspaceId="(route.params.id as string)"
        :bots="bots"
        :credentials="credentials"
        @saved="loadData(); showConfigModal = false"
        @cancel="showConfigModal = false"
    />

    <ChannelInfo 
        v-if="showChannelInfo && selectedChannel"
        :channel="selectedChannel"
        :workspaceId="(route.params.id as string)"
        @close="showChannelInfo = false"
    />

    <AppModal v-model="showWhatsAppControl" title="WhatsApp Link Protocol" maxWidth="max-w-lg" noPadding>
      <div class="p-10">
        <WhatsAppControl 
          v-if="selectedChannel"
          :channel="selectedChannel"
          :workspaceId="(route.params.id as string)"
        />
      </div>
    </AppModal>

    <!-- Workspace Edit Modal -->
    <AppModal v-model="showEditWorkspace" title="Workspace Administration" maxWidth="max-w-xl" noPadding noScroll>
        <div class="flex flex-col h-full overflow-hidden">
            <div class="flex-1 overflow-y-auto p-6 sm:p-12 custom-scrollbar space-y-10 bg-[#0b0e14]">
                <div class="section-title-premium text-primary/60">Metadata Configuration</div>
                <div class="space-y-8">
                    <div class="form-control w-full">
                        <label class="label-premium">Workspace Identification</label>
                        <input v-model="workspace.name" type="text" class="input-premium h-14 text-lg font-black w-full" />
                    </div>
                    <div class="form-control w-full">
                        <label class="label-premium">Operational Context</label>
                        <textarea v-model="workspace.description" class="input-premium h-32 py-4 resize-none w-full"></textarea>
                    </div>
                </div>

                <div class="h-px bg-white/10 my-8"></div>

                <div class="p-6 bg-red-500/5 rounded-2xl border border-red-500/10 space-y-4">
                    <div class="flex items-center gap-3 text-red-500">
                        <ShieldAlert class="w-5 h-5" />
                        <span class="font-black uppercase tracking-widest text-[10px]">Danger Zone</span>
                    </div>
                    <p class="text-[10px] text-slate-500 font-bold uppercase tracking-wide">
                        Terminating this workspace will unlink all associated telemetry data. This action is irreversible.
                    </p>
                    <button class="btn-premium w-full border border-red-500/20 text-red-500/80 hover:bg-red-500 hover:text-white h-12" @click="deleteWorkspace">
                        Terminate Workspace Protocol
                    </button>
                </div>
            </div>
            
            <!-- Responsive Footer -->
            <div class="flex-none p-6 sm:p-8 border-t border-white/5 bg-[#0b0e14] flex flex-col-reverse sm:flex-row justify-end gap-4 sm:gap-6">
                 <button class="btn-premium btn-premium-ghost px-10 h-14 w-full sm:w-auto text-sm" @click="showEditWorkspace = false">Discard</button>
                 <button class="btn-premium btn-premium-primary px-16 h-14 w-full sm:w-auto text-sm" @click="updateWorkspace">Save Changes</button>
            </div>
        </div>
    </AppModal>

    <ConfirmationDialog 
        v-model="confirmDialog.show" 
        :title="confirmDialog.title" 
        :message="confirmDialog.message" 
        :type="confirmDialog.type" 
        :confirmText="confirmDialog.confirmText"
        @confirm="confirmDialog.onConfirm(); confirmDialog.show = false" 
        @cancel="confirmDialog.show = false" 
    />
  </div>
</template>

<style scoped>
</style>

