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
  Hourglass
} from 'lucide-vue-next'

const props = defineProps<{
  workerStats: any
  webhookStats: any
  autoSync: boolean
}>()

const emit = defineEmits(['toggle-sync'])

const totalQueueDepth = computed(() => {
    if (!props.workerStats || !props.workerStats.worker_stats) return 0
    return props.workerStats.worker_stats.reduce((sum: number, w: any) => sum + w.queue_depth, 0)
})

const activeChatsCount = computed(() => {
    if (!props.workerStats || !props.workerStats.active_chats) return 0
    return Object.keys(props.workerStats.active_chats).length
})

function formatChatKey(key: string) {
    const parts = key.split('|')
    if (parts.length === 2 && parts[0] && parts[1]) {
        const inst = parts[0].substring(0, 8)
        const chat = parts[1].length > 20 ? parts[1].substring(0, 20) + '...' : parts[1]
        return `${inst} | ${chat}`
    }
    return key
}
</script>

<template>
  <div class="space-y-12">
    <!-- Worker Pool -->
    <div class="space-y-6">
        <div class="flex items-center justify-between border-l-4 border-primary pl-4">
            <div class="flex items-center gap-3">
                <Cpu class="w-4 h-4 text-primary" />
                <h3 class="text-sm font-bold text-white uppercase tracking-widest">Primary Worker Cluster</h3>
                <div v-if="workerStats" class="badge badge-neutral bg-white/5 border-white/10 text-[10px] font-black tracking-widest px-3 py-1 ml-2">
                    {{ workerStats.active_workers }} / {{ workerStats.num_workers }} NODES ACTIVE
                </div>
            </div>
            <button class="btn-premium btn-premium-ghost px-4" @click="emit('toggle-sync')">
                <RotateCw class="w-3 h-3 mr-2" :class="autoSync ? 'animate-spin' : ''" />
                <span class="text-xs font-bold uppercase tracking-widest">{{ autoSync ? 'Flow: ON' : 'Flow: OFF' }}</span>
            </button>
        </div>

        <!-- System-wide Stats for Primary Pool -->
        <div v-if="workerStats" class="space-y-6">
            <!-- Row 1: Core Metrics -->
            <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ workerStats.active_workers }} / {{ workerStats.num_workers }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Active workers</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <Server class="w-3.5 h-3.5 text-primary" />
                    </div>
                </div>
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ workerStats.total_processed }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Processed</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <CheckCircle class="w-3.5 h-3.5 text-success" />
                    </div>
                </div>
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ totalQueueDepth }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Total queued</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <Hourglass class="w-3.5 h-3.5 text-amber-500" />
                    </div>
                </div>
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ activeChatsCount }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Active chats</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <MessageSquare class="w-3.5 h-3.5 text-purple-500" />
                    </div>
                </div>
            </div>

            <!-- Row 2: Flow Metrics -->
            <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
                <div class="bg-[#161a23] border border-white/5 rounded-xl p-5 shadow-sm">
                    <h3 class="text-2xl font-bold text-white mb-1">{{ workerStats.total_dispatched }}</h3>
                    <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Dispatched</p>
                </div>
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ workerStats.total_errors }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Errors</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <AlertCircle class="w-3.5 h-3.5 text-error" />
                    </div>
                </div>
                <div class="bg-[#161a23] border border-white/5 rounded-xl overflow-hidden shadow-sm flex flex-col">
                    <div class="p-5 flex-1">
                        <h3 class="text-2xl font-bold text-white mb-1">{{ workerStats.total_dropped }}</h3>
                        <p class="text-xs font-bold uppercase tracking-widest text-slate-500">Dropped</p>
                    </div>
                    <div class="px-5 py-2 bg-white/[0.02] border-t border-white/5">
                        <Trash2 class="w-3.5 h-3.5 text-error" />
                    </div>
                </div>
            </div>
        </div>

        <div v-if="workerStats && workerStats.worker_stats" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div v-for="w in workerStats.worker_stats" :key="w.worker_id" class="bg-[#161a23] border border-white/5 rounded-xl p-5 hover:border-primary/20 transition-all group relative overflow-hidden">
                <div class="flex justify-between items-start mb-4 relative z-10">
                    <div class="flex items-center gap-2">
                        <Server class="w-3 h-3 text-slate-600 group-hover:text-primary transition-colors" />
                        <span class="text-xs font-bold text-slate-500 uppercase tracking-widest">Node-{{ w.worker_id }}</span>
                    </div>
                    <div v-if="w.is_processing" class="flex items-center gap-2">
                        <span class="text-[8px] font-black text-success uppercase animate-pulse">Processing</span>
                        <div class="w-2 h-2 rounded-full bg-success animate-pulse shadow-[0_0_8px_rgba(34,197,94,0.4)]"></div>
                    </div>
                </div>
                <div class="flex justify-between items-baseline mb-3 relative z-10">
                    <div class="text-2xl font-bold text-white">{{ w.jobs_processed }}</div>
                    <div class="text-xs font-black opacity-20 uppercase tracking-widest">Jobs</div>
                </div>
                <div class="w-full bg-white/5 h-1.5 rounded-full overflow-hidden relative z-10">
                    <div class="bg-primary h-full transition-all duration-500" :style="{ width: (w.queue_depth / workerStats.queue_size * 100) + '%' }"></div>
                </div>
                <div class="mt-2 text-[10px] font-bold text-slate-700 uppercase tracking-tighter relative z-10">Queue: {{ w.queue_depth }}/{{ workerStats.queue_size }}</div>
                
                <!-- Background decor -->
                <div v-if="w.is_processing" class="absolute -right-4 -bottom-4 opacity-[0.03] text-success group-hover:opacity-[0.07] transition-opacity">
                    <RotateCw class="w-16 h-16 animate-spin-slow" />
                </div>
            </div>
        </div>

        <!-- Active Chats List for Worker Pool -->
        <div v-if="workerStats && workerStats.active_chats && Object.keys(workerStats.active_chats).length > 0" class="bg-black/20 border border-white/5 rounded-xl p-6">
            <div class="flex items-center gap-2 mb-4">
                <MessageSquare class="w-4 h-4 text-purple-400" />
                <h4 class="text-xs font-black text-purple-400 uppercase tracking-widest">Active Chat Sessions ({{ Object.keys(workerStats.active_chats).length }})</h4>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                <div v-for="(workerId, chatKey) in workerStats.active_chats" :key="chatKey" class="flex items-center justify-between p-3 bg-white/[0.02] border border-white/5 rounded-lg">
                    <div class="flex items-center gap-3">
                        <div class="w-8 h-8 rounded-lg bg-white/5 flex items-center justify-center">
                            <MessageSquare class="w-3.5 h-3.5 text-slate-600" />
                        </div>
                        <code class="text-[10px] text-slate-400 font-mono">{{ formatChatKey(String(chatKey)) }}</code>
                    </div>
                    <div class="flex items-center gap-2">
                        <ChevronRight class="w-3 h-3 text-slate-700" />
                        <span class="text-[10px] font-black text-primary/80 uppercase">Node-{{ workerId }}</span>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <!-- Webhook Pipeline -->
    <div class="space-y-6 pt-12 border-t border-white/5">
        <div class="flex items-center justify-between border-l-4 border-secondary pl-4">
            <div class="flex items-center gap-3">
                <Zap class="w-4 h-4 text-secondary" />
                <h3 class="text-sm font-bold text-white uppercase tracking-widest">Webhook Response Pipeline</h3>
                <div v-if="webhookStats" class="badge badge-neutral bg-white/5 border-white/10 text-[10px] font-black tracking-widest px-3 py-1 ml-2 text-secondary">
                    LINEAR WEBHOOK FLOW ACTIVE
                </div>
            </div>
        </div>

        <!-- Webhook Global Metrics -->
        <div v-if="webhookStats" class="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div class="bg-white/[0.02] border border-white/5 rounded-xl p-6">
                <div class="flex items-center gap-2 mb-2">
                    <Database class="w-3.5 h-3.5 text-slate-500" />
                    <p class="text-[9px] font-black text-slate-600 uppercase tracking-widest">Total Dispatched</p>
                </div>
                <p class="text-2xl font-bold text-white">{{ webhookStats.total_dispatched }}</p>
            </div>
            <div class="bg-white/[0.02] border border-white/5 rounded-xl p-6">
                <div class="flex items-center gap-2 mb-2">
                    <CheckCircle class="w-3.5 h-3.5 text-success/60" />
                    <p class="text-[9px] font-black text-slate-600 uppercase tracking-widest">Handled OK</p>
                </div>
                <p class="text-2xl font-bold text-white">{{ webhookStats.total_processed }}</p>
            </div>
            <div class="bg-white/[0.02] border border-white/5 rounded-xl p-6">
                <div class="flex items-center gap-2 mb-2">
                    <AlertCircle class="w-3.5 h-3.5 text-error/60" />
                    <p class="text-[9px] font-black text-slate-600 uppercase tracking-widest">Handler Errors</p>
                </div>
                <p class="text-2xl font-bold text-error/80">{{ webhookStats.total_errors }}</p>
            </div>
            <div class="bg-white/[0.02] border border-white/5 rounded-xl p-6">
                <div class="flex items-center gap-2 mb-2">
                    <Trash2 class="w-3.5 h-3.5 text-slate-600" />
                    <p class="text-[9px] font-black text-slate-600 uppercase tracking-widest">Buffer Dropped</p>
                </div>
                <p class="text-2xl font-bold text-white">{{ webhookStats.total_dropped }}</p>
            </div>
        </div>

        <div v-if="webhookStats" class="bg-[#161a23] border border-white/10 rounded-xl p-8 space-y-8 relative overflow-hidden shadow-2xl">
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 relative z-10">
                <div v-for="w in webhookStats.worker_stats.slice(0, 8)" :key="'ws-' + w.worker_id" class="flex flex-col gap-3 p-4 bg-white/[0.02] border border-white/5 rounded-xl group hover:border-secondary/20 transition-all">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-2">
                            <Server class="w-3 h-3 text-slate-700 group-hover:text-secondary mb-0.5" />
                            <span class="text-xs font-bold text-slate-600 uppercase tracking-tighter">HND-{{ w.worker_id }}</span>
                        </div>
                        <div v-if="w.is_processing" class="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse"></div>
                    </div>
                    <div class="space-y-2">
                        <div class="flex justify-between items-center text-[9px] font-black uppercase tracking-widest mb-1">
                            <span class="text-slate-700">Load</span>
                            <span :class="w.queue_depth > 0 ? 'text-secondary' : 'text-slate-800'">{{ (w.queue_depth / webhookStats.queue_size * 100).toFixed(0) }}%</span>
                        </div>
                        <progress class="progress progress-secondary h-1.5 bg-white/5" :value="w.queue_depth" :max="webhookStats.queue_size"></progress>
                        <div class="flex justify-between items-center px-1">
                            <span class="text-[9px] font-bold text-slate-800 uppercase tracking-widest">Processed: {{ w.jobs_processed }}</span>
                            <span class="text-[9px] font-bold text-slate-800 uppercase tracking-widest">D: {{ w.queue_depth }}</span>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Active Webhook Chats (If any) -->
            <div v-if="webhookStats.active_chats && Object.keys(webhookStats.active_chats).length > 0" class="bg-black/40 border border-white/5 rounded-xl p-6 relative z-10">
                <div class="flex items-center gap-2 mb-4">
                    <Zap class="w-4 h-4 text-secondary/60" />
                    <h4 class="text-xs font-black text-secondary/60 uppercase tracking-widest">Active Dispatching Threads ({{ Object.keys(webhookStats.active_chats).length }})</h4>
                </div>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                    <div v-for="(workerId, chatKey) in webhookStats.active_chats" :key="'wh-' + chatKey" class="flex items-center justify-between p-3 bg-white/[0.02] border border-white/5 rounded-lg">
                        <code class="text-[9px] text-slate-500 font-mono">{{ formatChatKey(String(chatKey)) }}</code>
                        <span class="text-[10px] font-black text-secondary uppercase tracking-widest">HND-{{ workerId }}</span>
                    </div>
                </div>
            </div>

            <div class="absolute inset-0 bg-gradient-to-br from-secondary/[0.03] to-transparent pointer-events-none"></div>
        </div>
        <div v-else class="p-10 bg-white/[0.02] border border-dashed border-white/5 rounded-xl text-center">
            <p class="text-xs font-bold text-slate-600 uppercase tracking-widest">Initialising webhook dispatcher...</p>
        </div>
    </div>
  </div>
</template>

<style scoped>
.animate-spin-slow { animation: spin 8s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>
