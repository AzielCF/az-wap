<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AppTabModal from '@/components/AppTabModal.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import TierBadge from '@/components/clients/TierBadge.vue'
import { 
  Plus, 
  Trash2, 
  Edit3, 
  CheckCircle2, 
  Users,
  RefreshCw,
  Search,
  Tag,
  Link2,
  Eye,
  MessageSquare,
  Globe,
  Layout,
  Contact,
  ShieldCheck,
  Bot
} from 'lucide-vue-next'

interface Client {
  id: string
  platform_id: string
  platform_type: string
  display_name: string
  email: string
  phone: string
  tier: string
  tags: string[]
  metadata: Record<string, any>
  notes: string
  language: string
  allowed_bots: string[]
  enabled: boolean
  created_at: string
}

const api = useApi()
const loading = ref(true)
const clients = ref<Client[]>([])
const search = ref('')
const stats = ref<Record<string, number>>({})
const bots = ref<any[]>([])

const filteredClients = computed(() => {
  if (!search.value) return clients.value
  const s = search.value.toLowerCase()
  return clients.value.filter(c => 
    c.display_name?.toLowerCase().includes(s) || 
    c.platform_id?.toLowerCase().includes(s) ||
    c.email?.toLowerCase().includes(s) ||
    c.tags?.some(t => t.toLowerCase().includes(s))
  )
})


const showAddModal = ref(false)
const editingClient = ref<Client | null>(null)
const newClient = ref({
  platform_id: '',
  platform_type: 'whatsapp',
  display_name: '',
  email: '',
  phone: '',
  tier: 'standard',
  tags: [] as string[],
  notes: '',
  language: 'en',
  allowed_bots: [] as string[]
})
const newTag = ref('')
const workspaces = ref<any[]>([])
const validatingLid = ref(false)
const lidVerified = ref(false)
const botSearch = ref('')
const showBotResults = ref(false)
const activeClientTab = ref('identity')

const unselectedBots = computed(() => {
  return bots.value.filter(b => !newClient.value.allowed_bots.includes(b.id))
})

const filteredBots = computed(() => {
  if (!botSearch.value) return unselectedBots.value.slice(0, 3)
  const s = botSearch.value.toLowerCase()
  return unselectedBots.value.filter(b => 
    b.name.toLowerCase().includes(s) || 
    b.id.toLowerCase().includes(s)
  ).slice(0, 3)
})

function addAllowedBot(botId: string) {
  if (!newClient.value.allowed_bots.includes(botId)) {
    newClient.value.allowed_bots.push(botId)
  }
  botSearch.value = ''
  showBotResults.value = false
}

function removeAllowedBot(botId: string) {
  newClient.value.allowed_bots = newClient.value.allowed_bots.filter(id => id !== botId)
}

function getBotById(id: string) {
  return bots.value.find(b => b.id === id) || { name: 'Unknown Bot', id }
}

async function loadWorkspaces() {
  try {
    const res = await api.get('/workspaces') as any
    const wsList = res || []
    
    // Fetch channels for each workspace to populate the list for validation
    const channelsPromises = wsList.map((ws: any) => api.get(`/workspaces/${ws.id}/channels`))
    const channelsResults = await Promise.allSettled(channelsPromises)
    
    workspaces.value = wsList.map((ws: any, idx: number) => {
      const res = channelsResults[idx]
      let channels: any[] = []
      if (res && res.status === 'fulfilled') {
        channels = (res as PromiseFulfilledResult<any>).value || []
      }
      return {
        ...ws,
        channels: channels
      }
    })
  } catch (err) {
    console.error('Failed to load workspaces or channels:', err)
  }
}

async function extractLid() {
  if (!newClient.value.phone) {
    alert('Please enter a phone number first')
    return
  }

  // Find the first available whatsapp channel (enabled or connected)
  let targetWorkspace = null
  let targetChannel = null

  for (const ws of workspaces.value) {
    const ch = ws.channels?.find((c: any) => (c.type === 'whatsapp' || c.platform_type === 'whatsapp') && (c.enabled || c.status === 'connected'))
    if (ch) {
      targetWorkspace = ws
      targetChannel = ch
      break
    }
  }

  if (!targetChannel) {
    alert('No active WhatsApp channel found (Enabled or Connected) to perform validation.')
    return
  }

  validatingLid.value = true
  try {
    const res = await api.get(`/workspaces/${targetWorkspace.id}/channels/${targetChannel.id}/resolve-identity?identity=${newClient.value.phone}`) as any
    if (res.resolved_identity) {
      const parts = res.resolved_identity.split('|')
      const lidPart = parts.find((p: string) => p.includes('@lid'))
      
      if (lidPart) {
        newClient.value.platform_id = lidPart
        lidVerified.value = true
      } else {
        // Fallback if no explicit @lid is found
        newClient.value.platform_id = parts[0]
      }
      
      // If display name is empty, use the contact name
      if (!newClient.value.display_name && res.name) {
        newClient.value.display_name = res.name
      }
    }
  } catch (err) {
    alert('Could not resolve LID. Make sure the channel is connected.')
  } finally {
    validatingLid.value = false
  }
}

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
    const res = await api.get('/clients') as any
    clients.value = res.data || []
    
    // Load bots for selector
    const botsRes = await api.get('/bots') as any
    bots.value = botsRes.results || []

    // Load stats
    const statsRes = await api.get('/clients/stats') as any
    stats.value = statsRes.by_tier || {}
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingClient.value = null
  newClient.value = {
    platform_id: '',
    platform_type: 'whatsapp',
    display_name: '',
    email: '',
    phone: '',
    tier: 'standard',
    tags: [],
    notes: '',
    language: 'en',
    allowed_bots: []
  }
  newTag.value = ''
}

function openEdit(client: Client) {
  editingClient.value = client
  newClient.value = {
    platform_id: client.platform_id,
    platform_type: client.platform_type,
    display_name: client.display_name,
    email: client.email || '',
    phone: client.phone || '',
    tier: client.tier,
    tags: client.tags || [],
    notes: client.notes || '',
    language: client.language || 'en',
    allowed_bots: client.allowed_bots || []
  }
  showAddModal.value = true
}

function addTag() {
  if (newTag.value && !newClient.value.tags.includes(newTag.value)) {
    newClient.value.tags.push(newTag.value)
    newTag.value = ''
  }
}

function removeTag(tag: string) {
  newClient.value.tags = newClient.value.tags.filter(t => t !== tag)
}

async function saveClient() {
  try {
    if (editingClient.value) {
      await api.put(`/clients/${editingClient.value.id}`, newClient.value)
    } else {
      await api.post('/clients', newClient.value)
    }
    showAddModal.value = false
    resetForm()
    await loadData()
  } catch (err) {
    alert('Error saving client.')
  }
}

async function deleteClient(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Delete Client?',
      message: 'This action will permanently delete the client and all their subscriptions. Data cannot be recovered.',
      type: 'danger',
      confirmText: 'Delete',
      onConfirm: async () => {
          try {
            await api.delete(`/clients/${id}`)
            await loadData()
          } catch (err) {
            alert('Error deleting client.')
          }
      }
  }
}

async function toggleEnabled(client: Client) {
  try {
    if (client.enabled) {
      await api.put(`/clients/${client.id}/disable`, {})
    } else {
      await api.put(`/clients/${client.id}/enable`, {})
    }
    await loadData()
  } catch (err) {
    console.error(err)
  }
}

onMounted(() => {
  loadData()
  loadWorkspaces()
})
</script>

<template>
  <div v-if="loading" class="flex justify-center py-40">
    <span class="loading loading-ring loading-lg text-primary"></span>
  </div>

  <div v-else class="space-y-12 animate-in fade-in duration-700 max-w-[1400px] mx-auto pb-20">
    <!-- Header -->
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-8 border-b border-white/5 mx-6 lg:mx-0">
      <div class="space-y-4 flex-1">
        <div class="flex items-center gap-3">
          <Users class="w-4 h-4 text-primary" />
          <span class="section-title-premium py-0 border-none pl-0 text-primary">Customer Hub</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-xs font-bold uppercase tracking-[0.25em] text-slate-500">Global Registry</span>
        </div>
        <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none">Clients</h2>
      </div>
      
      <div class="flex flex-col lg:flex-row gap-4">
        <div class="relative group">
            <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-600 group-focus-within:text-primary transition-colors" />
            <input v-model="search" type="text" placeholder="Search clients..." class="input-premium h-14 pl-12 w-64 text-sm" />
        </div>
        <button class="btn-premium btn-premium-ghost px-10 h-14 w-full lg:w-auto" @click="loadData" :class="{ loading: loading }">
             <RefreshCw v-if="!loading" class="w-4 h-4 mr-2" />
             Refresh
        </button>
        <button class="btn-premium btn-premium-primary px-16 h-14 w-full lg:w-auto" @click="showAddModal = true; resetForm()">
             <Plus class="w-5 h-5 mr-2" />
             New Client
        </button>
      </div>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-6 px-6 lg:px-0">
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Total</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ clients.length }}</span>
                <span class="text-xs font-bold text-primary uppercase">Clients</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">VIP</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ stats['vip'] || 0 }}</span>
                <span class="text-xs font-bold text-amber-500 uppercase">Crown</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Premium</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ stats['premium'] || 0 }}</span>
                <span class="text-xs font-bold text-indigo-400 uppercase">Stars</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Enterprise</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ stats['enterprise'] || 0 }}</span>
                <span class="text-xs font-bold text-rose-400 uppercase">Accounts</span>
            </div>
        </div>
    </div>

    <!-- Client List -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Registered Clients</div>

        <div v-if="filteredClients.length === 0" class="py-40 flex flex-col items-center justify-center bg-[#161a23]/20 rounded-[3rem] border border-dashed border-white/5">
            <div class="w-20 h-20 rounded-full bg-white/5 flex items-center justify-center text-slate-700 mb-6">
                <Users class="w-8 h-8 opacity-20" />
            </div>
            <p class="text-sm font-bold text-slate-600 uppercase tracking-[0.2em]">No clients found</p>
            <button v-if="search" @click="search = ''" class="mt-4 text-xs font-black text-primary uppercase tracking-widest hover:underline">Clear search</button>
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
            <div v-for="client in filteredClients" :key="client.id" class="card-premium" :class="{ 'opacity-50': !client.enabled }">
                <div class="relative z-10 flex flex-col h-full">
                    <div class="flex justify-between items-start mb-8">
                        <div class="icon-box-premium"
                             :class="client.tier === 'vip' ? 'icon-box-amber' : client.tier === 'premium' ? 'icon-box-primary' : client.tier === 'enterprise' ? 'icon-box-rose' : 'icon-box-slate'">
                            <span class="text-xl font-black opacity-40">{{ client.display_name?.charAt(0) || 'C' }}</span>
                        </div>
                        <div class="flex gap-2">
                            <button class="btn-card-action" @click="openEdit(client)" title="Edit">
                                <Edit3 class="w-4 h-4" />
                            </button>
                            <button class="btn-card-action" @click="toggleEnabled(client)" :title="client.enabled ? 'Disable' : 'Enable'">
                                <Eye class="w-4 h-4" :class="client.enabled ? 'text-success' : 'text-slate-600'" />
                            </button>
                            <button class="btn-card-action btn-card-action-red" @click="deleteClient(client.id)" title="Delete">
                                <Trash2 class="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    
                    <h4 class="text-2xl font-black text-white uppercase tracking-tighter mb-2 group-hover:text-primary transition-colors leading-none truncate">
                      {{ client.display_name || 'No name' }}
                    </h4>
                    <div class="flex items-center gap-3 mb-4">
                        <TierBadge :tier="client.tier" />
                        <div class="w-1 h-1 rounded-full bg-slate-800"></div>
                        <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">{{ client.platform_type }}</span>
                    </div>

                    <div class="space-y-4 mt-auto">
                        <div class="storage-box-premium">
                            <div class="flex items-baseline justify-between mb-1">
                                <span class="text-[10px] font-bold text-slate-600 uppercase">Platform ID</span>
                                <MessageSquare class="w-3 h-3 text-slate-800" />
                            </div>
                            <div class="font-mono text-xs text-slate-400 break-all leading-relaxed tracking-tight truncate">
                                {{ client.platform_id }}
                            </div>
                        </div>

                        <div v-if="client.tags?.length" class="flex flex-wrap gap-2">
                            <span v-for="tag in client.tags.slice(0, 3)" :key="tag" 
                                  class="px-2 py-1 text-[9px] font-bold uppercase tracking-wider bg-white/5 text-slate-400 rounded-lg">
                                #{{ tag }}
                            </span>
                            <span v-if="client.tags.length > 3" class="px-2 py-1 text-[9px] font-bold text-slate-600">
                                +{{ client.tags.length - 3 }}
                            </span>
                        </div>
                        
                        <div class="flex items-center justify-between pt-4">
                            <div class="flex items-center gap-2">
                                <div class="w-1.5 h-1.5 rounded-full" :class="client.enabled ? 'bg-primary animate-pulse' : 'bg-slate-700'"></div>
                                <span class="text-[10px] font-black text-slate-500 uppercase tracking-widest">
                                  {{ client.enabled ? 'Active' : 'Inactive' }}
                                </span>
                            </div>
                            <router-link :to="`/clients/${client.id}/subscriptions`" class="text-[10px] font-black text-primary uppercase tracking-widest hover:underline flex items-center gap-1">
                                <Link2 class="w-3 h-3" /> Subscriptions
                            </router-link>
                        </div>
                    </div>
                </div>
                <div class="absolute -bottom-10 -right-10 w-40 h-40 rounded-full blur-[60px] transition-colors duration-700"
                     :class="client.tier === 'vip' ? 'bg-amber-500/5 group-hover:bg-amber-500/10' : 'bg-primary/5 group-hover:bg-primary/10'"></div>
            </div>
        </div>
    </div>

    <!-- Add/Edit Client Modal -->
    <AppTabModal 
        v-model="showAddModal"
        :title="editingClient ? 'Control Manifest: Edit Client' : 'Control Manifest: New Entry'"
        v-model:activeTab="activeClientTab"
        :tabs="[
            { id: 'identity', label: 'Identity', icon: ShieldCheck },
            { id: 'personal', label: 'Contact Details', icon: Contact },
            { id: 'settings', label: 'Configuration', icon: Layout },
            { id: 'operational', label: 'Authorization', icon: Bot }
        ]"
        :identity="editingClient ? {
            name: (newClient.display_name || 'Anonymous Object'),
            id: newClient.platform_id,
            icon: Users,
            iconType: 'component'
        } : undefined"
        saveText="Commit Changes"
        @save="saveClient"
        @cancel="showAddModal = false"
    >
        <!-- Tab: Identity -->
        <div v-if="activeClientTab === 'identity'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">Signal Identification</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Core platform parameters and access tier</p>
            </header>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
                <div class="form-control">
                    <label class="label-premium text-slate-400">Platform ID (LID/JID)</label>
                    <div class="flex gap-2">
                        <input v-model="newClient.platform_id" type="text" class="input-premium h-14 flex-1 text-sm font-mono" placeholder="Identification signal..." />
                        <button v-if="newClient.platform_type === 'whatsapp'" 
                                type="button"
                                class="btn-premium px-6 h-14" 
                                :class="lidVerified ? 'btn-premium-ghost text-green-500 border-green-500/20' : 'btn-premium-ghost'"
                                @click="extractLid"
                                :disabled="validatingLid">
                            <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': validatingLid }" />
                        </button>
                    </div>
                    <p class="text-[9px] text-slate-600 font-bold uppercase mt-2">Unique identifier for the selected platform.</p>
                </div>

                <div class="form-control">
                    <label class="label-premium text-slate-400">Target Platform</label>
                    <select v-model="newClient.platform_type" class="input-premium h-14 w-full text-sm" :disabled="!!editingClient">
                        <option value="whatsapp">WhatsApp Protocol</option>
                        <option value="telegram">Telegram Bot API</option>
                        <option value="webchat">Internal WebChat</option>
                    </select>
                </div>
            </div>

            <div class="form-control">
                <label class="label-premium text-slate-400">Authorization Tier</label>
                <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
                    <button v-for="t in ['standard', 'premium', 'vip', 'enterprise']" :key="t"
                            @click="newClient.tier = t" 
                            class="h-16 rounded-2xl border-2 transition-all flex items-center justify-center gap-2 cursor-pointer hover:scale-[1.02]"
                            :class="newClient.tier === t ? 'border-primary bg-primary/10 shadow-lg' : 'border-white/5 bg-black/40 text-slate-500'">
                        <TierBadge :tier="t" :show-icon="true" />
                    </button>
                </div>
            </div>
        </div>

        <!-- Tab: Personal -->
        <div v-if="activeClientTab === 'personal'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">Contact Profile</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Naming and communication vectors</p>
            </header>

            <div class="form-control">
                <label class="label-premium text-slate-400">Display Name / Alias</label>
                <input v-model="newClient.display_name" type="text" class="input-premium h-14 w-full text-lg font-black" placeholder="Contact Name" />
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
                <div class="form-control">
                    <label class="label-premium text-slate-400">Primary Email</label>
                    <input v-model="newClient.email" type="email" class="input-premium h-14 w-full text-sm" placeholder="contact@domain.com" />
                </div>
                <div class="form-control">
                    <label class="label-premium text-slate-400">Universal Phone</label>
                    <input v-model="newClient.phone" type="tel" class="input-premium h-14 w-full text-sm font-mono" placeholder="+XX XXX XXX XXX" />
                </div>
            </div>

            <div class="form-control">
                <label class="label-premium text-slate-400">Categorization Tags</label>
                <div class="flex gap-2 mb-4">
                    <input v-model="newTag" type="text" class="input-premium h-12 flex-1 text-sm bg-black/40" placeholder="Add custom tag..." @keyup.enter="addTag" />
                    <button @click="addTag" class="btn-premium btn-premium-ghost h-12 px-6">
                        <Tag class="w-4 h-4" />
                    </button>
                </div>
                <div class="flex flex-wrap gap-2 p-4 bg-black/40 rounded-2xl border border-white/5 min-h-[60px]">
                    <span v-for="tag in newClient.tags" :key="tag" 
                          class="px-4 py-2 text-[10px] font-black uppercase tracking-widest bg-primary/10 text-primary border border-primary/20 rounded-xl flex items-center gap-3">
                        {{ tag }}
                        <button @click="removeTag(tag)" class="hover:text-white transition-colors">&times;</button>
                    </span>
                    <p v-if="newClient.tags.length === 0" class="text-[10px] text-slate-700 font-bold uppercase items-center flex">No tags assigned / global pool</p>
                </div>
            </div>
        </div>

        <!-- Tab: Settings -->
        <div v-if="activeClientTab === 'settings'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">Vibe & Parameters</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Linguistic and internal processing notes</p>
            </header>

            <div class="form-control max-w-sm">
                <label class="label-premium text-slate-400">Primary AI Language</label>
                <div class="relative group">
                    <div class="absolute left-4 top-1/2 -translate-y-1/2 text-primary opacity-40 group-focus-within:opacity-100 transition-opacity">
                        <Globe class="w-5 h-5" />
                    </div>
                    <select v-model="newClient.language" class="input-premium h-14 pl-12 w-full text-sm font-bold">
                        <option value="es">Español (Castellano)</option>
                        <option value="en">English (International)</option>
                        <option value="pt">Português</option>
                        <option value="fr">Français</option>
                        <option value="it">Italiano</option>
                        <option value="de">Deutsch</option>
                    </select>
                </div>
            </div>

            <div class="form-control">
                <label class="label-premium text-slate-400">Internal Context Manifest</label>
                <textarea v-model="newClient.notes" class="input-premium w-full text-sm min-h-[220px] p-6 resize-none leading-relaxed" placeholder="Detailed profile context, specific rules or historical patterns for this client..."></textarea>
            </div>
        </div>

        <!-- Tab: Operational -->
        <div v-if="activeClientTab === 'operational'" class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300 h-full flex flex-col">
            <header>
                <h3 class="text-xl font-black text-white uppercase tracking-tight">Authorization Roster</h3>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Strict restriction of available Agent Instances</p>
            </header>

            <div class="form-control">
                <div class="relative">
                    <div class="absolute left-4 top-1/2 -translate-y-1/2 text-slate-600">
                        <Search class="w-4 h-4" />
                    </div>
                    <input 
                        v-model="botSearch" 
                        @focus="showBotResults = true"
                        type="text" 
                        class="input-premium h-14 pl-12 w-full text-base font-medium placeholder:text-slate-700 bg-black/40" 
                        placeholder="Search for authorized agents..." 
                    />
                    
                    <!-- Search Results Dropdown -->
                    <div v-if="showBotResults && (botSearch || filteredBots.length > 0)" 
                            class="absolute z-20 top-full left-0 right-0 mt-2 bg-[#12161f] border border-white/10 rounded-2xl shadow-2xl max-h-64 overflow-y-auto p-2">
                        <div v-if="filteredBots.length === 0" class="py-4 text-center text-[10px] text-slate-600 font-black uppercase">Zero matching agents</div>
                        <button v-for="bot in filteredBots" :key="bot.id"
                                @click="addAllowedBot(bot.id)"
                                class="w-full flex items-center gap-4 p-3 hover:bg-primary/10 rounded-xl transition-all group text-left border border-transparent hover:border-primary/20">
                            <div class="w-10 h-10 rounded-lg bg-black flex items-center justify-center text-slate-500 group-hover:text-primary transition-all shadow-inner">
                                <Bot class="w-5 h-5" />
                            </div>
                            <div class="flex-1 min-w-0">
                                <div class="text-[11px] font-black uppercase text-white truncate">{{ bot.name }}</div>
                                <div class="text-[9px] font-mono text-slate-600 truncate uppercase">{{ bot.id.substring(0,25) }}...</div>
                            </div>
                            <Plus class="w-4 h-4 text-slate-800 group-hover:text-primary" />
                        </button>
                    </div>
                </div>
                <div v-if="showBotResults" @click="showBotResults = false" class="fixed inset-0 z-10"></div>
            </div>

            <div class="flex-1 min-h-0 flex flex-col">
                <div class="flex items-center justify-between mb-4">
                    <span class="text-[10px] font-black text-slate-600 uppercase tracking-widest">Currently Whitelisted</span>
                    <span v-if="newClient.allowed_bots.length === 0" class="text-[9px] font-bold text-slate-500 uppercase italic">Unrestricted Global Access</span>
                </div>
                
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pb-20">
                    <div v-for="botId in newClient.allowed_bots" :key="botId" 
                            class="flex items-center justify-between p-4 bg-primary/5 border border-primary/10 rounded-2xl group border-l-4 border-l-primary/40 hover:bg-primary/[0.08] transition-all">
                        <div class="flex items-center gap-4 min-w-0">
                            <div class="w-10 h-10 rounded-xl bg-[#0b0e14] border border-white/5 flex items-center justify-center text-primary shadow-xl">
                                <Bot class="w-5 h-5" />
                            </div>
                            <div class="min-w-0">
                                <div class="text-[11px] font-black uppercase text-white truncate">{{ getBotById(botId).name }}</div>
                                <div class="text-[9px] font-mono text-slate-500 truncate">{{ botId.substring(0,12) }}...</div>
                            </div>
                        </div>
                        <button @click="removeAllowedBot(botId)" class="p-2 text-slate-700 hover:text-red-500 transition-colors">
                            <Trash2 class="w-4 h-4" />
                        </button>
                    </div>
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


