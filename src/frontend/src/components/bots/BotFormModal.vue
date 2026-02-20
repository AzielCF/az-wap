<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AppTabModal from '@/components/AppTabModal.vue'
import { 
  Bot, 
  Settings, 
  Trash2, 
  CheckCircle2, 
  Fingerprint, 
  Activity, 
  Zap,
  Lock,
  Wrench,
  Mic,
  Image,
  Video,
  FileText,
  Brain,
  ShieldCheck,
  Plus
} from 'lucide-vue-next'

const props = defineProps<{
  modelValue: boolean
  editingBot: any
  credentials: any[]
  availableModels: Record<string, any[]>
}>()

const emit = defineEmits(['update:modelValue', 'saved', 'clear-memory'])

const api = useApi()

const activeTab = ref('general')

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
  provider: 'gemini',
  variants: {} as Record<string, any>
})

const aiKinds = ['ai', 'gemini', 'openai', 'claude']
const aiCredentials = computed(() => props.credentials.filter(c => aiKinds.includes(c.kind)))
const chatwootCredentials = computed(() => props.credentials.filter(c => c.kind === 'chatwoot'))
const multimodalModels = computed(() => {
    const models = props.availableModels[newBot.value.provider] || []
    return models.filter(m => m.is_multimodal)
})

// MCP Per Bot
const botMCPServers = ref<any[]>([])
const loadingBotMCPs = ref(false)
const testingConnection = ref<Record<string, 'idle' | 'loading' | 'success' | 'error'>>({})
const connectionMessages = ref<Record<string, string>>({})
const botMCPServersSnapshot = ref<Record<string, string>>({})
const botDataSnapshot = ref<string>('')
const expandedServers = ref<string[]>([])

const selectedVariant = ref<string>('') // '' means base bot

const currentName = computed({
  get() {
    if (!selectedVariant.value) return newBot.value.name
    return newBot.value.variants[selectedVariant.value]?.name || ''
  },
  set(val) {
    if (!selectedVariant.value) newBot.value.name = val
    else if (newBot.value.variants[selectedVariant.value]) {
        newBot.value.variants[selectedVariant.value].name = val
    }
  }
})

const currentPrompt = computed({
  get() {
    if (!selectedVariant.value) return newBot.value.system_prompt
    return newBot.value.variants[selectedVariant.value]?.system_prompt || ''
  },
  set(val) {
    if (!selectedVariant.value) newBot.value.system_prompt = val
    else if (newBot.value.variants[selectedVariant.value]) {
        newBot.value.variants[selectedVariant.value].system_prompt = val
    }
  }
})

const currentDescription = computed({
  get() {
    if (!selectedVariant.value) return newBot.value.description
    return newBot.value.variants[selectedVariant.value]?.description || ''
  },
  set(val) {
    if (!selectedVariant.value) newBot.value.description = val
    else if (newBot.value.variants[selectedVariant.value]) {
        newBot.value.variants[selectedVariant.value].description = val
    }
  }
})

function createCapabilityProp(prop: 'memory_enabled' | 'audio_enabled' | 'image_enabled' | 'video_enabled' | 'document_enabled') {
  return computed({
    get() {
      if (!selectedVariant.value) return newBot.value[prop]
      const v = newBot.value.variants[selectedVariant.value]
      return v && v[prop] !== undefined && v[prop] !== null ? v[prop] : newBot.value[prop]
    },
    set(val: boolean) {
      if (!selectedVariant.value) {
          (newBot.value as any)[prop] = val
      } else if (newBot.value.variants[selectedVariant.value]) {
          newBot.value.variants[selectedVariant.value][prop] = val
      }
    }
  })
}

const currentMemoryEnabled = createCapabilityProp('memory_enabled')
const currentAudioEnabled = createCapabilityProp('audio_enabled')
const currentImageEnabled = createCapabilityProp('image_enabled')
const currentVideoEnabled = createCapabilityProp('video_enabled')
const currentDocumentEnabled = createCapabilityProp('document_enabled')

function addVariant(inherit: boolean) {
    const key = 'variant_' + Date.now()
    if (!newBot.value.variants) {
        newBot.value.variants = {}
    }
    
    if (inherit) {
        newBot.value.variants[key] = {
            name: newBot.value.name + ' (Copy)',
            description: newBot.value.description,
            system_prompt: newBot.value.system_prompt,
            allowed_tools: undefined, // Let it inherit all active tools initially
            allowed_mcps: undefined // Let it inherit all active servers initially 
        }
    } else {
        newBot.value.variants[key] = {
            name: 'New Sub-Role',
            description: '',
            system_prompt: '',
            allowed_tools: [],
            allowed_mcps: [] 
        }
    }
    selectedVariant.value = key
}

function deleteVariant(key: string) {
    if (newBot.value.variants && newBot.value.variants[key]) {
        delete newBot.value.variants[key]
    }
    if (selectedVariant.value === key) {
        selectedVariant.value = ''
    }
}

const modalTabs = computed(() => {
    const baseTabs = [
        { id: 'general', label: 'General Info', icon: Fingerprint },
        { id: 'engine', label: 'Engine Logic', icon: Zap },
        { id: 'auth', label: 'Authentication', icon: Lock },
        { id: 'mcp', label: 'Capabilities (MCP)', icon: Wrench }
    ]
    if (selectedVariant.value !== '') {
        return baseTabs.filter(t => t.id !== 'auth')
    }
    return baseTabs
})

watch(selectedVariant, (newVal) => {
    if (newVal !== '' && activeTab.value === 'auth') {
        activeTab.value = 'general'
    }
})

// Watch for changes to initialize form
watch(() => props.modelValue, (isOpen) => {
    if (isOpen) {
        initForm()
        loadBotMCPs(props.editingBot?.id || null)
    }
})

function initForm() {
    activeTab.value = 'general'
    selectedVariant.value = ''
    if (props.editingBot) {
        const bot = props.editingBot
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
            provider: bot.provider || 'gemini',
            variants: bot.variants ? JSON.parse(JSON.stringify(bot.variants)) : {}
        }
    } else {
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
            provider: 'gemini',
            variants: {}
        }
    }
    botDataSnapshot.value = JSON.stringify(newBot.value)
}

async function loadBotMCPs(botId: string | null) {
  loadingBotMCPs.value = true
  botMCPServers.value = []
  expandedServers.value = []
  try {
    const endpoint = botId ? `/bots/${botId}/mcp` : '/mcp/servers'
    const res = await api.get(endpoint)
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

      let urlVariables: Record<string, string> = {}
      if (srv.url_variables) {
        urlVariables = { ...srv.url_variables }
      }
      
      const urlMatches = (srv.url || '').match(/\{([^}]+)\}/g)?.map((m: string) => m.slice(1, -1)) || []
      urlMatches.forEach((key: string) => {
          if (!urlVariables[key]) urlVariables[key] = ''
      })

      const missingHeaders = srv.is_template && srv.template_config && 
                           Object.keys(srv.template_config).some(k => !headersMap[k] || headersMap[k].trim() === '');
      const missingVars = urlMatches.some((k: string) => !urlVariables[k] || urlVariables[k].trim() === '');

      const needsConfig = missingHeaders || missingVars;
      
      if (needsConfig && !expandedServers.value.includes(srv.id)) {
        expandedServers.value.push(srv.id)
      }

      const mapped = { 
        ...srv, 
        headersMap,
        urlVariables,
        urlVarKeys: urlMatches,
        botInstructions: srv.bot_instructions || '',
        disabled_tools: srv.disabled_tools || []
      }

      botMCPServersSnapshot.value[srv.id] = getMCPCompareState(mapped)

      return mapped
    })
  } catch (err) {
    console.error('Failed to load MCPs', err)
  } finally {
    loadingBotMCPs.value = false
  }
}

function getMCPCompareState(srv: any) {
  return JSON.stringify({
    enabled: !!srv.enabled,
    disabled_tools: [...(srv.disabled_tools || [])].sort(),
    custom_headers: Object.keys(srv.headersMap || {}).sort().reduce((obj: any, key) => {
      obj[key] = srv.headersMap[key]
      return obj
    }, {}),
    url_variables: Object.keys(srv.urlVariables || {}).sort().reduce((obj: any, key) => {
        obj[key] = srv.urlVariables[key]
        return obj
    }, {}),
    instructions: srv.botInstructions || ''
  })
}

async function saveMCPPreference(server: any) {
  if (!props.editingBot) return
  const payload = {
    enabled: server.enabled,
    disabled_tools: server.disabled_tools || [],
    custom_headers: server.headersMap,
    url_variables: server.urlVariables,
    instructions: server.botInstructions || ''
  }
  await api.put(`/bots/${props.editingBot.id}/mcp/${server.id}`, payload)
}

async function saveAllMCPPreferences(botId: string) {
  const promises = botMCPServers.value
    .filter(srv => {
        const currentState = getMCPCompareState(srv)
        return currentState !== botMCPServersSnapshot.value[srv.id]
    })
    .map(srv => {
      const payload = {
        enabled: srv.enabled,
        disabled_tools: srv.disabled_tools || [],
        custom_headers: srv.headersMap,
        url_variables: srv.urlVariables,
        instructions: srv.botInstructions || ''
      }
      return api.put(`/bots/${botId}/mcp/${srv.id}`, payload)
    })
  
  if (promises.length > 0) {
    await Promise.all(promises)
    botMCPServers.value.forEach(srv => {
      botMCPServersSnapshot.value[srv.id] = getMCPCompareState(srv)
    })
  }
}

async function testMCPConnection(server: any) {
  if (!props.editingBot) return
  testingConnection.value[server.id] = 'loading'
  connectionMessages.value[server.id] = ''
  
  try {
    await saveMCPPreference(server)
    const res = await api.post(`/api/health/mcp/${server.id}/check`, {})
    
    if (res.results?.status === 'OK') {
       testingConnection.value[server.id] = 'success'
       connectionMessages.value[server.id] = 'Bridge verified and protocol synced.'
    } else {
       testingConnection.value[server.id] = 'error'
       connectionMessages.value[server.id] = res.results?.last_message || 'MCP refused connection'
    }
  } catch (err) {
    testingConnection.value[server.id] = 'error'
    connectionMessages.value[server.id] = 'Failed to resolve MCP network bridge.'
  } finally {
    setTimeout(() => {
        if (testingConnection.value[server.id] === 'success') {
            testingConnection.value[server.id] = 'idle'
        }
    }, 5000)
  }
}

function isServerEnabledForVariant(srvId: string) {
  if (selectedVariant.value) {
      const variant = newBot.value.variants[selectedVariant.value]
      if (variant && variant.allowed_mcps !== undefined) {
          return variant.allowed_mcps.includes(srvId)
      }
      return true // Defaults to inherited base server if undefined!
  }
  return false
}

function toggleServer(srv: any) {
  if (selectedVariant.value) {
      const variant = newBot.value.variants[selectedVariant.value]
      if (variant.allowed_mcps === undefined) {
          // Initialize active variant servers to all currently active base bot servers
          const activeServers = botMCPServers.value.filter(s => s.enabled).map(s => s.id)
          variant.allowed_mcps = [...activeServers]
      }
      
      let allowed = [...variant.allowed_mcps]
      if (allowed.includes(srv.id)) {
          allowed = allowed.filter(id => id !== srv.id)
      } else {
          allowed.push(srv.id)
      }
      variant.allowed_mcps = allowed
  } else {
      srv.enabled = !srv.enabled
  }
}

function toggleServerExpansion(id: string) {
  if (expandedServers.value.includes(id)) {
    expandedServers.value = expandedServers.value.filter(i => i !== id)
  } else {
    expandedServers.value.push(id)
  }
}

function isToolDisabled(srv: any, toolName: string) {
  if (selectedVariant.value) {
     const variant = newBot.value.variants[selectedVariant.value]
     if (variant && variant.allowed_tools) {
         return !variant.allowed_tools.includes(toolName)
     }
     // Default for new variant is everything allowed (so not disabled) but we need to initialize
     return false
  }
  return srv.disabled_tools?.includes(toolName)
}

function toggleToolForBot(srv: any, toolName: string) {
  if (selectedVariant.value) {
      const variant = newBot.value.variants[selectedVariant.value]
      if (!variant.allowed_tools) {
          // Initialize allowed tools to all tools if this is the first toggle
          let allTools: string[] = []
          botMCPServers.value.forEach(s => {
              if (s.tools) {
                 allTools.push(...s.tools.map((t: any) => t.name))
              }
          })
          variant.allowed_tools = [...allTools]
      }
      
      let allowed = [...variant.allowed_tools]
      if (allowed.includes(toolName)) {
          allowed = allowed.filter(t => t !== toolName)
      } else {
          allowed.push(toolName)
      }
      variant.allowed_tools = allowed
      return
  }

  let disabled = [...(srv.disabled_tools || [])]
  if (disabled.includes(toolName)) {
    disabled = disabled.filter(t => t !== toolName)
  } else {
    disabled.push(toolName)
  }
  srv.disabled_tools = disabled
}

async function createBot() {
  try {
    let botId = props.editingBot?.id
    if (props.editingBot) {
      if (JSON.stringify(newBot.value) !== botDataSnapshot.value) {
        await api.put(`/bots/${props.editingBot.id}`, newBot.value)
        botDataSnapshot.value = JSON.stringify(newBot.value)
      }
    } else {
      const res = await api.post('/bots', newBot.value)
      botId = res?.results?.id
    }

    if (botId && botMCPServers.value.length > 0) {
      await saveAllMCPPreferences(botId)
    }

    emit('saved')
    closeModal()
  } catch (err) {
    alert('Failed to save bot template.')
  }
}

function closeModal() {
  emit('update:modelValue', false)
}

function clearMemory() {
    if (props.editingBot) {
        emit('clear-memory', props.editingBot.id)
    }
}
</script>

<template>
  <AppTabModal 
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      :title="editingBot ? 'Bot Configuration' : 'Compose Bot Identity'" 
      maxWidth="max-w-[1240px]"
      v-model:activeTab="activeTab"
      :tabs="modalTabs"
      :identity="{
          name: editingBot ? editingBot.name : 'New Bot Identity',
          subtitle: 'Identity Blueprint',
          icon: Bot,
          iconType: 'component'
      }"
      :saveText="editingBot ? 'Save Changes' : 'Create Identity'"
      @save="createBot"
      @cancel="closeModal"
  >
      <template #sidebar-bottom>
          <div v-if="editingBot" class="pt-8 border-t border-white/5">
              <button @click="clearMemory" class="btn-premium btn-premium-ghost text-red-400 hover:bg-red-500/10 hover:text-red-300 w-full btn-premium-sm border border-red-500/20">
                  <Trash2 class="w-3.5 h-3.5 mr-2" />
                  Flush Memory
              </button>
          </div>
      </template>

      <template #header-actions>
          <div class="flex items-center gap-2 flex-nowrap bg-black/20 p-1.5 rounded-[2rem] border border-white/5">
              <button 
                  @click="selectedVariant = ''"
                  class="px-4 py-2 rounded-[1.5rem] text-[11px] font-bold transition-all border shadow-sm flex items-center gap-2"
                  :class="selectedVariant === '' ? 'bg-primary text-white border-primary/50 shadow-primary/20' : 'bg-transparent border-transparent text-slate-400 hover:text-white'"
              >
                  <Fingerprint class="w-3.5 h-3.5" />
                  <span class="hidden sm:inline">Base Identity</span>
              </button>
              
              <div v-if="newBot.variants && Object.keys(newBot.variants).length > 0" class="w-px h-5 bg-white/10 mx-1"></div>

              <button 
                  v-for="(vr, key) in (newBot.variants || {})" :key="key"
                  @click="selectedVariant = key as string"
                  class="px-4 py-2 rounded-[1.5rem] text-[11px] font-bold transition-all border group flex items-center gap-2 shadow-sm"
                  :class="selectedVariant === key ? 'bg-indigo-500 text-white border-indigo-500/50 shadow-indigo-500/20' : 'bg-transparent border-transparent text-slate-400 hover:text-white'"
              >
                  <Bot class="w-3.5 h-3.5" />
                  <span class="max-w-[80px] truncate hidden sm:inline">{{ vr.name || 'Unnamed' }}</span>
                  <Trash2 
                     @click.stop="deleteVariant(key as string)"
                     class="w-3.5 h-3.5 ml-1 opacity-0 group-hover:opacity-100 transition-opacity hover:text-red-300 pointer-events-auto" 
                  />
              </button>
              
              <div class="dropdown dropdown-end">
                  <div tabindex="0" role="button" class="btn btn-ghost btn-sm btn-circle text-slate-400 hover:text-white ml-1">
                      <Plus class="w-5 h-5" />
                  </div>
                  <ul tabindex="0" class="dropdown-content z-[1] menu p-2 shadow-2xl bg-[#161a23] rounded-2xl w-48 border border-white/5 mt-2">
                      <li class="menu-title px-4 py-2 text-[10px] text-slate-500 uppercase tracking-widest">New Sub-Role</li>
                      <li><a @click="addVariant(true)" class="text-xs font-bold text-white hover:bg-white/5 rounded-xl py-3"><Brain class="w-3.5 h-3.5 text-primary mr-2"/> Inherit Base Config</a></li>
                      <li><a @click="addVariant(false)" class="text-xs font-bold text-white hover:bg-white/5 rounded-xl py-3"><FileText class="w-3.5 h-3.5 text-slate-400 mr-2"/> Blank Scope</a></li>
                  </ul>
              </div>
          </div>
      </template>

      <!-- TAB: General -->
      <div v-if="activeTab === 'general'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
          <div class="section-title-premium text-primary/60 mt-2">
              {{ selectedVariant === '' ? 'Core Definition' : 'Variant Definition' }}
          </div>
          <div class="grid grid-cols-2 gap-8">
              <div class="form-control" :class="{ 'col-span-2': selectedVariant !== '' }">
                  <label class="label-premium text-primary">{{ selectedVariant === '' ? 'Identity Name' : 'Sub-Role Name' }}</label>
                  <input v-model="currentName" type="text" class="input-premium h-14 w-full text-lg font-black" :placeholder="selectedVariant === '' ? 'e.g. Sales Specialist' : 'e.g. Cashier'" />
              </div>
              <div class="form-control" v-if="selectedVariant === ''">
                  <label class="label-premium">Description</label>
                  <input v-model="currentDescription" type="text" class="input-premium h-14 w-full" placeholder="What is this bot for?" />
              </div>
          </div>
          <div class="form-control -mt-6" v-if="selectedVariant !== ''">
              <label class="label-premium">Variant Description</label>
              <input v-model="currentDescription" type="text" class="input-premium h-14 w-full" placeholder="Internal note for this sub-role..." />
          </div>

          <div class="form-control mt-10">
              <label class="label-premium flex items-center gap-2">
                 {{ selectedVariant === '' ? 'Core Intelligence (System Prompt)' : 'Variant Override (System Prompt)' }}
                 <span v-if="selectedVariant !== ''" class="px-2 py-0.5 rounded bg-indigo-500/10 text-indigo-400 text-[10px] uppercase font-black ml-2">Replaces Core Prompt</span>
              </label>
              <textarea v-model="currentPrompt" rows="8" class="input-premium w-full leading-relaxed text-sm p-6" placeholder="Define boundaries and mission..."></textarea>
          </div>

          <div class="form-control" v-if="selectedVariant === ''">
              <label class="label-premium">Knowledge Context (RAG)</label>
              <textarea v-model="newBot.knowledge_base" rows="5" class="input-premium w-full leading-relaxed text-sm p-6" placeholder="Add custom domain data..."></textarea>
          </div>
          
          <div v-if="selectedVariant !== ''" class="p-6 bg-indigo-500/5 border border-indigo-500/10 rounded-2xl flex items-start gap-4">
              <div class="w-8 h-8 rounded-full bg-indigo-500/10 flex items-center justify-center shrink-0">
                  <Bot class="w-4 h-4 text-indigo-400" />
              </div>
              <div>
                  <h4 class="text-sm font-bold text-indigo-400 mb-1">Sub-Role Mode Active</h4>
                  <p class="text-xs text-slate-400 leading-relaxed">
                      This system prompt will completely override the Base Identity's prompt when this variant is invoked dynamically via the API. The Base Identity's Engine capabilities, MCP tools, and Knowledge Context are still inherited unless restricted.
                  </p>
              </div>
          </div>
      </div>

      <!-- TAB: Engine -->
      <div v-if="activeTab === 'engine'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
          <template v-if="selectedVariant === ''">
              <div class="section-title-premium text-primary/60">Model Selection</div>
          <div class="grid grid-cols-2 gap-8">
              <div class="form-control">
                  <label class="label-premium">Engine Provider</label>
                  <select v-model="newBot.provider" class="select-premium h-14 w-full text-sm font-bold uppercase transition-all">
                      <option value="gemini">Google Gemini (Active)</option>
                      <option value="openai">OpenAI (Experimental)</option>
                      <option value="claude" disabled class="opacity-30">Claude (Soon)</option>
                      <option value="ai">Legacy / Custom</option>
                  </select>
              </div>
              <div class="form-control">
                  <label class="label-premium">Core Logic Model</label>
                  <select v-model="newBot.model" class="select-premium h-14 w-full text-sm font-bold uppercase transition-all">
                      <option value="">(Inherit Provider Default)</option>
                      <option v-for="m in availableModels[newBot.provider] || []" :key="m.id" :value="m.id">
                          {{ m.name }} — {{ (m.avg_cost_in || m.avg_cost_out) ? `[$${m.avg_cost_in} / $${m.avg_cost_out}]` : '[--]' }}
                      </option>
                  </select>
              </div>
          </div>

          <div class="grid grid-cols-2 gap-8 pt-8 border-t border-white/5">
              <div class="form-control">
                  <label class="label-premium">Mindset Analyzer</label>
                  <select v-model="newBot.mindset_model" class="select-premium h-14 w-full text-sm font-bold uppercase">
                      <option value="">(Inherit Logic / Lite Preferred)</option>
                      <option v-for="m in availableModels[newBot.provider] || []" :key="m.id" :value="m.id">
                          {{ m.name }} — {{ (m.avg_cost_in || m.avg_cost_out) ? `[$${m.avg_cost_in} / $${m.avg_cost_out}]` : '[--]' }}
                      </option>
                  </select>
              </div>
              <div class="form-control">
                  <label class="label-premium">Vision/Multimodal Interpreter</label>
                  <select v-model="newBot.multimodal_model" class="select-premium h-14 w-full text-sm font-bold uppercase">
                      <option value="">(Vision Preferred)</option>
                      <option v-for="m in multimodalModels" :key="m.id" :value="m.id">
                          {{ m.name }} — {{ (m.avg_cost_in || m.avg_cost_out) ? `[$${m.avg_cost_in} / $${m.avg_cost_out}]` : '[--]' }}
                      </option>
                  </select>
              </div>
              </div>
          </template>

          <div class="section-title-premium text-primary/60" :class="{ 'mt-10': selectedVariant === '' }">
              Capabilities {{ selectedVariant !== '' ? '(Variant Scope)' : '' }}
          </div>
          <div class="grid grid-cols-2 lg:grid-cols-5 gap-4">
              <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all group">
                  <div class="flex items-center gap-2">
                       <Brain class="w-4 h-4 text-slate-500 group-hover:text-primary transition-colors" />
                       <span class="text-xs font-black uppercase text-slate-500 group-hover:text-slate-300">Memory</span>
                  </div>
                  <input type="checkbox" v-model="currentMemoryEnabled" class="toggle toggle-primary toggle-xs" />
              </label>
              <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all group">
                  <div class="flex items-center gap-2">
                       <Mic class="w-4 h-4 text-slate-500 group-hover:text-primary transition-colors" />
                       <span class="text-xs font-black uppercase text-slate-500 group-hover:text-slate-300">Audio</span>
                  </div>
                  <input type="checkbox" v-model="currentAudioEnabled" class="toggle toggle-primary toggle-xs" />
              </label>
              <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-primary transition-all group">
                  <div class="flex items-center gap-2">
                       <Image class="w-4 h-4 text-slate-500 group-hover:text-primary transition-colors" />
                       <span class="text-xs font-black uppercase text-slate-500 group-hover:text-slate-300">Vision</span>
                  </div>
                  <input type="checkbox" v-model="currentImageEnabled" class="toggle toggle-primary toggle-xs" />
              </label>
              <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-indigo-400 transition-all group">
                  <div class="flex items-center gap-2">
                       <Video class="w-4 h-4 text-slate-500 group-hover:text-indigo-400 transition-colors" />
                       <span class="text-xs font-black uppercase text-slate-500 group-hover:text-slate-300">Video</span>
                  </div>
                  <input type="checkbox" v-model="currentVideoEnabled" class="toggle toggle-info toggle-xs" />
              </label>
              <label class="flex items-center justify-between p-4 bg-[#161a23] border border-white/5 rounded-2xl cursor-pointer hover:border-amber-400 transition-all group">
                  <div class="flex items-center gap-2">
                       <FileText class="w-4 h-4 text-slate-500 group-hover:text-amber-400 transition-colors" />
                       <span class="text-xs font-black uppercase text-slate-500 group-hover:text-slate-300">Docs</span>
                  </div>
                  <input type="checkbox" v-model="currentDocumentEnabled" class="toggle toggle-warning toggle-xs" />
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
                  <input v-model="newBot.api_key" type="text" autocomplete="off" class="input-premium h-14 w-full font-mono text-xs" placeholder="Paste manual key..." />
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
                  <input v-model="newBot.chatwoot_bot_token" type="text" autocomplete="off" class="input-premium h-14 w-full font-mono text-xs" placeholder="Paste agent token..." />
              </div>
          </div>
      </div>

      <!-- TAB: MCP -->
      <div v-if="activeTab === 'mcp'" class="space-y-10 animate-in fade-in slide-in-from-right-4">
          <div class="section-title-premium text-primary/60 flex justify-between items-center mt-6">
              Tool Access Registry {{ selectedVariant !== '' ? '(Variant Scope)' : '' }}
              <span v-if="loadingBotMCPs" class="loading loading-spinner loading-xs"></span>
          </div>
          
          <div v-if="selectedVariant !== ''" class="mb-4 p-4 bg-indigo-500/5 border border-indigo-500/10 rounded-2xl flex items-start gap-4">
              <div class="w-6 h-6 rounded-full bg-indigo-500/10 flex items-center justify-center shrink-0">
                  <Wrench class="w-3 h-3 text-indigo-400" />
              </div>
              <div>
                  <h4 class="text-xs font-bold text-indigo-400 mb-1">Sub-Role Tool Scope</h4>
                  <p class="text-[11px] text-slate-400 leading-relaxed">
                      Toggle which tools THIS VARIANT explicitly has access to. When acting as this variant, the AI will only see the 'Allowed' tools you configure below.
                  </p>
              </div>
          </div>

          <div v-if="botMCPServers.length === 0" class="p-20 bg-white/[0.02] border border-dashed border-white/5 rounded-[2rem] text-center">
              <p class="text-xs font-bold uppercase tracking-widest text-slate-600">No MCP Servers detected.</p>
          </div>

          <div v-else class="space-y-6">
              <div v-for="srv in botMCPServers" :key="srv.id" 
                   class="rounded-[2rem] bg-[#161a23] border transition-all"
                   :class="srv.enabled ? 'border-primary/40' : 'border-white/5 opacity-60'">
                  
                  <div class="p-6 flex items-center justify-between">
                      <div class="flex items-center gap-4">
                           <div class="w-10 h-10 rounded-xl bg-white/5 flex items-center justify-center text-primary border border-white/10" :class="{ 'opacity-30 grayscale': selectedVariant !== '' }">
                               <Zap class="w-5 h-5" />
                           </div>
                           <div class="flex flex-col" :class="{ 'opacity-30 grayscale': selectedVariant !== '' }">
                               <div class="flex items-center gap-2">
                                   <h6 class="text-xs font-black text-white uppercase tracking-tight">{{ srv.name }}</h6>
                                   <div v-if="srv.is_template" class="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-500 text-xs font-black uppercase tracking-widest border border-amber-500/20">
                                       Template
                                   </div>
                               </div>
                               <span class="text-xs font-mono text-slate-600 truncate max-w-[200px]">{{ srv.url }}</span>
                           </div>
                      </div>
                      <div class="flex items-center gap-4">
               <button @click="toggleServerExpansion(srv.id)" 
                                   class="btn-premium btn-premium-ghost btn-premium-sm px-4"
                                   :class="{ 'bg-primary/20 text-primary border-primary/30': expandedServers.includes(srv.id) }">
                               <Settings class="w-3 h-3" />
                           </button>
                           <input type="checkbox" :checked="selectedVariant !== '' ? isServerEnabledForVariant(srv.id) : srv.enabled" @change="toggleServer(srv)" class="toggle toggle-primary toggle-sm" />
                       </div>
                   </div>

                  <div v-if="expandedServers.includes(srv.id)" class="px-6 pb-8 border-t border-white/5 pt-6 space-y-6">
                      <div class="form-control">
                          <label class="label-premium opacity-50">Local Instructions</label>
                          <textarea v-model="srv.botInstructions" rows="2" class="input-premium w-full text-xs" placeholder="Guidelines for this bot..."></textarea>
                      </div>

                      <!-- Required Header Configuration for Templates -->
                      <div v-if="(selectedVariant === '') && ((srv.is_template && srv.template_config && Object.keys(srv.template_config).length) || (srv.urlVarKeys && srv.urlVarKeys.length))" class="space-y-6 p-8 bg-amber-500/5 rounded-[2rem] border border-amber-500/10 shadow-xl">
                          <div class="flex items-center gap-3 mb-2">
                              <ShieldCheck class="w-4 h-4 text-amber-500" />
                              <h5 class="text-xs font-black text-amber-500 uppercase tracking-widest">Required Configuration (Bot Specific)</h5>
                          </div>
                          
                          <div class="grid grid-cols-1 gap-5">
                              <!-- Dynamic URL Variables -->
                              <div v-for="key in srv.urlVarKeys" :key="'var-'+key" class="form-control">
                                  <label class="text-xs font-bold text-amber-500/50 uppercase ml-1 mb-2">Endpoint Variable: {{ key }}</label>
                                  <input v-model="srv.urlVariables[key]" type="text" 
                                         class="input-premium h-14 w-full bg-black/40 border-amber-500/10 text-xs text-white" 
                                         :placeholder="'Value for {' + key + '}'" />
                              </div>

                              <div v-for="(help, key) in srv.template_config" :key="key" class="form-control">
                                  <label class="text-xs font-bold text-amber-500/50 uppercase ml-1 mb-2">Header: {{ key }}</label>
                                  <input v-model="srv.headersMap[key]" type="text" 
                                         class="input-premium h-14 w-full bg-black/40 border-amber-500/10 text-xs text-white" 
                                         :placeholder="help" />
                              </div>
                          </div>
                          
                           <div class="flex flex-col items-end gap-3 pt-2">
                               <div v-if="connectionMessages[srv.id] && testingConnection[srv.id] === 'error'" 
                                    class="text-xs font-bold uppercase tracking-widest text-error">
                                   {{ connectionMessages[srv.id] }}
                               </div>
                               <div class="flex items-center gap-4">
                                   <span class="text-xs font-bold text-slate-600 uppercase tracking-widest italic pr-2">Configurations will be saved with the blueprint</span>
                                   <button @click="testMCPConnection(srv)" 
                                           class="btn-premium btn-premium-ghost btn-premium-sm border-amber-500/20 px-6 transition-all"
                                           :class="{
                                              'text-amber-500 hover:bg-amber-500/10': testingConnection[srv.id] === 'idle' || !testingConnection[srv.id],
                                              'text-success border-success/40 bg-success/5': testingConnection[srv.id] === 'success',
                                              'text-error border-error/40 bg-error/5': testingConnection[srv.id] === 'error',
                                              'opacity-50 cursor-wait': testingConnection[srv.id] === 'loading'
                                           }"
                                           :disabled="testingConnection[srv.id] === 'loading'">
                                       <span v-if="testingConnection[srv.id] === 'loading'" class="loading loading-spinner loading-xs mr-2"></span>
                                       <CheckCircle2 v-else-if="testingConnection[srv.id] === 'success'" class="w-3 h-3 mr-2" />
                                       <Activity v-else class="w-3 h-3 mr-2" />
                                       {{ testingConnection[srv.id] === 'success' ? 'Verified' : testingConnection[srv.id] === 'error' ? 'Retry Test' : 'Test Configuration' }}
                                   </button>
                               </div>
                           </div>
                      </div>

                      <div v-if="srv.tools && srv.tools.length" class="grid grid-cols-1 md:grid-cols-2 gap-3">
                          <div v-for="tool in srv.tools" :key="tool.name" 
                               @click="toggleToolForBot(srv, tool.name)"
                               class="flex items-center justify-between p-4 bg-black/20 border border-white/5 rounded-2xl cursor-pointer hover:bg-black/40 transition-all"
                               :class="{ 'opacity-40 grayscale': isToolDisabled(srv, tool.name) }">
                              <div class="flex items-center gap-3">
                                  <Zap class="w-3 h-3 text-primary" />
                                  <span class="text-xs font-black text-white uppercase">{{ tool.name }}</span>
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
</template>
