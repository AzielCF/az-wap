<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import { 
  Database, 
  Clock, 
  Zap, 
  CheckCircle, 
  AlertCircle, 
  Eye, 
  ChevronRight, 
  MessageSquare,
  Bot,
  Filter,
  Cpu,
  Server,
  Activity,
  Wrench,
  ArrowRight,
  Brain,
  AlertTriangle,
  RotateCw
} from 'lucide-vue-next'

const props = defineProps<{
  stats: any,
  autoSync: boolean
}>()

const emit = defineEmits(['toggle-sync'])

const api = useApi()
const healthStatus = ref<Record<string, any>>({})
const workspaces = ref<any[]>([])
const filterWorkspace = ref('')
const page = ref(1)
const pageSize = 15
const filterErrorsOnly = ref(false)
const filterInstance = ref('')
const filterChat = ref('')
const filterProvider = ref('')
const expandedTraces = ref<Record<string, boolean>>({})
const expandedSubEvents = ref<Record<string, boolean>>({})
const metaMap = ref<Record<string, { wsId: string, wsName: string, channelName: string }>>({})

async function loadWorkspaces() {
  try {
    const res = await api.get('/workspaces')
    workspaces.value = res || []
    
    // Fetch channels for all workspaces to build metadata map
    const promises = workspaces.value.map(async (ws: any) => {
        try {
            const channels = await api.get(`/workspaces/${ws.id}/channels`)
            if (channels && Array.isArray(channels)) {
                channels.forEach((ch: any) => {
                    // Map both the internal ID and external ref (instance_id)
                    const meta = { wsId: ws.id, wsName: ws.name, channelName: ch.name }
                    if (ch.external_ref) metaMap.value[ch.external_ref] = meta
                    // Also map by channel ID just in case
                    metaMap.value[ch.id] = meta
                })
            }
        } catch (e) {
            console.warn(`Failed to load channels for ws ${ws.id}`, e)
        }
    })
    
    await Promise.all(promises)
  } catch (err) {
    console.warn('Failed to load workspaces for filter:', err)
  }
}

async function fetchHealth() {
  try {
    const hData = await api.get('/api/health/status')
    const results = hData.results || []
    const botHealth: Record<string, any> = {}
    results.forEach((r: any) => {
      if (r.entity_type === 'bot') {
        botHealth[r.entity_id] = r
      }
    })
    healthStatus.value = botHealth
  } catch (err) {
    console.warn('Failed to fetch health for bot monitor:', err)
  }
}

watch(() => props.autoSync, (newVal) => {
  if (newVal) fetchHealth()
}, { immediate: true })

// Polling for health if sync is on
let healthInterval: any = null
onMounted(() => {
  healthInterval = setInterval(fetchHealth, 5000)
  fetchHealth()
  loadWorkspaces()
})
onUnmounted(() => {
  if (healthInterval) clearInterval(healthInterval)
})

const filteredEvents = computed(() => {
    if (!props.stats || !props.stats.recent_events) return []
    const ws = filterWorkspace.value.trim()
    const instance = filterInstance.value.trim().toLowerCase() // This is now 'Channel Name' filter
    const chat = filterChat.value.trim().toLowerCase()
    const provider = filterProvider.value.trim().toLowerCase()
    
    return props.stats.recent_events.filter((e: any) => {
        // Resolve Metadata
        const meta = metaMap.value[e.instance_id]
        
        // Filter by Workspace
        if (ws) {
            if (!meta || meta.wsId !== ws) return false
        }
        
        // Filter by Channel Name (fuzzy search)
        if (instance) {
            const name = meta ? meta.channelName.toLowerCase() : ''
            const id = (e.instance_id || '').toLowerCase()
            if (name.indexOf(instance) === -1 && id.indexOf(instance) === -1) return false
        }
        
        if (filterErrorsOnly.value && e.status !== 'error') return false
        if (chat && String(e.chat_jid || '').toLowerCase().indexOf(chat) === -1) return false
        if (provider && String(e.provider || '').toLowerCase().indexOf(provider) === -1) return false
        return true
    })
})

function getStepType(e: any): 'mcp' | 'ai' | 'inbound' | 'outbound' | 'other' {
    if (e.kind === 'mcp_call' || e.stage === 'mcp_call') return 'mcp'
    if (e.stage === 'ai_request' || e.kind === 'ai_request' || e.stage === 'ai_reply') return 'ai'
    if (e.stage === 'inbound' || e.kind === 'inbound') return 'inbound'
    if (e.stage === 'outbound' || e.kind === 'outbound') return 'outbound'
    return 'other'
}

const groupedTraces = computed(() => {
    const ev = [...filteredEvents.value]
    ev.sort((a: any, b: any) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    const map = new Map()
    for (const e of ev) {
        const trace = e.trace_id || `${e.instance_id}|${e.chat_jid}|${e.timestamp}`
        let g = map.get(trace)
        if (!g) {
            map.set(trace, {
                trace_id: trace,
                instance_id: e.instance_id,
                chat_jid: e.chat_jid,
                provider: e.provider,
                latest_ts: new Date(e.timestamp).getTime(),
                has_error: false,
                events: [],
                total_cost: 0
            })
            g = map.get(trace)
        }
        if (g) {
            const ts = new Date(e.timestamp).getTime()
            if (ts > g.latest_ts) g.latest_ts = ts
            if (e.status === 'error') g.has_error = true
            
            // Extract cost from metadata if present
            if (e.metadata) {
                const costStr = e.metadata.usage_cost || e.metadata.cost
                if (costStr && typeof costStr === 'string') {
                    const clean = costStr.replace('$', '')
                    const val = parseFloat(clean)
                    if (!isNaN(val)) g.total_cost += val
                }
            }
            
            g.events.push(e)
        }
    }
    const groups = Array.from(map.values())
    groups.forEach((g: any) => g.events.sort((a: any, b: any) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()))
    groups.sort((a: any, b: any) => b.latest_ts - a.latest_ts)
    return groups
})

const totalPages = computed(() => Math.max(1, Math.ceil(groupedTraces.value.length / pageSize)))
const pagedGroups = computed(() => groupedTraces.value.slice((page.value - 1) * pageSize, page.value * pageSize))

watch([filterErrorsOnly, filterInstance, filterChat, filterProvider, filterWorkspace], () => { page.value = 1 })

function toggleTrace(traceId: string) {
    expandedTraces.value = { ...expandedTraces.value, [traceId]: !expandedTraces.value[traceId] }
}

function toggleSubEvent(traceId: string, idx: number) {
    const key = `${traceId}-${idx}`
    expandedSubEvents.value = { ...expandedSubEvents.value, [key]: !expandedSubEvents.value[key] }
}

function parseMetadata(val: any) {
    try {
        if (typeof val === 'string' && (val.trim().startsWith('{') || val.trim().startsWith('['))) {
            return JSON.stringify(JSON.parse(val), null, 2)
        }
        if (typeof val === 'object') return JSON.stringify(val, null, 2)
        return val
    } catch (e) { return val }
}
</script>

<template>
  <div class="space-y-12">
    <!-- Health Warnings -->
    <div v-if="Object.keys(healthStatus).length" class="space-y-2 animate-in slide-in-from-top-4 duration-500">
        <template v-for="(h, bid) in healthStatus" :key="bid">
            <div v-if="h.status === 'ERROR'" class="flex items-center gap-4 p-3 bg-amber-500/10 border border-amber-500/20 rounded-lg">
                <AlertTriangle class="w-4 h-4 text-amber-500 flex-none" />
                <div class="flex-1 min-w-0">
                    <p class="text-xs font-bold text-amber-500/80 tracking-tight">
                        <span class="font-black uppercase mr-2">Bot Health Alert:</span>
                        {{ h.last_message }}
                    </p>
                </div>
            </div>
        </template>
    </div>

    <!-- Grouped Metrics -->
    <div v-if="stats" class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        <!-- Inbound Triggers -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl p-5 shadow-sm hover:border-primary/20 transition-all">
            <h3 class="text-2xl font-bold text-white mb-1">{{ stats.total_inbound || 0 }}</h3>
            <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Inbound triggers</p>
        </div>
        <!-- AI Requests -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl p-5 shadow-sm hover:border-primary/20 transition-all">
            <h3 class="text-2xl font-bold text-white mb-1">{{ stats.total_ai_requests || 0 }}</h3>
            <p class="text-xs font-bold uppercase tracking-widest text-slate-500">AI requests</p>
        </div>
        <!-- AI Replies -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl p-5 shadow-sm hover:border-primary/20 transition-all">
            <h3 class="text-2xl font-bold text-white mb-1">{{ stats.total_ai_replies || 0 }}</h3>
            <p class="text-xs font-bold uppercase tracking-widest text-slate-500">AI replies</p>
        </div>
        <!-- Outbound Sent -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl p-5 shadow-sm hover:border-primary/20 transition-all">
            <h3 class="text-2xl font-bold text-white mb-1">{{ stats.total_outbound || 0 }}</h3>
            <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Outbound sent</p>
        </div>
        <!-- Errors -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col hover:border-error/20 transition-all">
            <div class="p-5 flex-1">
                <h3 class="text-2xl font-bold text-white mb-1">{{ stats.total_errors || 0 }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Errors</p>
            </div>
            <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5 flex items-center justify-between">
                <AlertCircle class="w-3.5 h-3.5" :class="stats.total_errors > 0 ? 'text-error' : 'text-slate-700'" />
            </div>
        </div>
    </div>

    <!-- Bot Monitor Log -->
    <div class="space-y-6">
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4 border-l-4 border-primary pl-4">
            <div class="flex items-center gap-3">
                <Database class="w-5 h-5 text-primary" />
                <h3 class="text-lg font-bold text-white uppercase tracking-tight">Bot Event Log</h3>
            </div>
            <div class="flex flex-wrap items-center gap-3">
                <!-- Workspace Filter -->
                <select v-model="filterWorkspace" class="select select-sm bg-[#161a23] border-white/10 text-xs font-bold uppercase tracking-widest text-slate-400 w-44">
                    <option value="">All Workspaces</option>
                    <option v-for="ws in workspaces" :key="ws.id" :value="ws.id">{{ ws.name }}</option>
                </select>
                <button class="btn-premium btn-premium-ghost px-4" @click="emit('toggle-sync')">
                    <RotateCw class="w-3 h-3 mr-2" :class="autoSync ? 'animate-spin' : ''" />
                    <span class="text-xs font-bold uppercase tracking-widest">{{ autoSync ? 'Auto Sync: ON' : 'Auto Sync: OFF' }}</span>
                </button>
                <div class="relative">
                    <Filter class="absolute left-3 top-1/2 -translate-y-1/2 w-3 h-3 text-slate-600" />
                    <!-- Filter changed to Channel Name concept -->
                    <input v-model="filterInstance" placeholder="Channel" class="input-premium pl-9 h-10 w-32 text-xs" />
                </div>
                <div class="relative">
                    <Filter class="absolute left-3 top-1/2 -translate-y-1/2 w-3 h-3 text-slate-600" />
                    <input v-model="filterChat" placeholder="Chat JID" class="input-premium pl-9 h-10 w-32 text-xs" />
                </div>
                <label class="flex items-center gap-3 bg-[#161a23] border border-white/10 rounded-lg px-4 h-10 cursor-pointer hover:border-error/40 transition-colors">
                    <input type="checkbox" v-model="filterErrorsOnly" class="checkbox checkbox-xs checkbox-error" />
                    <span class="text-xs font-bold uppercase text-slate-500">Errors Only</span>
                </label>
            </div>
        </div>

        <div class="bg-[#161a23] border border-white/10 rounded-xl overflow-hidden shadow-2xl">
            <table class="table w-full table-fixed">
                <thead>
                    <tr class="text-[10px] text-slate-500 uppercase tracking-widest border-b border-white/5 bg-white/[0.01]">
                        <th class="py-4 pl-6 w-24">Time</th>
                        <th class="w-24">Stage</th>
                        <th class="w-24">Kind</th>
                        <th class="w-32">Model</th>
                        <th class="w-32">Channel</th>
                        <th class="w-24">Cost</th>
                        <th class="w-24">Status</th>
                        <th class="pr-6 text-right w-20">Action</th>
                    </tr>
                </thead>
                <tbody>
                    <template v-for="g in pagedGroups" :key="g.trace_id">
                        <!-- Trace Row (Header for the Group) -->
                        <tr @click="toggleTrace(g.trace_id)" class="hover:bg-white/[0.03] transition-colors border-b border-white/[0.05] cursor-pointer group bg-[#1a1e29]">
                            <td colspan="6" class="py-3 px-4">
                                <div class="flex items-center gap-4 text-xs font-mono text-slate-400">
                                    <ChevronRight class="w-4 h-4 text-slate-500 transition-transform" :class="expandedTraces[g.trace_id] ? 'rotate-90' : ''" />
                                    <span class="font-bold text-slate-300">Trace: {{ g.trace_id.substring(0, 15) }}...</span>
                                    <span class="text-slate-600">|</span>
                                    <span class="text-primary/70 font-bold uppercase">{{ metaMap[g.instance_id]?.channelName || g.instance_id?.substring(0,12) }}</span>
                                    <span class="text-slate-600">|</span>
                                    <span>{{ g.chat_jid }}</span>
                                    <div class="ml-auto flex items-center gap-4">
                                        <div v-if="g.total_cost > 0" class="flex items-center gap-1.5 px-2 py-1 bg-primary/10 border border-primary/20 rounded-md">
                                            <Zap class="w-3 h-3 text-primary" />
                                            <span class="text-[10px] font-black text-primary tracking-tighter">${{ g.total_cost.toFixed(6) }}</span>
                                        </div>
                                        <span class="text-[10px] uppercase font-bold tracking-widest text-slate-600">{{ g.events.length }} EVENTS</span>
                                    </div>
                                </div>
                            </td>
                        </tr>
                        
                        <!-- Expanded Events List -->
                        <template v-if="expandedTraces[g.trace_id]">
                             <template v-for="(e, idx) in g.events" :key="g.trace_id + '-e-' + idx">
                                <tr class="border-b border-white/[0.02] hover:bg-white/[0.015] transition-colors bg-[#11141b]">
                                    <!-- Time -->
                                    <td class="py-3 pl-12 text-xs font-mono text-slate-500 truncate">
                                        {{ new Date(e.timestamp).toLocaleTimeString() }}
                                    </td>
                                    
                                    <!-- Stage (Badges) -->
                                    <td class="py-3">
                                        <div v-if="getStepType(e) === 'mcp'" class="inline-flex items-center px-2 py-1 rounded bg-teal-500/10 border border-teal-500/20 text-teal-400 shadow-sm">
                                            <Wrench class="w-3 h-3 mr-1.5" />
                                            <span class="text-[10px] font-bold uppercase tracking-wide">mcp_call</span>
                                        </div>
                                        <div v-else-if="getStepType(e) === 'ai' && (e.stage === 'ai_request' || e.kind === 'ai_request')" class="inline-flex items-center px-2 py-1 rounded bg-blue-500/10 border border-blue-500/20 text-blue-400 shadow-sm">
                                            <Brain class="w-3 h-3 mr-1.5" />
                                            <span class="text-[10px] font-bold uppercase tracking-wide">ai_request</span>
                                        </div>
                                         <div v-else-if="e.stage === 'ai_reply'" class="text-xs font-bold text-slate-400 pl-2">
                                            ai_response
                                        </div>
                                        <div v-else-if="getStepType(e) === 'inbound'" class="text-xs font-bold text-slate-500 pl-2">
                                            inbound
                                        </div>
                                        <div v-else-if="getStepType(e) === 'outbound'" class="text-xs font-bold text-slate-500 pl-2">
                                            outbound
                                        </div>
                                        <div v-else class="text-xs font-bold text-slate-500 pl-2">
                                            {{ e.stage }}
                                        </div>
                                    </td>

                                    <!-- Kind / Tool Name -->
                                    <td class="py-3">
                                        <div v-if="getStepType(e) === 'mcp'" class="flex items-center gap-2">
                                            <Wrench class="w-3 h-3 text-teal-600" />
                                            <span class="text-sm font-bold text-slate-200 truncate">{{ e.kind || 'unknown_tool' }}</span>
                                        </div>
                                        <div v-else-if="e.kind" class="text-xs text-slate-500 font-medium truncate">
                                            {{ e.kind }}
                                        </div>
                                    </td>

                                    <!-- Model -->
                                    <td class="py-3">
                                        <span v-if="e.metadata?.model" class="text-[10px] font-bold text-primary/80 uppercase truncate block mr-4" :title="e.metadata.model">
                                            {{ e.metadata.model }}
                                        </span>
                                        <span v-else class="text-[10px] text-slate-700">-</span>
                                    </td>

                                    <!-- Channel Name (Resolved) -->
                                    <td class="py-3">
                                        <span class="text-xs font-bold text-slate-400 truncate block mr-4" :title="metaMap[e.instance_id]?.channelName || e.instance_id">
                                            {{ metaMap[e.instance_id]?.channelName || e.instance_id?.substring(0,12) }}
                                        </span>
                                    </td>

                                    <!-- Cost -->
                                    <td class="py-3">
                                        <span v-if="e.metadata && (e.metadata.usage_cost || e.metadata.cost)" class="text-[10px] font-mono font-bold text-slate-400">
                                            {{ e.metadata.usage_cost || e.metadata.cost }}
                                        </span>
                                        <span v-else class="text-[10px] text-slate-700">-</span>
                                    </td>

                                    <!-- Status Badge -->
                                    <td class="py-3">
                                        <div class="flex flex-col gap-1">
                                            <div class="w-fit px-2 py-0.5 rounded text-[9px] font-black uppercase tracking-wider" 
                                                :class="e.status === 'ok' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/20' : 'bg-red-500/20 text-red-400 border border-red-500/20'">
                                                {{ e.status }}
                                            </div>
                                            <span v-if="e.duration_ms" class="text-[9px] text-slate-600 font-mono">
                                                {{ e.duration_ms }} ms
                                            </span>
                                        </div>
                                    </td>

                                    <!-- Inspect -->
                                    <td class="py-3 pr-6 text-right">
                                        <button v-if="e.metadata" @click.stop="toggleSubEvent(g.trace_id, Number(idx))" 
                                                class="btn btn-xs bg-white/5 border-white/10 hover:bg-white/10 text-slate-400 hover:text-white normal-case transition-colors">
                                            <span class="mr-1 opacity-50">&lt;/&gt;</span>
                                        </button>
                                    </td>
                                </tr>
                                
                                <!-- Metadata Row -->
                                <tr v-if="expandedSubEvents[`${g.trace_id}-${idx}`]">
                                    <td colspan="6" class="p-0 bg-[#0b0e14] border-b border-white/[0.05]">
                                        <div class="p-6 ml-12 border-l border-white/5 space-y-4">
                                            <div v-if="e.kind === 'mcp_call'" class="mb-2 p-3 bg-teal-950/20 border border-teal-500/10 rounded-lg">
                                                <div class="flex items-center gap-2 mb-1">
                                                    <Wrench class="w-3 h-3 text-teal-500" />
                                                    <span class="text-[10px] font-bold text-teal-400 uppercase tracking-widest">MCP Call Details</span>
                                                </div>
                                            </div>

                                            <div v-for="(val, key) in e.metadata" :key="key" class="space-y-2">
                                                <div class="text-[10px] font-bold uppercase tracking-widest text-slate-600">{{ String(key).replace(/_/g, ' ') }}</div>
                                                <pre class="bg-black/40 p-3 rounded border border-white/5 text-[11px] font-mono text-slate-400 overflow-auto max-h-60 select-all custom-scrollbar">{{ parseMetadata(val) }}</pre>
                                            </div>
                                            
                                            <div v-if="e.error" class="bg-red-950/20 border border-red-500/20 p-3 rounded text-xs font-mono text-red-400">
                                                <strong>Error:</strong> {{ e.error }}
                                            </div>
                                        </div>
                                    </td>
                                </tr>
                             </template>
                        </template>
                    </template>
                </tbody>
            </table>
            <div class="flex items-center justify-between p-6 border-t border-white/5 bg-white/[0.01]">
                <div class="text-xs font-bold text-slate-600 uppercase tracking-widest">Page {{ page }} / {{ totalPages }}</div>
                <div class="flex gap-2">
                    <button class="btn-premium btn-premium-ghost btn-premium-sm" :disabled="page <= 1" @click="page--">PREV</button>
                    <button class="btn-premium btn-premium-ghost btn-premium-sm" :disabled="page >= totalPages" @click="page++">NEXT</button>
                </div>
            </div>
        </div>
    </div>
  </div>
</template>

<style scoped>
.table :where(thead, tfoot) :where(th, td) { background-color: transparent !important; color: inherit; font-size: 11px; font-weight: bold; border: none; }
.checkbox-error { --chkbg: var(--er); --chkfg: white; }
pre::-webkit-scrollbar { width: 4px; }
pre::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.1); border-radius: 2px; }
</style>
