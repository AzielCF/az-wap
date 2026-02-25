<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import AppPageHeader from '@/components/AppPageHeader.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import TierBadge from '@/components/clients/TierBadge.vue'
import ClientEditorModal from '@/components/clients/ClientEditorModal.vue'
import { 
  Plus, 
  Trash2, 
  Edit3, 
  Users,
  RefreshCw,
  Search,
  Link2,
  Eye,
  MessageSquare,
  Key,
  ShieldCheck
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
  timezone: string
  country: string
  allowed_bots: string[]
  owned_channels: string[]
  is_tester: boolean
  enabled: boolean
  created_at: string
}

const api = useApi()
const loading = ref(true)
const clients = ref<Client[]>([])
const search = ref('')
const stats = ref<Record<string, number>>({})
const portalAccounts = ref<Record<string, any>>({})

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
    
    // Load bots for selector only on demand, not in grid!

    // Load stats
    const statsRes = await api.get('/clients/stats') as any
    stats.value = statsRes.by_tier || {}

    // Load portal account status for current page clients
    try {
      if (clients.value.length > 0) {
        const ids = clients.value.map(c => c.id).join(',')
        portalAccounts.value = await api.get(`/internal/portal-accounts?ids=${ids}`) as Record<string, any>
      }
    } catch (e) {
      console.warn('Could not load portal account status', e)
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingClient.value = null
}

function openEdit(client: Client) {
  editingClient.value = client
  showAddModal.value = true
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

async function provisionPortalAccount(client: Client) {
  confirmModal.value = {
    show: true,
    title: 'Enable Portal Access?',
    message: `This will enable portal access for ${client.display_name}. ${client.email ? 'It will be linked to ' + client.email : 'The client can register their email later'}.`,
    type: 'info',
    confirmText: 'Enable Portal',
    onConfirm: async () => {
      try {
        await api.post(`/internal/clients/${client.id}/portal-account`, {
          email: client.email,
          full_name: client.display_name
        })
        alert('Portal account enabled successfully.')
      } catch (err: any) {
        const msg = err.response?.data?.error || err.message || 'Unknown error'
        alert('Error: ' + msg)
      }
    }
  }
}



onMounted(() => {
  loadData()
  // Workspaces y canales NO se cargan en la vista principal
})
</script>

<template>
  <div v-if="loading" class="flex justify-center py-40">
    <span class="loading loading-ring loading-lg text-primary"></span>
  </div>

  <div v-else class="space-y-12 animate-in fade-in duration-700 max-w-[1400px] mx-auto pb-20">
    <!-- Header -->
    <AppPageHeader title="Clients">
      <template #breadcrumb>
          <Users class="w-4 h-4 text-primary shrink-0" />
          <span class="text-sm font-bold uppercase tracking-widest text-primary">Clients</span>
          <span class="opacity-30 text-xs font-black text-slate-500">/</span>
          <span class="text-xs font-bold uppercase tracking-widest text-slate-500">Registry</span>
      </template>

      <template #actions>
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
      </template>
    </AppPageHeader>

    <!-- Stats -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-6 px-6 lg:px-0">
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-xs font-bold text-slate-600 uppercase tracking-widest">Total</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ clients.length }}</span>
                <span class="text-xs font-bold text-primary uppercase">Clients</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-xs font-bold text-slate-600 uppercase tracking-widest">VIP</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ stats['vip'] || 0 }}</span>
                <span class="text-xs font-bold text-amber-500 uppercase">Crown</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-xs font-bold text-slate-600 uppercase tracking-widest">Premium</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ stats['premium'] || 0 }}</span>
                <span class="text-xs font-bold text-indigo-400 uppercase">Stars</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-xs font-bold text-slate-600 uppercase tracking-widest">Enterprise</span>
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
            <p class="text-sm font-bold text-slate-600 uppercase tracking-widest">No clients found</p>
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
                            <button v-if="!portalAccounts[client.id]" class="btn-card-action" @click="provisionPortalAccount(client)" title="Provision Portal Access">
                                <Key class="w-4 h-4 text-primary" />
                            </button>
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
                        <span class="text-xs font-bold text-slate-600 uppercase tracking-widest">{{ client.platform_type }}</span>
                        <!-- Tester Badge -->
                        <div v-if="client.is_tester" class="ml-auto px-2 py-0.5 rounded bg-amber-500/10 border border-amber-500/20 text-amber-500 text-xs font-black uppercase tracking-widest flex items-center gap-1 shadow-[0_0_10px_rgba(245,158,11,0.1)] animate-pulse scale-90 origin-right">
                            <ShieldCheck class="w-3 h-3" /> TESTER
                        </div>
                    </div>

                    <!-- Portal Status Indicator -->
                    <div class="mb-6 flex items-center gap-2">
                      <div v-if="!portalAccounts[client.id]" class="px-3 py-1 rounded-full bg-slate-800/50 border border-white/5 text-xs font-bold text-slate-500 uppercase tracking-widest">
                        Portal: Inactive
                      </div>
                      <div v-else-if="portalAccounts[client.id].is_shadow" class="px-3 py-1 rounded-full bg-amber-500/10 border border-amber-500/20 text-xs font-black text-amber-500 uppercase tracking-widest flex items-center gap-1.5">
                        <div class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse"></div>
                        Portal: Pending
                      </div>
                      <div v-else class="px-3 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-xs font-black text-green-500 uppercase tracking-widest flex items-center gap-1.5">
                        <div class="w-1.5 h-1.5 rounded-full bg-green-500"></div>
                        Portal: Active
                      </div>

                      <span v-if="portalAccounts[client.id]?.email" class="text-xs font-bold text-slate-600 truncate max-w-[120px]">
                        {{ portalAccounts[client.id].email }}
                      </span>
                    </div>

                    <div class="space-y-4 mt-auto">
                        <div class="storage-box-premium">
                            <div class="flex items-baseline justify-between mb-1">
                                <span class="text-xs font-bold text-slate-600 uppercase">Platform ID</span>
                                <MessageSquare class="w-3 h-3 text-slate-800" />
                            </div>
                            <div class="font-mono text-xs text-slate-400 break-all leading-relaxed tracking-tight truncate">
                                {{ client.platform_id }}
                            </div>
                        </div>

                        <div v-if="client.tags?.length" class="flex flex-wrap gap-2">
                            <span v-for="tag in client.tags.slice(0, 3)" :key="tag" 
                                  class="px-2 py-1 text-xs font-bold uppercase tracking-wider bg-white/5 text-slate-400 rounded-lg">
                                #{{ tag }}
                            </span>
                            <span v-if="client.tags.length > 3" class="px-2 py-1 text-xs font-bold text-slate-600">
                                +{{ client.tags.length - 3 }}
                            </span>
                        </div>

                        <div class="flex items-center justify-between pt-4">
                            <div class="flex items-center gap-2">
                                <div class="w-1.5 h-1.5 rounded-full" :class="client.enabled ? 'bg-primary animate-pulse' : 'bg-slate-700'"></div>
                                <span class="text-xs font-black text-slate-500 uppercase tracking-widest">
                                  {{ client.enabled ? 'Active' : 'Inactive' }}
                                </span>
                            </div>
                            <router-link :to="`/clients/${client.id}/subscriptions`" class="text-xs font-black text-primary uppercase tracking-widest hover:underline flex items-center gap-1">
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
    <ClientEditorModal 
        v-model:show="showAddModal"
        :clientToEdit="editingClient"
        @saved="loadData"
        @closed="resetForm"
    />

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


