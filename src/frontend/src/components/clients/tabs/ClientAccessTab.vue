<script setup lang="ts">
import { computed, ref } from 'vue'
import { ShieldCheck, Globe, Search, Bot, Plus, Trash2, Building2 } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: any
  editingClient: any
  clientChannels: any[]
  bots: any[]
}>()

const emit = defineEmits(['update:modelValue', 'resolve-bots'])

const client = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const enableWorkspaces = computed({
  get: () => {
    if (!client.value.tags) return false
    return client.value.tags.includes('enable_workspaces')
  },
  set: (val: boolean) => {
    if (!client.value.tags) client.value.tags = []
    if (val) {
      if (!client.value.tags.includes('enable_workspaces')) {
        client.value.tags.push('enable_workspaces')
      }
    } else {
      client.value.tags = client.value.tags.filter((t: string) => t !== 'enable_workspaces')
    }
  }
})

const botSearch = ref('')
const showBotResults = ref(false)

const unselectedBots = computed(() => {
  if (!client.value.allowed_bots) return []
  return props.bots.filter(b => !client.value.allowed_bots.includes(b.id))
})

const filteredBots = computed(() => {
  if (!botSearch.value) return unselectedBots.value.slice(0, 3)
  const s = botSearch.value.toLowerCase()
  return unselectedBots.value.filter(b => 
    (b.name && b.name.toLowerCase().includes(s)) || 
    (b.id && b.id.toLowerCase().includes(s))
  ).slice(0, 3)
})

function addAllowedBot(botId: string) {
  if (!botId) return
  const idToAssign = botId.trim()
  if (!client.value.allowed_bots) {
    client.value.allowed_bots = []
  }
  if (!client.value.allowed_bots.includes(idToAssign)) {
    client.value.allowed_bots.push(idToAssign)
    emit('resolve-bots', [idToAssign])
  }
  botSearch.value = ''
  showBotResults.value = false
}

function removeAllowedBot(botId: string) {
  if (!client.value.allowed_bots) return
  client.value.allowed_bots = client.value.allowed_bots.filter((id: string) => id !== botId)
}

function getBotById(id: string) {
  return props.bots.find(b => b.id === id) || { name: 'Unknown Bot', id }
}
</script>

<template>
  <div class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
    <header>
      <h3 class="text-xl font-black text-white uppercase tracking-tight">System & Access</h3>
      <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Bot control and debug mode</p>
    </header>

    <div class="p-6 bg-amber-500/5 border border-amber-500/20 rounded-2xl flex items-start gap-4">
      <div class="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-500 shrink-0">
        <ShieldCheck class="w-5 h-5" />
      </div>
      <div class="flex-1">
        <div class="flex items-center justify-between mb-2">
          <label class="text-sm font-black text-white uppercase tracking-tight">Tester Mode</label>
          <input type="checkbox" v-model="client.is_tester" class="toggle toggle-warning toggle-sm" />
        </div>
        <p class="text-xs text-slate-400 font-medium leading-relaxed">
          When enabled, all interactions from this client (messages, tool inputs, files) will be logged 
          <span class="text-amber-500 font-bold">UNFILTERED (FULL DATA)</span> for debugging purposes. 
          Use with caution and only for development accounts.
        </p>
      </div>
    </div>

    <!-- Gestión de Workspace -->
    <div class="p-6 mb-8 bg-blue-500/5 border border-blue-500/20 rounded-2xl flex items-start gap-4">
      <div class="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center text-blue-500 shrink-0">
        <Building2 class="w-5 h-5" />
      </div>
      <div class="flex-1">
        <div class="flex items-center justify-between mb-2">
          <label class="text-sm font-black text-white uppercase tracking-tight">Workspace Management</label>
          <input type="checkbox" v-model="enableWorkspaces" class="toggle toggle-info toggle-sm" />
        </div>
        <p class="text-xs text-slate-400 font-medium leading-relaxed">
          Allow this client to create and manage their own conversational workspaces and link channels natively from their UI.
        </p>
      </div>
    </div>

    <!-- Visualización dinámica de los Canales del Cliente -->
    <div class="form-control mb-8">
      <div class="flex items-center justify-between mb-4">
        <span class="text-xs font-black text-slate-600 uppercase tracking-widest">Assigned Channels</span>
      </div>
      
      <div v-if="!editingClient" class="py-10 text-center bg-black/20 rounded-[1.5rem] border border-dashed border-white/5">
        <p class="text-xs font-bold text-slate-600 uppercase tracking-widest px-6">Select a client first to view their infrastructure</p>
      </div>
      
      <!-- Lista reactiva de canales traídos on-demand -->
      <div v-else-if="clientChannels && clientChannels.length > 0" class="flex flex-col gap-3">
        <div v-for="channel in clientChannels" :key="'owned-' + channel.id" 
              class="flex items-center justify-between p-4 rounded-2xl bg-primary/10 border border-primary/20 shadow-lg shadow-primary/5">
          
          <div class="flex items-center gap-4">
            <div class="h-10 w-10 shrink-0 rounded-xl bg-primary flex items-center justify-center text-white shadow-inner">
              <Globe class="w-5 h-5" />
            </div>
            <div class="flex-1 min-w-0">
              <h4 class="text-sm font-black text-white uppercase truncate">{{ channel.name }}</h4>
              <p class="text-xs font-bold text-slate-500 uppercase flex gap-1 items-center truncate">
                {{ channel.type }} <span class="opacity-30">•</span> {{ channel.workspaceName || 'Assigned Channel' }}
              </p>
            </div>
          </div>
        </div>
        
        <div class="mt-4 p-4 bg-black/20 rounded-xl border border-white/5 text-center">
          <p class="text-xs text-slate-500 font-bold uppercase tracking-widest">
            To view details or remove channels, use the Workspace Tab.
          </p>
        </div>
      </div>
      <div v-else class="py-10 text-center bg-black/20 rounded-[1.5rem] border border-dashed border-white/5 flex flex-col items-center">
        <Globe class="w-8 h-8 text-slate-700 mb-3 opacity-20" />
        <p class="text-xs font-bold text-slate-600 uppercase tracking-widest">This client has no assigned channels</p>
        <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-2">
          To manage their spaces, go to the Workspaces tab.
        </p>
      </div>
    </div>

    <header>
      <h4 class="text-sm font-black text-slate-400 uppercase tracking-widest">Authorized Agents</h4>
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
          <div v-if="filteredBots.length === 0" class="py-4 text-center text-xs text-slate-600 font-black uppercase flex flex-col items-center gap-2">
            <span>Zero matching local agents</span>
            <button v-if="botSearch.length >= 10" @click="addAllowedBot(botSearch)" class="btn btn-sm btn-primary mt-2">
              Add by exact ID
            </button>
          </div>
          <button v-for="bot in filteredBots" :key="bot.id"
                  @click="addAllowedBot(bot.id)"
                  class="w-full flex items-center gap-4 p-3 hover:bg-primary/10 rounded-xl transition-all group text-left border border-transparent hover:border-primary/20">
            <div class="w-10 h-10 rounded-lg bg-black flex items-center justify-center text-slate-500 group-hover:text-primary transition-all shadow-inner">
              <Bot class="w-5 h-5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-xs font-black uppercase text-white truncate">{{ bot.name }}</div>
              <div class="text-xs font-mono text-slate-600 truncate uppercase">{{ (bot.id || '').substring(0,25) }}...</div>
            </div>
            <Plus class="w-4 h-4 text-slate-800 group-hover:text-primary" />
          </button>
        </div>
      </div>
      <div v-if="showBotResults" @click="showBotResults = false" class="fixed inset-0 z-10"></div>
    </div>

    <div class="flex-1 min-h-0 flex flex-col">
      <div class="flex items-center justify-between mb-4">
        <span class="text-xs font-black text-slate-600 uppercase tracking-widest">Currently Whitelisted</span>
        <span v-if="!client.allowed_bots || client.allowed_bots.length === 0" class="text-xs font-bold text-slate-500 uppercase italic">Unrestricted Global Access</span>
      </div>
      
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pb-20">
        <div v-for="botId in (client.allowed_bots || [])" :key="botId" 
             class="flex items-center justify-between p-4 bg-primary/5 border border-primary/10 rounded-2xl group border-l-4 border-l-primary/40 hover:bg-primary/[0.08] transition-all">
          <div class="flex items-center gap-4 min-w-0">
            <div class="w-10 h-10 rounded-xl bg-[#0b0e14] border border-white/5 flex items-center justify-center text-primary shadow-xl">
              <Bot class="w-5 h-5" />
            </div>
            <div class="min-w-0">
              <div class="text-xs font-black uppercase text-white truncate">{{ getBotById(botId).name }}</div>
              <div class="text-xs font-mono text-slate-500 truncate">{{ (botId || '').substring(0,12) }}...</div>
            </div>
          </div>
          <button @click="removeAllowedBot(botId)" class="p-2 text-slate-700 hover:text-red-500 transition-colors">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
