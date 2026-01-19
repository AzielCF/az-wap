<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AppModal from '@/components/AppModal.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { 
  Plus, 
  Trash2, 
  Edit3, 
  CheckCircle2, 
  Key, 
  ShieldCheck, 
  Lock,
  Globe,
  HardDrive,
  Cpu,
  Search,
} from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const credentials = ref<any[]>([])
const search = ref('')

const aiKinds = ['ai', 'gemini', 'openai', 'claude']

const filteredCredentials = computed(() => {
  if (!search.value) return credentials.value
  const s = search.value.toLowerCase()
  return credentials.value.filter(c => 
    c.name.toLowerCase().includes(s) || 
    c.kind.toLowerCase().includes(s)
  )
})

const isAI = (kind: string) => aiKinds.includes(kind)

const showAddModal = ref(false)
const editingCredential = ref<any>(null)
const newCredential = ref({
  name: '',
  kind: 'ai' as 'ai' | 'chatwoot',
  ai_api_key: '',
  chatwoot_base_url: '',
  chatwoot_account_token: '',
  chatwoot_bot_token: ''
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
    const res = await api.get('/credentials') as any
    credentials.value = res.results || []
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingCredential.value = null
  newCredential.value = {
    name: '',
    kind: 'ai',
    ai_api_key: '',
    chatwoot_base_url: '',
    chatwoot_account_token: '',
    chatwoot_bot_token: ''
  }
}

function openEdit(cred: any) {
  editingCredential.value = cred
  newCredential.value = {
    name: cred.name,
    kind: cred.kind,
    ai_api_key: cred.ai_api_key || '',
    chatwoot_base_url: cred.chatwoot_base_url || '',
    chatwoot_account_token: cred.chatwoot_account_token || '',
    chatwoot_bot_token: cred.chatwoot_bot_token || ''
  }
  showAddModal.value = true
}

async function saveCredential() {
  try {
    if (editingCredential.value) {
      await api.put(`/credentials/${editingCredential.value.id}`, newCredential.value)
    } else {
      await api.post('/credentials', newCredential.value)
    }
    showAddModal.value = false
    resetForm()
    await loadData()
  } catch (err) {
    alert('Failed to save credential.')
  }
}

async function deleteCredential(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Delete Credential?',
      message: 'This will permanently remove this credential from the vault. Bots using this credential will no longer be able to authenticate.',
      type: 'danger',
      confirmText: 'Delete Permanently',
      onConfirm: async () => {
          try {
            await api.delete(`/credentials/${id}`)
            await loadData()
          } catch (err) {
            alert('Failed to delete.')
          }
      }
  }
}

onMounted(loadData)
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
          <Key class="w-4 h-4 text-primary" />
          <span class="section-title-premium py-0 border-none pl-0 text-primary">Security Vault</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-xs font-bold uppercase tracking-[0.25em] text-slate-500">Global Credentials</span>
        </div>
        <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none">Credentials</h2>
      </div>
      
      <div class="flex flex-col lg:flex-row gap-4">
        <div class="relative group">
            <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-600 group-focus-within:text-primary transition-colors" />
            <input v-model="search" type="text" placeholder="Search keys..." class="input-premium h-14 pl-12 w-64 text-sm" />
        </div>
        <button class="btn-premium btn-premium-ghost px-10 h-14 w-full lg:w-auto" @click="loadData" :class="{ loading: loading }">
             <RefreshCw v-if="!loading" class="w-4 h-4 mr-2" />
             Sync Vault
        </button>
        <button class="btn-premium btn-premium-primary px-16 h-14 w-full lg:w-auto" @click="showAddModal = true; resetForm()">
             <Plus class="w-5 h-5 mr-2" />
             New Key
        </button>
      </div>
    </div>

    <!-- Stats / Info -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 px-6 lg:px-0">
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">Total Keys</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ credentials.length }}</span>
                <span class="text-xs font-bold text-primary uppercase">Vaulted</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">AI Providers</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ credentials.filter(c => isAI(c.kind)).length }}</span>
                <span class="text-xs font-bold text-indigo-400 uppercase">Models</span>
            </div>
        </div>
        <div class="p-8 bg-[#161a23]/30 border border-white/5 rounded-[2rem] space-y-2">
            <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">External Bridges</span>
            <div class="flex items-baseline gap-2">
                <span class="text-4xl font-black text-white">{{ credentials.filter(c => c.kind === 'chatwoot').length }}</span>
                <span class="text-xs font-bold text-amber-500 uppercase">Integrations</span>
            </div>
        </div>
    </div>

    <!-- Content Area -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Manage Encrypted Keys</div>

        <div v-if="filteredCredentials.length === 0" class="py-40 flex flex-col items-center justify-center bg-[#161a23]/20 rounded-[3rem] border border-dashed border-white/5">
            <div class="w-20 h-20 rounded-full bg-white/5 flex items-center justify-center text-slate-700 mb-6">
                <Lock class="w-8 h-8 opacity-20" />
            </div>
            <p class="text-sm font-bold text-slate-600 uppercase tracking-[0.2em]">No credentials found in the vault</p>
            <button v-if="search" @click="search = ''" class="mt-4 text-xs font-black text-primary uppercase tracking-widest hover:underline">Clear Search</button>
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-8">
            <div v-for="cred in filteredCredentials" :key="cred.id" class="card-premium">
                <div class="relative z-10 flex flex-col h-full">
                    <div class="flex justify-between items-start mb-8">
                        <div class="icon-box-premium"
                             :class="isAI(cred.kind) ? 'icon-box-primary' : 'icon-box-amber'">
                            <Cpu v-if="isAI(cred.kind)" class="w-8 h-8" />
                            <HardDrive v-else class="w-8 h-8" />
                        </div>
                        <div class="flex gap-2">
                            <button class="btn-card-action" @click="openEdit(cred)">
                                <Edit3 class="w-4 h-4" />
                            </button>
                            <button class="btn-card-action btn-card-action-red" @click="deleteCredential(cred.id)">
                                <Trash2 class="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    
                    <h4 class="text-2xl font-black text-white uppercase tracking-tighter mb-2 group-hover:text-primary transition-colors leading-none truncate">{{ cred.name }}</h4>
                    <div class="flex items-center gap-3 mb-6">
                        <span class="badge-premium" :class="isAI(cred.kind) ? 'badge-indigo' : 'badge-amber'">
                            {{ isAI(cred.kind) ? (cred.kind === 'ai' ? 'AI PROVIDER' : 'AI: ' + cred.kind.toUpperCase()) : 'CHATWOOT CRM' }}
                        </span>
                        <div class="w-1 h-1 rounded-full bg-slate-800"></div>
                        <span class="text-[10px] font-bold text-slate-600 uppercase tracking-widest font-mono">{{ cred.id.substring(0,8) }}</span>
                    </div>

                    <div class="space-y-4 mt-auto">
                        <div v-if="isAI(cred.kind)" class="storage-box-premium">
                            <div class="flex items-baseline justify-between mb-1">
                                <span class="text-[10px] font-bold text-slate-600 uppercase">API KEY</span>
                                <Lock class="w-3 h-3 text-slate-800" />
                            </div>
                            <div class="font-mono text-xs text-slate-400 break-all leading-relaxed tracking-tight select-all">
                                ••••••••••••••••••••••
                            </div>
                        </div>

                        <div v-if="cred.kind === 'chatwoot'" class="storage-box-premium space-y-3">
                            <div>
                                <span class="text-[10px] font-bold text-slate-600 uppercase block mb-1">INSTANCE URL</span>
                                <span class="text-[11px] font-medium text-slate-400 line-clamp-1 italic">{{ cred.chatwoot_base_url }}</span>
                            </div>
                            <div class="flex items-center justify-between border-t border-white/5 pt-2">
                                <div class="flex items-center gap-2">
                                    <ShieldCheck class="w-3 h-3 text-success/50" />
                                    <span class="text-[9px] font-black text-slate-600 uppercase">TOKEN VAULTED</span>
                                </div>
                                <span class="text-[9px] font-black text-slate-700 uppercase">ENCRYPTED</span>
                            </div>
                        </div>
                        
                        <div class="flex items-center justify-between pt-4">
                            <div class="flex items-center gap-2">
                                <div class="w-1.5 h-1.5 rounded-full bg-primary animate-pulse"></div>
                                <span class="text-[10px] font-black text-slate-500 uppercase tracking-widest">Secure Connection</span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="absolute -bottom-10 -right-10 w-40 h-40 bg-primary/5 rounded-full blur-[60px] group-hover:bg-primary/10 transition-colors duration-700"></div>
            </div>
        </div>
    </div>

    <!-- Add/Edit Credential Modal -->
    <AppModal v-model="showAddModal" :title="editingCredential ? 'Update Secure Credential' : 'Compose Global Credential'" maxWidth="max-w-2xl" noPadding noScroll>
        <div class="flex flex-col h-full overflow-hidden">
            <div class="flex-1 overflow-y-auto p-12 custom-scrollbar space-y-10 bg-[#0b0e14]">
                <div class="section-title-premium text-primary/60">Credential Definition</div>
                <div class="space-y-8">
                    <div class="form-control">
                        <label class="label-premium text-primary">Friendly Reference Name</label>
                        <input v-model="newCredential.name" type="text" class="input-premium h-16 w-full text-lg font-black" placeholder="e.g. Master Gemini Key" />
                        <p class="text-[10px] text-slate-600 font-bold uppercase tracking-widest mt-2 px-1">Used to identify this key when configuring bots.</p>
                    </div>

                    <div class="form-control">
                        <label class="label-premium text-slate-400">Credential Type</label>
                        <div class="grid grid-cols-2 gap-4">
                            <button @click="newCredential.kind = 'ai'" class="h-16 rounded-2xl border-2 transition-all flex items-center justify-center gap-4 uppercase font-black tracking-widest text-xs cursor-pointer hover:scale-[1.02] active:scale-[0.98]"
                                    :class="isAI(newCredential.kind) ? 'border-primary bg-primary/10 text-primary shadow-xl shadow-primary/10' : 'border-white/5 bg-white/5 text-slate-500 hover:border-white/20'">
                                <Cpu class="w-5 h-5" />
                                AI Provider
                            </button>
                            <button @click="newCredential.kind = 'chatwoot'" class="h-16 rounded-2xl border-2 transition-all flex items-center justify-center gap-4 uppercase font-black tracking-widest text-xs cursor-pointer hover:scale-[1.02] active:scale-[0.98]"
                                    :class="newCredential.kind === 'chatwoot' ? 'border-amber-500 bg-amber-500/10 text-amber-500 shadow-xl shadow-amber-500/10' : 'border-white/5 bg-white/5 text-slate-500 hover:border-white/20'">
                                <HardDrive class="w-5 h-5" />
                                Chatwoot Bridge
                            </button>
                        </div>
                    </div>

                    <TransitionGroup enter-active-class="transition duration-300 ease-out" enter-from-class="opacity-0 translate-y-4" move-class="transition duration-500">
                        <!-- AI Fields -->
                        <div v-if="isAI(newCredential.kind)" key="ai" class="space-y-6 animate-in fade-in slide-in-from-top-4">
                            <div class="form-control">
                                <div class="flex items-center gap-2 mb-2">
                                     <Lock class="w-3 h-3 text-indigo-400" />
                                     <label class="label-premium mb-0">API Key</label>
                                </div>
                                <input v-model="newCredential.ai_api_key" type="password" class="input-premium h-14 w-full text-sm font-mono" placeholder="Paste your API key here..." />
                            </div>
                        </div>

                        <!-- Chatwoot Fields -->
                        <div v-if="newCredential.kind === 'chatwoot'" key="cw" class="space-y-6 animate-in fade-in slide-in-from-top-4">
                            <div class="form-control">
                                <div class="flex items-center gap-2 mb-2">
                                     <Globe class="w-3 h-3 text-amber-500" />
                                     <label class="label-premium mb-0">Base URL</label>
                                </div>
                                <input v-model="newCredential.chatwoot_base_url" type="text" class="input-premium h-14 w-full text-sm font-mono" placeholder="https://app.chatwoot.com" />
                            </div>
                            <div class="form-control">
                                <div class="flex items-center gap-2 mb-2">
                                     <Lock class="w-3 h-3 text-amber-500" />
                                     <label class="label-premium mb-0">Account Token</label>
                                </div>
                                <input v-model="newCredential.chatwoot_account_token" type="password" class="input-premium h-14 w-full text-sm font-mono" placeholder="Paste account token..." />
                            </div>
                        </div>
                    </TransitionGroup>
                </div>
            </div>

            <!-- Fixed Footer -->
            <div class="flex-none p-8 border-t border-white/5 bg-[#0b0e14] flex justify-end gap-6">
                <button class="btn-premium btn-premium-ghost px-10" @click="showAddModal = false">Discard</button>
                <button class="btn-premium btn-premium-primary px-16" @click="saveCredential">
                    <CheckCircle2 class="w-4 h-4 mr-2" />
                    {{ editingCredential ? 'Confirm Update' : 'Vault Credential' }}
                </button>
            </div>
        </div>
    </AppModal>

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

<style scoped>
</style>

