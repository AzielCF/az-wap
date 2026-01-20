<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'
import AppModal from '@/components/AppModal.vue'
import { 
  Zap, 
  Trash2, 
  Edit3, 
  Activity, 
  Search, 
  Settings2, 
  ShieldAlert, 
  Heart, 
  Terminal, 
  Globe, 
  Wrench,
  ChevronRight,
  ShieldCheck,
  X,
  PlusCircle,
  Save,
  Trash,
  Brain
} from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const mcpServers = ref<any[]>([])
const healthStatus = ref<Record<string, any>>({})
const refreshInterval = ref<any>(null)
const checkingHealth = ref<Record<string, boolean>>({})

// MCP Management
const showAddMCP = ref(false)
const editingMCP = ref<any>(null)
const newMCP = ref({
  name: '',
  description: '',
  type: 'http',
  url: '',
  command: '',
  args: [] as string[],
  headers: {} as Record<string, string>,
  enabled: true,
  is_template: false,
  template_config: {} as Record<string, string>,
  instructions: '',
  bot_instructions: ''
})

// Helper strings for UI
const headersString = ref('')
const templateHeadersList = ref<{ key: string, help: string }[]>([])

// Tools Monitor
const showTools = ref(false)
const selectedServerTools = ref<any[]>([])
const selectedServerName = ref('')
const loadingTools = ref(false)

async function loadData() {
  if (!refreshInterval.value) loading.value = true
  try {
    const [mcps, health] = await Promise.all([
      api.get('/api/mcp/servers'),
      api.get('/api/health/status')
    ])
    
    mcpServers.value = mcps?.results || []
    
    const hMap: Record<string, any> = {}
    if (health && health.results) {
      health.results.forEach((h: any) => {
        if (h.entity_type === 'mcp_server') {
          hMap[`mcp_server:${h.entity_id}`] = h
        }
      })
    }
    healthStatus.value = hMap
    
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function checkHealth(srv: any) {
  checkingHealth.value[srv.id] = true
  try {
    const res = await api.post(`/api/health/mcp/${srv.id}/check`, {})
    await loadData()
    if (res.results?.status === 'ERROR') {
      alert(`Health check failed: ${res.results.last_message}`)
    }
  } catch (err) {
    alert('Failed to trigger health check')
  } finally {
    checkingHealth.value[srv.id] = false
  }
}

async function fetchTools(server: any) {
  loadingTools.value = true
  selectedServerName.value = server.name
  selectedServerTools.value = []
  showTools.value = true
  try {
    const res = await api.get(`/api/mcp/servers/${server.id}/tools`)
    selectedServerTools.value = res.results || []
  } catch (err) {
    console.error('Failed to fetch tools', err)
  } finally {
    loadingTools.value = false
  }
}

async function saveMCP() {
  try {
    // Parse headers
    const headers: Record<string, string> = {}
    headersString.value.split('\n').forEach(line => {
      const parts = line.split(':')
      if (parts.length >= 2 && parts[0]) {
        const key = parts[0].trim()
        const val = parts.slice(1).join(':').trim()
        if (key) headers[key] = val
      }
    })
    newMCP.value.headers = headers

    // Parse template config
    const tplConfig: Record<string, string> = {}
    if (newMCP.value.is_template) {
      templateHeadersList.value.forEach(h => {
        if (h.key) tplConfig[h.key] = h.help || ''
      })
    }
    newMCP.value.template_config = tplConfig

    if (editingMCP.value) {
      await api.put(`/api/mcp/servers/${editingMCP.value.id}`, newMCP.value)
    } else {
      await api.post('/api/mcp/servers', newMCP.value)
    }
    showAddMCP.value = false
    await loadData()
  } catch (err) {
    alert('Failed to save configuration.')
  }
}

function openAdd() {
  editingMCP.value = null
  resetForm()
  showAddMCP.value = true
}

function openEdit(mcp: any) {
  editingMCP.value = mcp
  newMCP.value = {
    ...mcp,
    headers: mcp.headers || {},
    template_config: mcp.template_config || {},
    args: mcp.args || []
  }
  
  // Prep UI helpers
  headersString.value = Object.entries(newMCP.value.headers)
    .map(([k, v]) => `${k}: ${v}`)
    .join('\n')
    
  templateHeadersList.value = Object.entries(newMCP.value.template_config)
    .map(([k, v]) => ({ key: k, help: v as string }))
    
  showAddMCP.value = true
}

async function deleteMCP(id: string) {
  if (!confirm('Destroy this capability node?')) return
  try {
    await api.delete(`/api/mcp/servers/${id}`)
    await loadData()
  } catch (err) {
    alert('Failed to delete.')
  }
}

function resetForm() {
  newMCP.value = {
    name: '',
    description: '',
    type: 'http',
    url: '',
    command: '',
    args: [],
    headers: {},
    enabled: true,
    is_template: false,
    template_config: {},
    instructions: '',
    bot_instructions: ''
  }
  headersString.value = ''
  templateHeadersList.value = []
}

function addTemplateHeader() {
  templateHeadersList.value.push({ key: '', help: '' })
}

function removeTemplateHeader(idx: number) {
  templateHeadersList.value.splice(idx, 1)
}

function getHealth(id: string) {
  return healthStatus.value[`mcp_server:${id}`] || { status: 'UNKNOWN', last_message: 'Waiting for heartbeat...' }
}

onMounted(() => {
  loadData()
  refreshInterval.value = setInterval(loadData, 5000)
})

onUnmounted(() => {
  if (refreshInterval.value) clearInterval(refreshInterval.value)
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
          <Terminal class="w-4 h-4 text-primary" />
          <span class="section-title-premium py-0 border-none pl-0 text-primary">MCP Network</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-[10px] font-bold uppercase tracking-[0.25em] text-slate-500">Node Management</span>
        </div>
        <h2 class="text-6xl font-black tracking-tighter text-white uppercase leading-none">MCP Manager</h2>
      </div>
      
      <button class="btn-premium btn-premium-primary px-12" @click="openAdd">
         <PlusCircle class="w-4 h-4" />
         Connect New Node
      </button>
    </div>

    <!-- Main Grid -->
    <div class="px-6 lg:px-0">
        <div class="section-title-premium text-primary/60">Active capability connectors</div>

        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-10">
            <div v-for="mcp in mcpServers" :key="mcp.id" class="bg-[#161a23] border border-white/5 rounded-[2.5rem] p-10 hover:border-primary/30 transition-all duration-500 group relative flex flex-col min-h-[380px] shadow-2xl overflow-hidden">
                
                <!-- ID/Header Tag -->
                <div class="flex justify-between items-start mb-10">
                    <div class="flex flex-col gap-1">
                        <div class="flex items-center gap-3">
                            <span class="text-xs font-mono text-slate-600 uppercase tracking-widest">{{ mcp.id.substring(0,8) }}</span>
                            <div class="flex items-center gap-2 px-2 py-1 rounded bg-white/5 border border-white/10">
                                <span class="text-[10px] font-black uppercase text-slate-400">{{ mcp.type }}</span>
                            </div>
                        </div>
                    </div>
                    <div class="flex gap-2">
                        <button class="btn-card-action" @click="openEdit(mcp)">
                            <Edit3 class="w-4 h-4" />
                        </button>
                        <button class="btn-card-action btn-card-action-red" @click="deleteMCP(mcp.id)">
                            <Trash2 class="w-4 h-4" />
                        </button>
                    </div>
                </div>

                <div class="relative z-10 flex flex-col h-full">
                    <div class="flex items-center gap-3 mb-3">
                        <h4 class="text-2xl font-black text-white uppercase tracking-tighter group-hover:text-primary transition-colors leading-none">{{ mcp.name }}</h4>
                        <div v-if="mcp.is_template" class="px-2 py-1 rounded bg-amber-500/10 text-amber-500 text-[10px] font-black uppercase tracking-widest border border-amber-500/20 flex items-center gap-1">
                            <Settings2 class="w-2.5 h-2.5" />
                            Template
                        </div>
                    </div>
                    
                    <p class="text-xs text-slate-500 font-bold uppercase tracking-tight mb-6 line-clamp-1 opacity-60">{{ mcp.url || mcp.command }}</p>
                    
                    <div class="p-6 bg-black/20 rounded-2xl border border-white/5 space-y-3 mb-8">
                        <div class="flex items-center justify-between">
                            <div class="flex items-center gap-2">
                                <Activity class="w-3 h-3 text-slate-500" />
                                <span class="text-[9px] font-black uppercase text-slate-500 tracking-widest">Health Record</span>
                            </div>
                            <div class="flex items-center gap-2">
                                <div class="w-2 h-2 rounded-full animate-pulse" :class="getHealth(mcp.id).status === 'OK' ? 'bg-success shadow-[0_0_10px_rgba(34,197,94,0.4)]' : 'bg-error'"></div>
                                <span class="text-xs font-black uppercase" :class="getHealth(mcp.id).status === 'OK' ? 'text-success' : 'text-error'">{{ getHealth(mcp.id).status }}</span>
                            </div>
                        </div>
                        <p class="text-xs text-slate-400 font-medium leading-relaxed italic line-clamp-2 h-10">{{ getHealth(mcp.id).last_message }}</p>
                        
                        <!-- Tool Count Indicator -->
                        <div v-if="mcp.tools && mcp.tools.length" class="pt-2 border-t border-white/5 flex items-center justify-between">
                            <div class="flex items-center gap-2">
                                <Wrench class="w-3 h-3 text-slate-600" />
                                <span class="text-[10px] font-black uppercase text-slate-600 tracking-widest">Capabilty Set</span>
                            </div>
                            <span class="text-xs font-black text-success">{{ mcp.tools.length }} Tools Loaded</span>
                        </div>
                    </div>

                    <div class="mt-auto flex items-center gap-3">
                        <button @click="fetchTools(mcp)" class="btn-premium btn-premium-ghost flex-1">
                            <Search class="w-3.5 h-3.5 mr-2" />
                            List Capabilites
                        </button>
                        <button @click="checkHealth(mcp)" class="btn-card-action group/health" :disabled="checkingHealth[mcp.id]">
                            <Heart class="w-4 h-4 transition-transform group-hover/health:scale-110" :class="{ 'animate-pulse text-error': !checkingHealth[mcp.id] && getHealth(mcp.id).status === 'ERROR', 'animate-spin': checkingHealth[mcp.id] }" />
                        </button>
                    </div>
                </div>
            </div>
            
            <!-- Empty State -->
            <div v-if="mcpServers.length === 0" class="xl:col-span-3 py-20 bg-white/[0.02] border-2 border-dashed border-white/5 rounded-[2.5rem] flex flex-col items-center justify-center text-center px-10">
                <div class="w-20 h-20 rounded-full bg-white/5 flex items-center justify-center text-slate-700 mb-6">
                    <Zap class="w-10 h-10" />
                </div>
                <h3 class="text-xl font-black text-white uppercase tracking-tight mb-2">No Capability Nodes</h3>
                <p class="text-xs text-slate-500 max-w-sm uppercase font-bold tracking-tight">Expand the bot intelligence by connecting specialized external micro-servers.</p>
            </div>
        </div>
    </div>

    <!-- Modals -->
    
    <!-- MCP Tools Inspector -->
    <AppModal v-model="showTools" :title="'Protocol Inspector: ' + selectedServerName" maxWidth="max-w-4xl" noPadding noScroll>
        <div class="flex flex-col h-[70vh] overflow-hidden">
            <div class="flex-1 overflow-y-auto p-12 custom-scrollbar bg-[#0b0e14] space-y-8">
                <div v-if="loadingTools" class="py-20 flex flex-col items-center justify-center gap-6">
                    <span class="loading loading-spinner loading-lg text-primary"></span>
                    <span class="text-xs font-black uppercase tracking-widest text-slate-600">Syncing definitions...</span>
                </div>
                <div v-else-if="selectedServerTools.length === 0" class="py-20 text-center text-slate-500 uppercase font-black text-xs tracking-widest">
                    <ShieldAlert class="w-8 h-8 mx-auto mb-4 opacity-20" />
                    No tools exposed by this server
                </div>
                <div v-else v-for="tool in selectedServerTools" :key="tool.name" class="p-8 bg-[#161a23] border border-white/5 rounded-3xl space-y-4 shadow-xl">
                    <div class="flex items-center justify-between border-b border-white/5 pb-4">
                        <div class="flex items-center gap-3">
                            <div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center text-primary border border-primary/20">
                                <Wrench class="w-4 h-4" />
                            </div>
                            <h5 class="text-lg font-black text-white uppercase tracking-tight">{{ tool.name }}</h5>
                        </div>
                    </div>
                    <p class="text-xs text-slate-400 leading-relaxed font-medium uppercase italic">{{ tool.description || 'No specialized description.' }}</p>
                    <div class="pt-4">
                        <div class="text-[9px] font-black text-slate-600 uppercase tracking-widest mb-3 flex items-center gap-2">
                            <Terminal class="w-3 h-3" />
                            Input Specification
                        </div>
                        <pre class="bg-black/40 p-6 rounded-2xl text-[10px] font-mono text-slate-500 overflow-x-auto border border-white/5">{{ JSON.stringify(tool.input_schema, null, 2) }}</pre>
                    </div>
                </div>
            </div>
            
            <!-- Fixed Footer -->
            <div class="flex-none p-8 border-t border-white/5 bg-[#0b0e14] flex justify-end">
                <button class="btn-premium btn-premium-ghost px-12" @click="showTools = false">
                    <ChevronRight class="w-4 h-4 mr-2" />
                    Dismiss Protocol
                </button>
            </div>
        </div>
    </AppModal>

    <!-- MCP Editor Modal -->
    <AppModal v-model="showAddMCP" :title="editingMCP ? 'Configure Node' : 'Bridge New Connector'" maxWidth="max-w-6xl" noPadding noScroll>
        <div class="flex flex-col lg:flex-row bg-[#0b0e14] h-[85vh] overflow-hidden">
            <div class="lg:w-80 bg-[#161a23] border-r border-white/5 p-12 flex flex-col items-center text-center flex-none">
                <div class="w-24 h-24 rounded-[2rem] bg-primary/10 flex items-center justify-center text-primary mb-8 border border-primary/20 shadow-2xl relative">
                    <Zap class="w-12 h-12" />
                    <div class="absolute -bottom-2 -right-2 w-8 h-8 rounded-full bg-success flex items-center justify-center text-white text-[10px] font-black border-4 border-[#161a23]">v2</div>
                </div>
                <h4 class="text-xl font-black text-white uppercase tracking-tighter mb-4">{{ editingMCP ? 'Modify Port' : 'Establish Link' }}</h4>
                <p class="text-xs text-slate-500 font-bold uppercase tracking-widest leading-relaxed opacity-60">Interface definition for external brain extensions.</p>
                
                <div class="mt-auto w-full space-y-4 pt-20">
                    <div class="p-5 bg-white/5 rounded-2xl border border-white/5 text-left">
                        <div class="flex items-center gap-2 mb-2">
                            <ShieldCheck class="w-3.5 h-3.5 text-primary" />
                            <span class="text-xs font-black text-slate-400 uppercase tracking-widest">Protocol</span>
                        </div>
                        <p class="text-xs text-slate-600 leading-tight uppercase font-bold">Standard JSON-RPC over modern transport layers.</p>
                    </div>
                </div>
            </div>
            
            <div class="flex-1 flex flex-col overflow-hidden bg-[#0b0e14]">
                <div class="flex-1 p-12 lg:p-16 space-y-12 overflow-y-auto custom-scrollbar">
                    <div class="grid grid-cols-2 gap-10">
                        <div class="form-control">
                            <label class="label-premium text-primary">Connector Identity</label>
                            <input v-model="newMCP.name" type="text" class="input-premium h-16 w-full text-lg font-black" placeholder="e.g. Brain Database" />
                        </div>
                        <div class="form-control">
                            <label class="label-premium text-slate-400">Transport Type</label>
                            <div class="grid grid-cols-2 gap-3 h-16">
                                <button @click="newMCP.type = 'http'" class="btn-premium btn-premium-ghost rounded-xl border transition-all font-black text-[10px] uppercase tracking-widest flex items-center justify-center gap-2" 
                                    :class="newMCP.type === 'http' ? 'bg-primary border-primary text-white shadow-lg shadow-primary/20' : 'bg-white/5 border-white/10 text-slate-500'">
                                    <Globe class="w-4 h-4" />
                                    HTTP
                                </button>
                                <button @click="newMCP.type = 'sse'" class="btn-premium btn-premium-ghost rounded-xl border transition-all font-black text-[10px] uppercase tracking-widest flex items-center justify-center gap-2"
                                    :class="newMCP.type === 'sse' ? 'bg-primary border-primary text-white shadow-lg shadow-primary/20' : 'bg-white/5 border-white/10 text-slate-500'">
                                    <Zap class="w-4 h-4" />
                                    SSE
                                </button>
                            </div>
                        </div>
                    </div>

                    <div class="form-control">
                        <label class="label-premium">Endpoint access URL</label>
                        <div class="relative">
                            <Globe class="absolute left-5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-600" />
                            <input v-model="newMCP.url" type="text" class="input-premium h-16 w-full font-mono text-sm pl-14" placeholder="https://api.yournodes.com/sse/mcp" />
                        </div>
                    </div>

                    <div class="form-control">
                        <label class="label-premium">Authentication & Proxy Headers (Optional)</label>
                        <textarea v-model="headersString" rows="3" class="input-premium w-full p-6 font-mono text-xs leading-relaxed" placeholder="Authorization: Bearer xyz...&#10;X-MCP-Token: custom-auth..."></textarea>
                        <p class="mt-3 text-[10px] text-slate-600 font-bold uppercase tracking-wider">Line by line: Key: Value</p>
                    </div>

                    <div class="grid grid-cols-1 gap-10">
                        <label class="flex items-center justify-between h-20 bg-[#161a23] border border-white/10 rounded-2xl px-8 cursor-pointer hover:border-amber-500/40 transition-all group">
                            <div class="flex items-center gap-4">
                                <div class="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-500 group-hover:scale-110 transition-transform">
                                    <Settings2 class="w-5 h-5" />
                                </div>
                                <div class="flex flex-col">
                                    <span class="text-[11px] font-black uppercase tracking-widest text-slate-400">Is Capability Template</span>
                                    <span class="text-[9px] text-slate-600 font-bold uppercase">Require credentials per individual bot</span>
                                </div>
                            </div>
                            <input type="checkbox" v-model="newMCP.is_template" class="toggle toggle-warning" />
                        </label>
                    </div>

                    <!-- Template Settings -->
                    <div v-if="newMCP.is_template" class="space-y-8 animate-in fade-in slide-in-from-top-4 duration-500 p-10 bg-amber-500/5 rounded-[2.5rem] border border-amber-500/10 shadow-2xl">
                        <div class="flex items-center justify-between mb-2">
                            <div class="flex items-center gap-3">
                                <ShieldCheck class="w-4 h-4 text-amber-500" />
                                <h5 class="text-xs font-black text-amber-500 uppercase tracking-widest">Required Configuration (Bot Specific)</h5>
                            </div>
                            <button @click="addTemplateHeader" class="btn-premium btn-premium-ghost btn-premium-sm border-amber-500/20 text-amber-500 hover:bg-amber-500/10">
                                <PlusCircle class="w-4 h-4" />
                                Add Rule
                            </button>
                        </div>
                        
                        <div v-if="templateHeadersList.length === 0" class="py-10 text-center border border-dashed border-amber-500/20 rounded-3xl">
                            <p class="text-[9px] font-bold text-amber-500/40 uppercase tracking-[0.2em]">No credential requirements defined yet.</p>
                        </div>

                        <div v-for="(th, idx) in templateHeadersList" :key="idx" class="flex gap-6 items-end animate-in fade-in scale-in duration-300">
                            <div class="flex-1 space-y-3">
                                 <label class="text-[9px] font-bold text-amber-500/40 uppercase ml-1">Header Name</label>
                                 <input v-model="th.key" type="text" class="input-premium h-14 w-full bg-black/40 border-amber-500/10" placeholder="e.g. X-User-ID" />
                            </div>
                            <div class="flex-1 space-y-3">
                                 <label class="text-[9px] font-bold text-amber-500/40 uppercase ml-1">Helper Instruction</label>
                                 <input v-model="th.help" type="text" class="input-premium h-14 w-full bg-black/40 border-amber-500/10" placeholder="Ask user for their ID..." />
                            </div>
                            <button @click="removeTemplateHeader(idx)" class="btn-card-action btn-card-action-red">
                                <Trash class="w-5 h-5" />
                            </button>
                        </div>
                    </div>

                    <!-- Technical Description -->
                    <div class="form-control">
                        <div class="flex items-center gap-2 mb-3">
                            <Brain class="w-4 h-4 text-primary" />
                            <label class="label-premium mb-0">Universal AI System Instructions</label>
                        </div>
                        <textarea v-model="newMCP.instructions" rows="5" class="input-premium w-full p-8 leading-relaxed text-sm" placeholder="Define boundaries and usage patterns for the AI..."></textarea>
                        <p class="mt-3 text-[10px] text-slate-600 font-bold uppercase tracking-wider">These guidelines are injected into the bot's system prompt whenever this node is active.</p>
                    </div>
                </div>

                <!-- Fixed Footer -->
                <div class="flex-none p-10 lg:px-16 border-t border-white/5 flex justify-end gap-6 bg-[#0b0e14]">
                    <button class="btn-premium btn-premium-ghost px-12 h-14" @click="showAddMCP = false">
                        <X class="w-4 h-4 mr-2" />
                        Discard changes
                    </button>
                    <button class="btn-premium btn-premium-success px-20 h-14" @click="saveMCP">
                        <Save class="w-4 h-4 mr-2" />
                        Authorize node
                    </button>
                </div>
            </div>
        </div>
    </AppModal>
  </div>
</template>

<style scoped>
</style>

