<script setup lang="ts">
import { computed } from 'vue'
import { 
  Cpu, 
  Server, 
  Zap, 
  Database, 
  Clock,
  RotateCw,
  MessageSquare,
  AlertCircle,
  Trash2,
  ChevronRight,
  CheckCircle,
  Hourglass,
  Layers
} from 'lucide-vue-next'

const props = defineProps<{
  clusterActivity: any[]
  activeServers: any[]
  globalStats: any
  autoSync: boolean
}>()

const emit = defineEmits(['toggle-sync'])

const activeWorkersCount = computed(() => {
    return props.clusterActivity.filter(w => w.is_processing).length
})

const totalWorkersCount = computed(() => {
    return props.clusterActivity.length
})

// Group workers by server and then by pool type
const serversWithActivity = computed(() => {
    return props.activeServers.map(server => {
        const workers = props.clusterActivity.filter(w => w.server_id === server.id)
        return {
            ...server,
            primaryWorkers: workers.filter(w => w.pool_type === 'primary'),
            webhookWorkers: workers.filter(w => w.pool_type === 'webhook')
        }
    })
})

function formatUptime(seconds: number) {
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

function formatChatKey(key: string) {
    if (!key) return ''
    const parts = key.split('|')
    if (parts.length >= 2 && parts[0] && parts[1]) {
        const inst = parts[0].substring(0, 8)
        const chat = parts[1].length > 15 ? parts[1].substring(0, 15) + '...' : parts[1]
        return `${inst} | ${chat}`
    }
    return key
}
</script>

<template>
  <div class="space-y-12">
    <!-- Cluster Summary -->
    <div class="space-y-6">
        <div class="flex items-center justify-between border-l-4 border-primary pl-4">
            <div class="flex items-center gap-3">
                <Layers class="w-4 h-4 text-primary" />
                <h3 class="text-sm font-bold text-white uppercase tracking-widest">Cluster Intelligence Dashboard</h3>
                <div class="badge badge-neutral bg-white/5 border-white/10 text-xs font-black tracking-widest px-3 py-1 ml-2">
                    {{ activeServers.length }} NODES CONNECTED
                </div>
            </div>
            <button class="btn-premium btn-premium-ghost px-4" @click="emit('toggle-sync')">
                <RotateCw class="w-3 h-3 mr-2" :class="autoSync ? 'animate-spin' : ''" />
                <span class="text-xs font-bold uppercase tracking-widest">{{ autoSync ? 'Flow: ON' : 'Flow: OFF' }}</span>
            </button>
        </div>

        <!-- Global Metrics -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div class="bg-[#161a23] border border-white/5 rounded-2xl overflow-hidden shadow-sm flex flex-col p-6 group transition-all hover:bg-white/[0.02]" :class="globalStats.valkey_enabled ? 'border-primary/20' : 'border-warning/30'">
                <div class="flex justify-between items-start mb-4">
                    <div class="p-2 rounded-xl" :class="globalStats.valkey_enabled ? 'bg-primary/10' : 'bg-warning/10'">
                        <Database class="w-5 h-5" :class="globalStats.valkey_enabled ? 'text-primary' : 'text-warning'" />
                    </div>
                    <span class="text-xs font-black uppercase" :class="globalStats.valkey_enabled ? 'text-primary' : 'text-warning'">Infrastructure</span>
                </div>
                <h3 class="text-3xl font-bold text-white mb-1">{{ globalStats.valkey_enabled ? 'Valkey' : 'Hybrid' }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">
                    {{ globalStats.valkey_enabled ? 'Distributed Mode: ON' : 'Fallback Mode: RAM' }}
                </p>
            </div>

            <div class="bg-[#161a23] border border-white/5 rounded-2xl overflow-hidden shadow-sm flex flex-col p-6 group transition-all hover:bg-white/[0.02]">
                <div class="flex justify-between items-start mb-4">
                    <div class="bg-success/10 p-2 rounded-xl">
                        <CheckCircle class="w-5 h-5 text-success" />
                    </div>
                    <span class="text-xs font-black text-slate-700 uppercase">Handled</span>
                </div>
                <h3 class="text-3xl font-bold text-white mb-1">{{ globalStats.total_processed || 0 }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Global Messages</p>
            </div>

            <div class="bg-[#161a23] border border-white/5 rounded-2xl overflow-hidden shadow-sm flex flex-col p-6 group transition-all hover:bg-white/[0.02]">
                <div class="flex justify-between items-start mb-4">
                    <div class="bg-warning/10 p-2 rounded-xl">
                        <Hourglass class="w-5 h-5 text-warning" />
                    </div>
                    <span class="text-xs font-black text-slate-700 uppercase">Today's Pool</span>
                </div>
                <h3 class="text-3xl font-bold text-white mb-1">{{ globalStats.pending_tasks_memory || 0 }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">In-Memory Tasks</p>
            </div>

            <div class="bg-[#161a23] border border-white/5 rounded-2xl overflow-hidden shadow-sm flex flex-col p-6 group transition-all hover:bg-white/[0.02]">
                <div class="flex justify-between items-start mb-4">
                    <div class="bg-slate-500/10 p-2 rounded-xl">
                        <RotateCw class="w-5 h-5 text-slate-500" />
                    </div>
                    <span class="text-xs font-black text-slate-700 uppercase">Scheduled</span>
                </div>
                <h3 class="text-3xl font-bold text-white mb-1">{{ globalStats.pending_tasks_db || 0 }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Long-term (SQLite)</p>
            </div>

            <div class="bg-[#161a23] border border-white/5 rounded-2xl overflow-hidden shadow-sm flex flex-col p-6 group transition-all hover:bg-white/[0.02]">
                <div class="flex justify-between items-start mb-4">
                    <div class="bg-purple-500/10 p-2 rounded-xl">
                        <Cpu class="w-5 h-5 text-purple-500" />
                    </div>
                    <span class="text-xs font-black text-slate-700 uppercase">Cluster Load</span>
                </div>
                <h3 class="text-3xl font-bold text-white mb-1">{{ activeWorkersCount }} / {{ totalWorkersCount }}</h3>
                <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Processing Threads</p>
            </div>
        </div>
    </div>

    <!-- Servers Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div v-for="server in serversWithActivity" :key="server.id" class="bg-[#161a23] border border-white/5 rounded-3xl overflow-hidden flex flex-col shadow-2xl">
            <!-- Server Header -->
            <div class="p-6 bg-white/[0.02] border-b border-white/5 flex items-center justify-between">
                <div class="flex items-center gap-4">
                    <div class="w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center border border-primary/20">
                        <Server class="w-6 h-6 text-primary" />
                    </div>
                    <div>
                        <h4 class="text-lg font-bold text-white leading-none mb-1">{{ server.id }}</h4>
                        <div class="flex items-center gap-3">
                             <div class="flex items-center gap-1.5">
                                <span class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></span>
                                <span class="text-xs font-bold text-slate-500 uppercase tracking-widest">Server Online</span>
                            </div>
                            <div class="flex items-center gap-1.5">
                                <Clock class="w-3 h-3 text-slate-600" />
                                <span class="text-xs font-bold text-slate-500 uppercase tracking-widest">Uptime: {{ formatUptime(server.uptime_seconds) }}</span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="bg-black/20 px-3 py-1.5 rounded-lg border border-white/5">
                    <span class="text-xs font-black text-slate-600 italic">{{ server.version ? server.version : 'v2.0-beta' }}</span>
                </div>
            </div>

            <!-- Server Body: Worker Sections -->
            <div class="p-6 space-y-8">
                <!-- Primary Pool Section -->
                <div class="space-y-4">
                    <div class="flex items-center gap-2 mb-2">
                        <div class="w-1.5 h-4 bg-primary rounded-full"></div>
                        <h5 class="text-xs font-black text-white uppercase tracking-widest mt-0.5">Primary Processor Hub</h5>
                    </div>
                    
                    <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
                        <div v-for="w in server.primaryWorkers" :key="server.id + '-p-' + w.worker_id" 
                             class="p-4 bg-black/20 border border-white/5 rounded-2xl group transition-all hover:bg-white/[0.02] relative overflow-hidden"
                             :class="w.is_processing ? 'border-primary/30 ring-1 ring-primary/20' : ''"
                        >
                            <div class="flex justify-between items-start mb-3">
                                <span class="text-xs font-black text-slate-600 uppercase">Worker #{{ w.worker_id }}</span>
                                <div v-if="w.is_processing" class="flex items-center gap-2">
                                    <div class="w-1.5 h-1.5 rounded-full bg-primary animate-pulse"></div>
                                </div>
                            </div>
                            <div class="text-xs font-bold text-white transition-opacity truncate" :class="w.is_processing ? 'opacity-100' : 'opacity-20'">
                                {{ w.is_processing ? formatChatKey(w.chat_id) : 'READY' }}
                            </div>
                            <!-- Background pattern for processing -->
                            <div v-if="w.is_processing" class="absolute -right-2 -bottom-2 opacity-[0.05] text-primary rotate-12">
                                <RotateCw class="w-12 h-12 animate-spin-slow" />
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Webhook Pool Section -->
                <div class="space-y-4 pt-4 border-t border-white/5">
                    <div class="flex items-center gap-2 mb-2">
                        <div class="w-1.5 h-4 bg-secondary rounded-full"></div>
                        <h5 class="text-xs font-black text-white uppercase tracking-widest mt-0.5">Webhook Dispatcher Array</h5>
                    </div>
                    
                    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
                        <div v-for="w in server.webhookWorkers" :key="server.id + '-w-' + w.worker_id" 
                             class="p-3 bg-white/[0.02] border border-white/5 rounded-xl transition-all"
                             :class="w.is_processing ? 'bg-secondary/5 border-secondary/30' : ''"
                        >
                            <div class="flex items-center justify-between mb-1">
                                <span class="text-xs font-black text-slate-700 uppercase">W-{{ w.worker_id }}</span>
                                <Zap v-if="w.is_processing" class="w-2.5 h-2.5 text-secondary animate-pulse" />
                            </div>
                            <div class="text-xs font-bold text-white transition-opacity truncate" :class="w.is_processing ? 'opacity-100' : 'opacity-20'">
                                {{ w.is_processing ? 'ACTIVE' : 'READY' }}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
  </div>
</template>

<style scoped>
.animate-spin-slow { animation: spin 8s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>
