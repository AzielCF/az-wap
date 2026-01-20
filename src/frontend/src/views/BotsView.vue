<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AppTabModal from '@/components/AppTabModal.vue'
import AppModal from '@/components/AppModal.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { 
  Bot, 
  Settings, 
  Plus, 
  Cpu, 
  Trash2, 
  Edit3, 
  CheckCircle2, 
  Fingerprint, 
  Activity, 
  Globe, 
  Clock, 
  Type,
  Zap,
  Lock,
  Wrench,
  Mic,
  Image,
  Video,
  FileText,
  Brain,
} from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const bots = ref<any[]>([])
const credentials = ref<any[]>([])
const availableModels = ref<Record<string, any[]>>({})

// Global AI Settings
const showGlobalSettings = ref(false)
const globalSettings = ref({
  global_system_prompt: '',
  timezone: 'UTC',
  debounce_ms: 1500,
  wait_contact_idle_ms: 5000,
  typing_enabled: true
})

const timezones = [
  { value: 'UTC', label: '(Use server default / UTC)' },
  { value: 'America/Bogota', label: 'America/Bogota' },
  { value: 'America/Lima', label: 'America/Lima' },
  { value: 'America/Mexico_City', label: 'America/Mexico_City' },
  { value: 'America/Santo_Domingo', label: 'America/Santo_Domingo (República Dominicana)' },
  { value: 'America/Santiago', label: 'America/Santiago' },
  { value: 'America/Argentina/Buenos_Aires', label: 'America/Argentina/Buenos_Aires' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles' },
  { value: 'America/New_York', label: 'America/New_York' },
  { value: 'Europe/Madrid', label: 'Europe/Madrid' },
  { value: 'Europe/London', label: 'Europe/London' }
]

const aiKinds = ['ai', 'gemini', 'openai', 'claude']
const aiCredentials = computed(() => credentials.value.filter(c => aiKinds.includes(c.kind)))
const chatwootCredentials = computed(() => credentials.value.filter(c => c.kind === 'chatwoot'))

// Bot Management
const showAddBot = ref(false)
const editingBot = ref<any>(null)
const newBot = ref({
  name: '',
  description: '',
  system_prompt: '',
  knowledge_base: '',
  model: '',
  api_key: '',
  credential_id: '',
  chatwoot_credential_id: '',
  chatwoot_bot_token: '',
  audio_enabled: true,
  image_enabled: true,
  video_enabled: false,
  document_enabled: false,
  memory_enabled: true,
  mindset_model: '',
  multimodal_model: '',
  timezone: '',
  provider: 'gemini'
})

// MCP Per Bot
const botMCPServers = ref<any[]>([])
const loadingBotMCPs = ref(false)
const expandedServers = ref<string[]>([])

// Confirmation State
const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'info' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

const activeTab = ref('general')

async function loadData() {
  loading.value = true
  try {
    const responses = (await Promise.all([
      api.get('/bots'),
      api.get('/credentials'),
      api.get('/settings/ai'),
      api.get('/bots/config/models')
    ])) as any[]
    
    const [bts, creds, settings, models] = responses
    bots.value = bts?.results || []
    credentials.value = creds?.results || []
    availableModels.value = models?.results || {}
    if (settings && settings.results) {
      globalSettings.value = {
        global_system_prompt: settings.results.global_system_prompt || '',
        timezone: settings.results.timezone || 'UTC',
        debounce_ms: settings.results.debounce_ms ?? 1500,
        wait_contact_idle_ms: settings.results.wait_contact_idle_ms ?? 5000,
        typing_enabled: settings.results.typing_enabled ?? true
      }
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function loadBotMCPs(botId: string) {
  loadingBotMCPs.value = true
  botMCPServers.value = []
  try {
    const res = await api.get(`/bots/${botId}/mcp`)
    const results = res.results || []
    
    botMCPServers.value = results.map((srv: any) => {
      let headersMap: Record<string, string> = {}
      if (srv.custom_headers) {
        headersMap = { ...srv.custom_headers }
      }
      if (srv.is_template && srv.template_config) {
        Object.keys(srv.template_config).forEach(k => {
          if (!headersMap[k]) headersMap[k] = ''
        })
      }
      return { 
        ...srv, 
        headersMap,
        botInstructions: srv.bot_instructions || '',
        disabled_tools: srv.disabled_tools || []
      }
    })
  } catch (err) {
    console.error('Failed to load MCPs', err)
  } finally {
    loadingBotMCPs.value = false
  }
}

async function saveMCPPreference(server: any) {
  if (!editingBot.value) return
  try {
    const payload = {
      enabled: server.enabled,
      disabled_tools: server.disabled_tools || [],
      custom_headers: server.headersMap,
      instructions: server.botInstructions || ''
    }
    await api.put(`/bots/${editingBot.value.id}/mcp/${server.id}`, payload)
  } catch (err) {
    alert('Failed to update MCP preferences.')
  }
}

async function toggleMCPForBot(server: any) {
  server.enabled = !server.enabled
  await saveMCPPreference(server)
}

function toggleServerExpansion(id: string) {
  if (expandedServers.value.includes(id)) {
    expandedServers.value = expandedServers.value.filter(i => i !== id)
  } else {
    expandedServers.value.push(id)
  }
}

function isToolDisabled(srv: any, toolName: string) {
  return srv.disabled_tools?.includes(toolName)
}

async function toggleToolForBot(srv: any, toolName: string) {
  let disabled = [...(srv.disabled_tools || [])]
  if (disabled.includes(toolName)) {
    disabled = disabled.filter(t => t !== toolName)
  } else {
    disabled.push(toolName)
  }
  srv.disabled_tools = disabled
  await saveMCPPreference(srv)
}

async function saveGlobalSettings() {
  try {
    await api.put('/settings/ai', globalSettings.value)
    showGlobalSettings.value = false
  } catch (err) {
    alert('Failed to update settings.')
  }
}

async function createBot() {
  try {
    if (editingBot.value) {
      await api.put(`/bots/${editingBot.value.id}`, newBot.value)
    } else {
      await api.post('/bots', newBot.value)
    }
    showAddBot.value = false
    resetForm()
    await loadData()
  } catch (err) {
    alert('Failed to save bot template.')
  }
}

async function deleteBot(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Delete Identity?',
      message: 'This bot identity will be permanently destroyed. Active instances using this bot will fallback to manual mode.',
      type: 'danger',
      confirmText: 'Delete Forever',
      onConfirm: async () => {
          try {
            await api.delete(`/bots/${id}`)
            await loadData()
          } catch (err) {
            alert('Failed to delete.')
          }
      }
  }
}

async function clearBotMemory(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Flush Memory Core?',
      message: 'EXTREME WARNING: This will wipe ALL short-term memory for this bot entity across ALL active users and instances. This action cannot be undone. Are you sure you want to trigger a global amnesia event?',
      type: 'danger',
      confirmText: 'Execute Flush',
      onConfirm: async () => {
          try {
            await api.post(`/bots/${id}/memory/clear`, {})
            alert('Memory core flushed successfully.')
          } catch (err) {
            alert('Failed to clear memory.')
          }
      }
  }
}

function openEdit(bot: any) {
  editingBot.value = bot
  newBot.value = {
    name: bot.name,
    description: bot.description,
    system_prompt: bot.system_prompt,
    knowledge_base: bot.knowledge_base || '',
    model: bot.model || '',
    api_key: bot.api_key || '',
    credential_id: bot.credential_id || '',
    chatwoot_credential_id: bot.chatwoot_credential_id || '',
    chatwoot_bot_token: bot.chatwoot_bot_token || '',
    audio_enabled: bot.audio_enabled !== false,
    image_enabled: bot.image_enabled !== false,
    video_enabled: !!bot.video_enabled,
    document_enabled: !!bot.document_enabled,
    memory_enabled: bot.memory_enabled !== false,
    mindset_model: bot.mindset_model || '',
    multimodal_model: bot.multimodal_model || '',
    timezone: bot.timezone || '',
    provider: bot.provider || 'gemini'
  }
  expandedServers.value = []
  loadBotMCPs(bot.id)
  showAddBot.value = true
}

function resetForm() {
  editingBot.value = null
  newBot.value = {
    name: '',
    description: '',
    system_prompt: '',
    knowledge_base: '',
    model: '',
    api_key: '',
    credential_id: '',
    chatwoot_credential_id: '',
    chatwoot_bot_token: '',
    audio_enabled: true,
    image_enabled: true,
    video_enabled: false,
    document_enabled: false,
    memory_enabled: true,
    mindset_model: '',
    multimodal_model: '',
    timezone: '',
    provider: 'gemini'
  }
  botMCPServers.value = []
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
          <Cpu class="w-4 h-4 text-primary" />
          <span class="section-title-premium py-0 border-none pl-0 text-primary">Automation Logic</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-xs font-bold uppercase tracking-[0.25em] text-slate-500">Identity Blueprints</span>
        </div>
        <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none">AI Control Board</h2>
      </div>
      
      <div class="flex flex-col lg:flex-row gap-4">
        <button class="btn-premium btn-premium-ghost px-8" @click="showGlobalSettings = true">
           <Settings class="w-4 h-4" />
           Engine Configuration
        </button>
        <button class="btn-premium btn-premium-primary px-12" @click="showAddBot = true; resetForm()">
           <Plus class="w-4 h-4" />
           New Bot Template
        </button>
      </div>
    </div>

    <!-- Content Area -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Reusable Bot Templates</div>

        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-10">
            <!-- Add Card -->
            <div @click="showAddBot = true; resetForm()" class="bg-[#161a23]/30 border-2 border-dashed border-white/5 hover:border-primary/30 rounded-[2.5rem] p-10 flex flex-col items-center justify-center text-center group cursor-pointer transition-all duration-500 min-h-[340px]">
                <div class="w-16 h-16 rounded-[1.5rem] bg-white/5 flex items-center justify-center text-slate-600 group-hover:bg-primary/10 group-hover:text-primary transition-all mb-6 border border-white/5 shadow-2xl">
                    <Plus class="w-8 h-8" />
                </div>
                <h4 class="text-sm font-black text-white/40 group-hover:text-white uppercase tracking-widest transition-all">Compose Template</h4>
            </div>

            <!-- Bot Cards -->
            <div v-for="bot in bots" :key="bot.id" class="card-premium min-h-[340px]">
                <div class="relative z-10 flex flex-col h-full">
                    <div class="flex justify-between items-start mb-10">
                        <div class="icon-box-premium icon-box-primary">
                            <Bot class="w-8 h-8" />
                        </div>
                        <div class="flex gap-2">
                            <button class="btn-card-action" @click="openEdit(bot)">
                                <Edit3 class="w-4 h-4" />
                            </button>
                            <button class="btn-card-action btn-card-action-red" @click="deleteBot(bot.id)">
                                <Trash2 class="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    
                    <h4 class="text-2xl font-black text-white uppercase tracking-tighter mb-4 group-hover:text-primary transition-colors leading-none truncate">{{ bot.name }}</h4>
                    <p class="text-sm text-slate-500 font-bold uppercase tracking-tight leading-relaxed line-clamp-2 h-10 opacity-60">{{ bot.description || 'Professional identity blueprint.' }}</p>
                    
                    <div class="flex gap-2 mt-6">
                        <div v-if="bot.audio_enabled !== false" class="p-1.5 rounded-lg bg-indigo-500/10 border border-indigo-500/20 text-indigo-400" title="Audio Capable">
                            <Mic class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.image_enabled !== false" class="p-1.5 rounded-lg bg-pink-500/10 border border-pink-500/20 text-pink-400" title="Vision Capable">
                            <Image class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.video_enabled" class="p-1.5 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-400" title="Video Capable">
                            <Video class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.document_enabled" class="p-1.5 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-400" title="Document Processing">
                            <FileText class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.memory_enabled !== false" class="p-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400" title="Long-term Memory">
                            <Brain class="w-3.5 h-3.5" />
                        </div>
                    </div>
                    
                    <div class="mt-auto pt-8 border-t border-white/5 flex items-center justify-between">
                        <div class="badge-premium badge-success gap-2">
                            <div class="w-2 h-2 rounded-full bg-success shadow-[0_0_8px_rgba(var(--su),0.5)]"></div>
                            <span>Ready</span>
                        </div>
                        <span class="text-xs font-mono text-slate-700 font-bold uppercase">{{ bot.id.substring(0,8) }}</span>
                    </div>
                </div>
                <div class="absolute -bottom-10 -right-10 w-40 h-40 bg-primary/5 rounded-full blur-[60px] group-hover:bg-primary/10 transition-colors duration-700"></div>
            </div>
        </div>
    </div>

    <!-- Global Engine Modal -->
    <AppModal v-model="showGlobalSettings" title="Engine Global Override" maxWidth="max-w-xl">
        <div class="space-y-8 py-4">
            <div class="form-control">
                <div class="flex items-center gap-2 mb-2">
                    <Zap class="w-3 h-3 text-primary" />
                    <label class="label-premium mb-0">Master System Prompt</label>
                </div>
                <textarea v-model="globalSettings.global_system_prompt" rows="5" class="input-premium w-full min-h-[120px] leading-relaxed text-sm" placeholder="Universal laws..."></textarea>
            </div>
            <div class="form-control">
                <div class="flex items-center gap-2 mb-2">
                    <Globe class="w-3 h-3 text-slate-400" />
                    <label class="label-premium mb-0">AI Timezone (IANA)</label>
                </div>
                <select v-model="globalSettings.timezone" class="select-premium h-14 w-full text-sm font-bold uppercase">
                    <option v-for="tz in timezones" :key="tz.value" :value="tz.value">{{ tz.label }}</option>
                </select>
            </div>
            <div class="grid grid-cols-2 gap-6">
                <div class="form-control">
                    <div class="flex items-center gap-2 mb-2">
                        <Clock class="w-3 h-3 text-primary" />
                        <label class="label-premium mb-0">Response Delay (ms)</label>
                    </div>
                    <input v-model.number="globalSettings.debounce_ms" type="number" class="input-premium h-14 w-full font-mono text-sm" />
                </div>
                <div class="form-control">
                    <div class="flex items-center gap-2 mb-2">
                        <Activity class="w-3 h-3 text-primary" />
                        <label class="label-premium mb-0">Idle Check (ms)</label>
                    </div>
                    <input v-model.number="globalSettings.wait_contact_idle_ms" type="number" class="input-premium h-14 w-full font-mono text-sm" />
                </div>
            </div>
            <label class="flex items-center justify-between h-14 bg-[#161a23] border border-white/10 rounded-xl px-6 cursor-pointer hover:border-success/40 transition-colors">
                <div class="flex items-center gap-3">
                    <Type class="w-4 h-4 text-success" />
                    <span class="text-xs font-black uppercase tracking-widest text-slate-400">Emulate Typing</span>
                </div>
                <input type="checkbox" v-model="globalSettings.typing_enabled" class="toggle toggle-success" />
            </label>
        </div>
        <template #actions>
            <button class="btn-premium btn-premium-ghost px-8" @click="showGlobalSettings = false">Discard</button>
            <button class="btn-premium btn-premium-success px-12" @click="saveGlobalSettings">Propagate Changes</button>
        </template>
    </AppModal>

    <!-- Bot Identity Modal -->
    <AppTabModal 
        v-model="showAddBot" 
        :title="editingBot ? 'Edit Identity Template' : 'Compose Bot Identity'" 
        maxWidth="max-w-[1240px]"
        v-model:activeTab="activeTab"
        :tabs="[
            { id: 'general', label: 'General Info', icon: Fingerprint },
            { id: 'engine', label: 'Engine Logic', icon: Zap },
            { id: 'auth', label: 'Authentication', icon: Lock },
            { id: 'mcp', label: 'Capabilities (MCP)', icon: Wrench }
        ]"
        :identity="{
            name: editingBot ? editingBot.name : 'New Bot Identity',
            subtitle: 'Identity Blueprint',
            icon: Bot,
            iconType: 'component'
        }"
        :saveText="editingBot ? 'Save Changes' : 'Create Identity'"
        @save="createBot"
        @cancel="showAddBot = false"
    >
        <template #sidebar-bottom>
            <div v-if="editingBot" class="pt-8 border-t border-white/5">
                <button @click="clearBotMemory(editingBot.id)" class="btn-premium btn-premium-ghost text-red-400 hover:bg-red-500/10 hover:text-red-300 w-full btn-premium-sm border border-red-500/20">
                    <Trash2 class="w-3.5 h-3.5 mr-2" />
                    Flush Memory
                </button>
            </div>
        </template>
                    <!-- TAB: General -->
                    <div v-if="activeTab === 'general'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
                        <div class="section-title-premium text-primary/60">Core Definition</div>
                        <div class="grid grid-cols-2 gap-8">
                            <div class="form-control">
                                <label class="label-premium text-primary">Template Name</label>
                                <input v-model="newBot.name" type="text" class="input-premium h-14 w-full text-lg font-black" placeholder="e.g. Sales Specialist" />
                            </div>
                            <div class="form-control">
                                <label class="label-premium">Description</label>
                                <input v-model="newBot.description" type="text" class="input-premium h-14 w-full" placeholder="What is this bot for?" />
                            </div>
                        </div>

                        <div class="form-control mt-10">
                            <label class="label-premium">Core Intelligence (System Prompt)</label>
                            <textarea v-model="newBot.system_prompt" rows="8" class="input-premium w-full leading-relaxed text-sm p-6" placeholder="Define boundaries and mission..."></textarea>
                        </div>

                        <div class="form-control">
                            <label class="label-premium">Knowledge Context (RAG)</label>
                            <textarea v-model="newBot.knowledge_base" rows="5" class="input-premium w-full leading-relaxed text-sm p-6" placeholder="Add custom domain data..."></textarea>
                        </div>
                    </div>

                    <!-- TAB: Engine -->
                    <div v-if="activeTab === 'engine'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
                        <div class="section-title-premium text-primary/60">Model Selection</div>
                        <div class="grid grid-cols-2 gap-8">
                            <div class="form-control">
                                <label class="label-premium">Engine Provider</label>
                                <select v-model="newBot.provider" class="select-premium h-14 w-full text-sm font-bold uppercase transition-all">
                                    <option value="gemini">Google Gemini (Active)</option>
                                    <option value="openai" disabled class="opacity-30">OpenAI (Soon)</option>
                                    <option value="claude" disabled class="opacity-30">Claude (Soon)</option>
                                    <option value="ai">Legacy / Custom</option>
                                </select>
                            </div>
                            <div class="form-control">
                                <label class="label-premium">Core Logic Model</label>
                                <select v-model="newBot.model" class="select-premium h-14 w-full text-sm font-bold uppercase">
                                    <option value="">(Inherit Provider Default)</option>
                                    <option v-for="m in availableModels[newBot.provider] || []" :key="m.id" :value="m.id">{{ m.name }}</option>
                                </select>
                            </div>
                        </div>

                        <div class="grid grid-cols-2 gap-8 pt-8 border-t border-white/5">
                            <div class="form-control">
                                <label class="label-premium">Mindset Analyzer</label>
                                <select v-model="newBot.mindset_model" class="select-premium h-14 w-full text-sm font-bold uppercase">
                                    <option value="">(Inherit Logic / Lite Preferred)</option>
                                    <option v-for="m in availableModels[newBot.provider] || []" :key="m.id" :value="m.id">{{ m.name }}</option>
                                </select>
                            </div>
                            <div class="form-control">
                                <label class="label-premium">Vision/Multimodal Interpreter</label>
                                <select v-model="newBot.multimodal_model" class="select-premium h-14 w-full text-sm font-bold uppercase">
                                    <option value="">(Vision Preferred)</option>
                                    <option v-for="m in availableModels[newBot.provider] || []" :key="m.id" :value="m.id">{{ m.name }}</option>
                                </select>
                            </div>
                        </div>

                        <div class="section-title-premium text-primary/60 pt-10">Sensory Capabilities</div>
                        <div class="grid grid-cols-2 lg:grid-cols-5 gap-4">
                            <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all">
                                <span class="text-[9px] font-black uppercase text-slate-500">Memory</span>
                                <input type="checkbox" v-model="newBot.memory_enabled" class="toggle toggle-primary toggle-xs" />
                            </label>
                            <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all">
                                <span class="text-[9px] font-black uppercase text-slate-500">Audio</span>
                                <input type="checkbox" v-model="newBot.audio_enabled" class="toggle toggle-primary toggle-xs" />
                            </label>
                            <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all">
                                <span class="text-[9px] font-black uppercase text-slate-500">Vision</span>
                                <input type="checkbox" v-model="newBot.image_enabled" class="toggle toggle-primary toggle-xs" />
                            </label>
                            <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-indigo-400 transition-all">
                                <span class="text-[9px] font-black uppercase text-slate-500">Video</span>
                                <input type="checkbox" v-model="newBot.video_enabled" class="toggle toggle-info toggle-xs" />
                            </label>
                            <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-amber-400 transition-all">
                                <span class="text-[9px] font-black uppercase text-slate-500">Docs</span>
                                <input type="checkbox" v-model="newBot.document_enabled" class="toggle toggle-warning toggle-xs" />
                            </label>
                        </div>
                    </div>

                    <!-- TAB: Auth -->
                    <div v-if="activeTab === 'auth'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
                        <div class="section-title-premium text-primary/60">Vaulted Credentials</div>
                        <div class="grid grid-cols-2 gap-8">
                            <div class="form-control">
                                <label class="label-premium">AI Provider Key</label>
                                <select v-model="newBot.credential_id" class="select-premium h-14 w-full text-sm font-bold uppercase">
                                    <option value="">(Manual Key Entry)</option>
                                    <option v-for="cred in aiCredentials" :key="cred.id" :value="cred.id">{{ cred.name }}</option>
                                </select>
                            </div>
                            <div v-if="!newBot.credential_id" class="form-control">
                                <label class="label-premium">Direct API Access Key</label>
                                <input v-model="newBot.api_key" type="password" class="input-premium h-14 w-full font-mono text-xs" placeholder="Paste manual key..." />
                            </div>
                        </div>

                        <div class="grid grid-cols-2 gap-8 pt-10 border-t border-white/5">
                            <div class="form-control">
                                <label class="label-premium">Chatwoot Credential</label>
                                <select v-model="newBot.chatwoot_credential_id" class="select-premium h-14 w-full text-sm font-bold uppercase">
                                    <option value="">(None)</option>
                                    <option v-for="cred in chatwootCredentials" :key="cred.id" :value="cred.id">{{ cred.name }}</option>
                                </select>
                            </div>
                            <div class="form-control">
                                <label class="label-premium">Chatwoot Agent Token</label>
                                <input v-model="newBot.chatwoot_bot_token" type="password" class="input-premium h-14 w-full font-mono text-xs" placeholder="Paste agent token..." />
                            </div>
                        </div>
                    </div>

                    <!-- TAB: MCP -->
                    <div v-if="activeTab === 'mcp'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
                        <div class="section-title-premium text-primary/60 flex justify-between items-center">
                            Tool Access Registry
                            <span v-if="loadingBotMCPs" class="loading loading-spinner loading-xs"></span>
                        </div>

                        <div v-if="!editingBot" class="p-20 bg-white/[0.02] border border-dashed border-white/5 rounded-[2rem] text-center">
                            <Wrench class="w-10 h-10 text-slate-700 mx-auto mb-4 opacity-20" />
                            <p class="text-xs font-bold uppercase tracking-widest text-slate-600">MCP Configuration is available after bot creation.</p>
                        </div>

                        <div v-else-if="botMCPServers.length === 0" class="p-20 bg-white/[0.02] border border-dashed border-white/5 rounded-[2rem] text-center">
                            <p class="text-xs font-bold uppercase tracking-widest text-slate-600">No MCP Servers detected.</p>
                        </div>

                        <div v-else class="space-y-6">
                            <div v-for="srv in botMCPServers" :key="srv.id" 
                                 class="rounded-[2rem] bg-[#161a23] border transition-all"
                                 :class="srv.enabled ? 'border-primary/40' : 'border-white/5 opacity-60'">
                                
                                <div class="p-6 flex items-center justify-between">
                                    <div class="flex items-center gap-4">
                                         <div class="w-10 h-10 rounded-xl bg-white/5 flex items-center justify-center text-primary border border-white/10">
                                             <Zap class="w-5 h-5" />
                                         </div>
                                         <div class="flex flex-col">
                                             <h6 class="text-xs font-black text-white uppercase tracking-tight">{{ srv.name }}</h6>
                                             <span class="text-[9px] font-mono text-slate-600 truncate max-w-[200px]">{{ srv.url }}</span>
                                         </div>
                                    </div>
                                    <div class="flex items-center gap-4">
                                         <button @click="toggleServerExpansion(srv.id)" class="btn-premium btn-premium-ghost btn-premium-sm px-4">
                                             <Settings class="w-3 h-3" />
                                         </button>
                                         <input type="checkbox" :checked="srv.enabled" @change="toggleMCPForBot(srv)" class="toggle toggle-primary toggle-sm" />
                                    </div>
                                </div>

                                <div v-if="expandedServers.includes(srv.id) || srv.enabled" class="px-6 pb-8 border-t border-white/5 pt-6 space-y-6">
                                    <div class="form-control">
                                        <label class="label-premium opacity-50">Local Instructions</label>
                                        <textarea v-model="srv.botInstructions" rows="2" class="input-premium w-full text-xs" placeholder="Guidelines for this bot..."></textarea>
                                        <div class="flex justify-end mt-2">
                                            <button @click="saveMCPPreference(srv)" class="text-[9px] font-black uppercase text-success tracking-widest hover:underline">Update strategy</button>
                                        </div>
                                    </div>

                                    <div v-if="srv.tools && srv.tools.length" class="grid grid-cols-1 md:grid-cols-2 gap-3">
                                        <div v-for="tool in srv.tools" :key="tool.name" 
                                             @click="toggleToolForBot(srv, tool.name)"
                                             class="flex items-center justify-between p-4 bg-black/20 border border-white/5 rounded-2xl cursor-pointer hover:bg-black/40 transition-all"
                                             :class="{ 'opacity-40 grayscale': isToolDisabled(srv, tool.name) }">
                                            <div class="flex items-center gap-3">
                                                <Wrench class="w-3.5 h-3.5 text-primary" />
                                                <span class="text-[10px] font-black text-white uppercase">{{ tool.name }}</span>
                                            </div>
                                            <div class="w-5 h-5 rounded-lg flex items-center justify-center border border-white/10"
                                                 :class="!isToolDisabled(srv, tool.name) ? 'bg-primary text-white' : 'bg-transparent text-transparent'">
                                                <CheckCircle2 class="w-3 h-3" />
                                            </div>
                                        </div>
                                    </div>
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

<style scoped>
</style>
