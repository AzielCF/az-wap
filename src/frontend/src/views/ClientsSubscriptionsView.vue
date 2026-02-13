<script setup lang="ts">
import { ref, watch, computed, onMounted, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'
import TierBadge from '@/components/clients/TierBadge.vue'
import AppTabModal from '@/components/AppTabModal.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { 
  ArrowLeft, 
  Plus, 
  Trash2, 
  Edit3,
  Calendar,
  Bot,
  Terminal,
  Hash,
  Activity,
  User,
  Search,
  X,
  Target,
  Cpu,
  Globe,
  Layout,
} from 'lucide-vue-next'
import ResourceSelector from '@/components/ResourceSelector.vue'
import SessionTimers from '@/components/SessionTimers.vue'
import HistoryLimitConfig from '@/components/HistoryLimitConfig.vue'

const WhatsAppIcon = (props: any) => h('svg', { 
  viewBox: "0 0 24 24", 
  fill: "currentColor",
  class: props.class 
}, [
  h('path', { d: "M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z" })
])

const route = useRoute()
const router = useRouter()
const api = useApi()
const clientID = route.params.id as string

const loading = ref(true)
const client = ref<any>(null)
const subscriptions = ref<any[]>([])
const workspaces = ref<any[]>([])
const bots = ref<any[]>([])
const availableChannels = ref<any[]>([])
const allKnownChannels = ref<any[]>([])

const workspaceSearch = ref('')
const botSearch = ref('')
const channelSearch = ref('')

const showWorkspaceResults = ref(false)
const showChannelResults = ref(false)
const showBotResults = ref(false)

const filteredWorkspaces = computed(() => {
    if (!workspaceSearch.value) return workspaces.value.slice(0, 3)
    const q = workspaceSearch.value.toLowerCase()
    return workspaces.value.filter(w => 
        w.name.toLowerCase().includes(q) || w.id.toLowerCase().includes(q)
    ).slice(0, 3)
})

const filteredBots = computed(() => {
    if (!botSearch.value) return bots.value.slice(0, 3)
    const q = botSearch.value.toLowerCase()
    return bots.value.filter(b => 
        b.name.toLowerCase().includes(q) || b.id.toLowerCase().includes(q)
    ).slice(0, 3)
})

const filteredChannels = computed(() => {
    if (!channelSearch.value) return availableChannels.value.slice(0, 3)
    const q = channelSearch.value.toLowerCase()
    return availableChannels.value.filter(c => 
        c.name.toLowerCase().includes(q) || 
        c.id.toLowerCase().includes(q)
    ).slice(0, 3)
})

function selectWorkspace(ws: any) {
    newSub.value.workspace_id = ws.id
    workspaceSearch.value = ''
    showWorkspaceResults.value = false
}

function selectChannel(ch: any) {
    newSub.value.channel_id = ch.id
    channelSearch.value = ''
    showChannelResults.value = false
}

function selectBot(bot: any) {
    newSub.value.custom_bot_id = bot.id
    botSearch.value = ''
    showBotResults.value = false
}

function getWorkspaceName(id: string) {
    return workspaces.value.find(w => w.id === id)?.name || id
}

function getChannelName(id: string) {
    return allKnownChannels.value.find(c => c.id === id)?.name || 
           availableChannels.value.find(c => c.id === id)?.name || 
           id
}

function getBotName(id: string) {
    if (!id) return 'Use Channel Bot (Default)'
    return bots.value.find(b => b.id === id)?.name || id
}

function getChannel(id: string) {
    return allKnownChannels.value.find(c => c.id === id) || 
           availableChannels.value.find(c => c.id === id)
}

function getChannelLogo(id: string) {
    const ch = getChannel(id)
    if (!ch) return null
    // Prioritize real profile picture from settings if synced
    if (ch.config?.settings?.profile_picture_url) {
        return ch.config.settings.profile_picture_url
    }
    return null
}

const showAddModal = ref(false)
const editingSub = ref<any>(null)
const newSub = ref({
  workspace_id: '',
  channel_id: '',
  custom_bot_id: '',
  custom_system_prompt: '',
  expires_at: '',
  session_timeout: 10, // User requested defaults for subscription
  inactivity_warning_time: 3,
  max_history_limit: null as number | null,
  max_recurring_reminders: 5 as number | null // Default to 5
})

watch(() => newSub.value.workspace_id, async (newID, oldID) => {
  if (newID) {
    // Only clear channel if we are changing workspace manually (not during initial edit load)
    if (oldID !== undefined && !editingSub.value) {
        newSub.value.channel_id = ''
    }
    
    try {
      const res = await api.get(`/workspaces/${newID}/channels`) as any
      availableChannels.value = res || []
      channelSearch.value = ''
    } catch (err) {
      console.error('Error loading channels:', err)
      availableChannels.value = []
    }
  } else {
    availableChannels.value = []
  }
})

const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'info' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

async function loadData() {
  loading.value = true
  try {
    const cRes = await api.get(`/clients/${clientID}`) as any
    client.value = cRes
    
    const sRes = await api.get(`/clients/${clientID}/subscriptions`) as any
    subscriptions.value = sRes.data || []
    
    const wRes = await api.get('/workspaces') as any
    workspaces.value = wRes || []

    const bRes = await api.get('/bots') as any
    bots.value = bRes.results || []

    // Pre-load channels for each workspace that has a subscription to ensure names resolve in the list
    // Load channels for ALL workspaces to build a global lookup map.
    // This is critical because subscriptions only have channel_id, not workspace_id.
    // We need this map to reverse-resolve the workspace when editing a subscription.
    for (const ws of workspaces.value) {
        try {
            const res = await api.get(`/workspaces/${ws.id}/channels`) as any
            const channels = res || []
            channels.forEach((c: any) => {
                 // Ensure we preserve the workspace_id in the channel object for reverse lookup
                 c.workspace_id = ws.id 
                 if (!allKnownChannels.value.find(existing => existing.id === c.id)) {
                     allKnownChannels.value.push(c)
                 }
            })
        } catch (e) {
            console.error(`Error loading channels for workspace ${ws.id}:`, e)
        }
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function createSubscription() {
  try {
    const payload = {
        ...newSub.value,
        expires_at: newSub.value.expires_at ? new Date(newSub.value.expires_at).toISOString() : null,
        clear_expires_at: !newSub.value.expires_at,
        session_timeout: newSub.value.session_timeout && newSub.value.session_timeout > 0 ? newSub.value.session_timeout : null,
        inactivity_warning_time: newSub.value.inactivity_warning_time && newSub.value.inactivity_warning_time > 0 ? newSub.value.inactivity_warning_time : null,
        clear_session_timeout: !newSub.value.session_timeout,
        clear_inactivity_warning: !newSub.value.inactivity_warning_time,
        max_history_limit: newSub.value.max_history_limit,
        clear_max_history_limit: newSub.value.max_history_limit === undefined || newSub.value.max_history_limit === null,
        max_recurring_reminders: newSub.value.max_recurring_reminders
    }

    if (editingSub.value) {
      await api.put(`/clients/${clientID}/subscriptions/${editingSub.value.id}`, payload)
    } else {
      await api.post(`/clients/${clientID}/subscriptions`, payload)
    }
    showAddModal.value = false
    await loadData()
    resetForm()
  } catch (err: any) {
    console.error(err)
    alert(err.response?.data?.error || 'Error al guardar la suscripción')
  }
}

async function deleteSubscription(subID: string) {
  confirmModal.value = {
      show: true,
      title: '¿Eliminar Suscripción?',
      message: 'Esta suscripción dejará de ser válida inmediatamente y el bot recuperará su configuración por defecto para este cliente.',
      type: 'danger',
      confirmText: 'Eliminar',
      onConfirm: async () => {
          try {
            await api.delete(`/clients/${clientID}/subscriptions/${subID}`)
            await loadData()
          } catch (err) {
            alert('Error al eliminar')
          }
      }
  }
}

const activeTab = ref('targeting')

async function openEditSub(sub: any) {
    editingSub.value = sub
    activeTab.value = 'targeting'
    
    // Resolve workspace ID from channel if not present in sub
    let wsID = sub.workspace_id
    if (!wsID) {
        const ch = getChannel(sub.channel_id)
        if (ch) wsID = ch.workspace_id
    }

    // Set initial values early to trigger UI cards
    newSub.value = {
        workspace_id: wsID || '',
        channel_id: sub.channel_id,
        custom_bot_id: sub.custom_bot_id || '',
        custom_system_prompt: sub.custom_system_prompt || '',
        expires_at: sub.expires_at ? sub.expires_at.split('T')[0] : '',
        session_timeout: sub.session_timeout || null,
        inactivity_warning_time: sub.inactivity_warning_time || null,
        max_history_limit: sub.max_history_limit !== undefined ? sub.max_history_limit : null,
        max_recurring_reminders: sub.max_recurring_reminders !== undefined ? sub.max_recurring_reminders : 5
    }

    workspaceSearch.value = ''
    botSearch.value = ''
    channelSearch.value = ''
    
    // Load specific channels for this workspace context
    if (newSub.value.workspace_id) {
        try {
            const res = await api.get(`/workspaces/${newSub.value.workspace_id}/channels`) as any
            availableChannels.value = res || []
            // Sync allKnownChannels with these results too
            availableChannels.value.forEach(c => {
                if (!allKnownChannels.value.find(existing => existing.id === c.id)) {
                    allKnownChannels.value.push(c)
                }
            })
        } catch (err) {
            console.error('Error loading channels for edit:', err)
        }
    }

    showAddModal.value = true
}

function resetForm() {
    editingSub.value = null
    activeTab.value = 'targeting'
    newSub.value = {
        workspace_id: '',
        channel_id: '',
        custom_bot_id: '',
        custom_system_prompt: '',
        expires_at: '',
        session_timeout: 10,
        inactivity_warning_time: 3,
        max_history_limit: null,
        max_recurring_reminders: 5
    }
    availableChannels.value = []
}

onMounted(loadData)
</script>

<template>
  <div v-if="loading" class="flex justify-center py-40">
    <span class="loading loading-ring loading-lg text-primary"></span>
  </div>

  <div v-else-if="client" class="space-y-12 animate-in fade-in duration-700 max-w-[1200px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-8 border-b border-white/5 mx-6 lg:mx-0">
      <div class="space-y-4 flex-1">
        <button @click="router.back()" class="flex items-center gap-2 text-slate-500 hover:text-primary transition-colors text-xs font-bold uppercase tracking-widest mb-4">
           <ArrowLeft class="w-4 h-4" /> Back to Clients
        </button>
        <div class="flex items-center gap-3">
          <User class="w-4 h-4 text-primary" />
          <span class="text-xs font-bold uppercase tracking-[0.25em] text-slate-500">Client Subscriptions</span>
        </div>
        <h2 class="text-4xl lg:text-5xl font-black tracking-tighter text-white uppercase leading-none">
          {{ client.display_name }}
          <TierBadge :tier="client.tier" class="ml-4 inline-block align-middle" />
        </h2>
      </div>
      
      <button class="btn-premium btn-premium-primary px-16 h-14 w-full lg:w-auto" @click="resetForm(); showAddModal = true">
           <Plus class="w-5 h-5 mr-2" />
           New Subscription
      </button>
    </div>

    <!-- Client Info Summary -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 px-6 lg:px-0">
        <div class="p-6 bg-[#161a23]/30 border border-white/5 rounded-3xl space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Platform ID</span>
            <div class="text-sm font-mono text-white">{{ client.platform_id }} ({{ client.platform_type }})</div>
        </div>
        <div class="p-6 bg-[#161a23]/30 border border-white/5 rounded-3xl space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Contact</span>
            <div class="text-sm text-slate-300 italic">{{ client.email || 'No email' }} • {{ client.phone || 'No phone' }}</div>
        </div>
        <div class="p-6 bg-[#161a23]/30 border border-white/5 rounded-3xl space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Active Subscriptions</span>
            <div class="text-xl font-black text-primary">{{ subscriptions.length }}</div>
        </div>
    </div>

    <!-- Subscriptions Table -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Manage Assigned Channels</div>
        
        <div v-if="subscriptions.length === 0" class="py-32 flex flex-col items-center justify-center bg-[#161a23]/20 rounded-[3rem] border border-dashed border-white/5">
             <Activity class="w-12 h-12 text-slate-700 mb-4 opacity-20" />
             <p class="text-sm font-bold text-slate-600 uppercase tracking-widest">No active subscriptions found</p>
        </div>

        <div v-else class="grid grid-cols-1 gap-6">
            <div v-for="sub in subscriptions" :key="sub.id" class="p-8 bg-[#161a23] border border-white/5 rounded-[2.5rem] flex flex-col lg:flex-row justify-between lg:items-center gap-8 group hover:border-primary/20 transition-all">
                <div class="flex items-center gap-6">
                    <div class="w-16 h-16 rounded-2xl bg-white/5 flex items-center justify-center text-primary border border-white/5 group-hover:bg-primary/10 transition-colors overflow-hidden">
                        <img v-if="getChannelLogo(sub.channel_id)" :src="getChannelLogo(sub.channel_id)" class="w-full h-full object-cover" />
                        <WhatsAppIcon v-else-if="getChannel(sub.channel_id)?.type === 'whatsapp'" class="w-8 h-8 text-[#25D366]" />
                        <Hash v-else class="w-8 h-8" />
                    </div>
                    <div>
                        <div class="flex items-center gap-2 mb-1">
                            <h4 class="text-xl font-black text-white uppercase tracking-tighter">Channel: {{ getChannelName(sub.channel_id) }}</h4>
                            <span class="badge badge-success badge-xs font-bold uppercase py-2">Active</span>
                        </div>
                        <p class="text-[10px] font-bold text-slate-400 uppercase tracking-[0.2em] flex items-center gap-2">
                            <Activity class="w-3 h-3 text-primary" /> UUID: {{ sub.id.substring(0,8) }}
                        </p>
                        <p class="text-[10px] font-bold text-slate-600 uppercase tracking-widest flex items-center gap-2 mt-1">
                            <Calendar class="w-3 h-3" /> Expires: {{ sub.expires_at ? new Date(sub.expires_at).toLocaleDateString() : 'Never' }}
                        </p>
                    </div>
                </div>

                <div class="flex-1 grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div class="storage-box-premium m-0 py-3">
                         <span class="text-xs font-black text-slate-600 uppercase block mb-1">Override Bot</span>
                         <span class="text-xs font-mono text-indigo-400">{{ getBotName(sub.custom_bot_id) }}</span>
                    </div>
                    <div v-if="sub.custom_system_prompt" class="storage-box-premium m-0 py-3">
                         <span class="text-xs font-black text-slate-600 uppercase block mb-1">Custom Prompt</span>
                         <span class="text-xs text-slate-400 italic line-clamp-1">Customized</span>
                    </div>
                </div>

                <div class="flex gap-2">
                    <button class="btn-card-action" @click="openEditSub(sub)">
                        <Edit3 class="w-5 h-5 text-primary" />
                    </button>
                    <button class="btn-card-action btn-card-action-red" @click="deleteSubscription(sub.id)">
                        <Trash2 class="w-5 h-5" />
                    </button>
                </div>
            </div>
        </div>
    </div>

    <!-- Add/Edit Subscription Modal -->
    <AppTabModal 
        v-model="showAddModal"
        :title="editingSub ? 'Protocol Manifest: Edit Subscription' : 'Protocol Manifest: New Subscription'"
        v-model:activeTab="activeTab"
        :tabs="[
            { id: 'targeting', label: 'Targeting', icon: Target },
            { id: 'intelligence', label: 'Intelligence', icon: Cpu }
        ]"
        :identity="editingSub ? {
            name: `Subscription: ${getChannelName(editingSub.channel_id)}`,
            id: editingSub.id,
            icon: getChannel(editingSub.channel_id)?.type === 'whatsapp' ? WhatsAppIcon : Activity,
            iconType: 'component',
            iconClass: getChannel(editingSub.channel_id)?.type === 'whatsapp' ? 'text-[#25D366]' : ''
        } : undefined"
        saveText="Deploy Subscription"
        :saveDisabled="!newSub.channel_id"
        @save="createSubscription"
        @cancel="showAddModal = false"
    >
        <!-- Tab: Targeting -->
        <div v-if="activeTab === 'targeting'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">Deployment Context</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Workspace and channel destination</p>
            </header>

            <!-- Workspace Selector -->
            <div class="form-control">
                <ResourceSelector
                    v-model="newSub.workspace_id"
                    :items="workspaces"
                    label="Target Workspace"
                    placeholder="Search workspace context..."
                    iconType="workspace"
                    resourceLabel="Context Validated"
                    color="primary"
                    @select="() => {
                        if (!editingSub) {
                            newSub.channel_id = ''
                        }
                    }"
                />
            </div>

            <!-- Channel Selector -->
            <div v-if="newSub.workspace_id" class="form-control animate-in fade-in slide-in-from-top-6">
                <ResourceSelector
                    v-model="newSub.channel_id"
                    :items="availableChannels"
                    label="Assigned Delivery Channel"
                    placeholder="Search active channel..."
                    iconType="channel"
                    resourceLabel="Signal Acquisition Ready"
                    color="primary"
                />
            </div>

            <div class="form-control">
                <label class="label-premium text-slate-400">Subscription Lifespan</label>
                <div class="relative group">
                    <Calendar class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-600 group-focus-within:text-primary transition-colors" />
                    <input v-model="newSub.expires_at" type="date" class="input-premium h-16 pl-14 w-full text-sm font-bold" />
                </div>
                <p class="text-xs text-slate-600 font-bold uppercase mt-2 tracking-widest">Automatic termination date for this protocol routing.</p>
            </div>
        </div>

        <!-- Tab: Intelligence -->
        <div v-if="activeTab === 'intelligence'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">AI Orchestration</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Provider and prompt specialization</p>
            </header>

            <!-- Bot Selector -->
            <div class="form-control">
                <ResourceSelector
                    v-model="newSub.custom_bot_id"
                    :items="bots"
                    label="Specialized Agent Override"
                    placeholder="Search intelligence profiles..."
                    iconType="bot"
                    resourceLabel="Override Active"
                    :nullable="true"
                    color="indigo"
                />
                <p class="text-xs text-slate-600 font-bold uppercase mt-2 tracking-[0.2em]">Defaults to channel configuration if signal is null.</p>
            </div>

             <!-- Session Override Section -->
             <div class="py-2">
                <div class="flex justify-between items-end mb-2 px-1">
                    <span class="text-xs text-slate-500 font-bold uppercase tracking-widest">Session Configuration</span>
                    <span class="text-xs text-primary/60 font-mono font-bold uppercase tracking-tight">Standard: 10m Duration / 3m Warning</span>
                </div>
                <SessionTimers 
                    v-model:timeout="newSub.session_timeout"
                    v-model:warning="newSub.inactivity_warning_time"
                    :isOverride="true"
                    :inheritedTimeout="getChannel(newSub.channel_id)?.config?.session_timeout || 4"
                    :inheritedWarning="getChannel(newSub.channel_id)?.config?.inactivity_warning_time || 3"
                />
             </div>

             <div class="py-2">
                <HistoryLimitConfig 
                    v-model="newSub.max_history_limit" 
                    :isOverride="true"
                />
             </div>

             <div class="form-control">
                <label class="label-premium text-slate-400">Max Recurring Reminders</label>
                <div class="relative group">
                    <Calendar class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-600 group-focus-within:text-primary transition-colors" />
                    <input 
                        v-model.number="newSub.max_recurring_reminders" 
                        type="number" 
                        min="0" 
                        max="20"
                        class="input-premium h-16 pl-14 w-full text-sm font-bold" 
                        placeholder="Limit (Default 5)"
                    />
                </div>
                <p class="text-xs text-slate-600 font-bold uppercase mt-2 tracking-widest">Maximum number of active recurring reminders allowed (Max 20).</p>
             </div>

            <div class="form-control">
                <label class="label-premium text-slate-400">Context Augmentation (System Prompt)</label>
                <div class="relative group">
                    <Terminal class="absolute left-6 top-6 w-5 h-5 text-slate-700 group-focus-within:text-primary transition-colors" />
                    <textarea 
                        v-model="newSub.custom_system_prompt" 
                        class="input-premium h-48 pl-14 pt-5 w-full text-xs leading-relaxed resize-none" 
                        placeholder="Additional instruction layers for the AI engine specifically for this client/canal nexus..."
                    ></textarea>
                </div>
            </div>
        </div>
    </AppTabModal>

    <ConfirmationDialog 
        v-model="confirmModal.show"
        :title="confirmModal.title"
        :message="confirmModal.message"
        :type="confirmModal.type"
        :confirmText="confirmModal.confirmText"
        @confirm="confirmModal.onConfirm"
    />
  </div>
</template>
