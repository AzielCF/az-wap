<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import { 
  Database, 
  RefreshCw, 
  Trash2, 
  Clock, 
  Cpu,
  Globe,
  Bot,
  MessageSquare,
  Lightbulb
} from 'lucide-vue-next'

interface CacheEntry {
  name: string
  expires_at: string
  model?: string
  type?: string
  scope?: string
  description?: string
  fingerprint?: string
  provider?: string
}

const api = useApi()
const caches = ref<CacheEntry[]>([])
const loading = ref(false)
const autoRefresh = ref(false)
let interval: any = null

async function loadCaches() {
  loading.value = true
  try {
    const res = await api.get('/api/monitoring/ai-caches')
    caches.value = res || []
  } catch (err) {
    console.error('Failed to load AI caches:', err)
    caches.value = []
  } finally {
    loading.value = false
  }
}

function startSync() {
  loadCaches()
  if (autoRefresh.value) {
    interval = setInterval(loadCaches, 10000) // Every 10 seconds
  }
}

function stopSync() {
  if (interval) { clearInterval(interval); interval = null }
}

function toggleSync() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    startSync()
  } else {
    stopSync()
  }
}

function getTimeRemaining(expiresAt: string): string {
  const expiry = new Date(expiresAt)
  const now = new Date()
  const diffMs = expiry.getTime() - now.getTime()
  
  if (diffMs <= 0) return 'Expired'
  
  const mins = Math.floor(diffMs / 60000)
  if (mins < 60) return `${mins}m remaining`
  
  const hours = Math.floor(mins / 60)
  return `${hours}h ${mins % 60}m remaining`
}

function getTypeIcon(type?: string) {
  switch (type) {
    case 'global': return Globe
    case 'bot': return Bot
    case 'chat': return MessageSquare
    default: return Database
  }
}

function getTypeBadgeClass(type?: string) {
  switch (type) {
    case 'global': return 'bg-purple-500/10 border-purple-500/20 text-purple-400'
    case 'bot': return 'bg-blue-500/10 border-blue-500/20 text-blue-400'
    case 'chat': return 'bg-emerald-500/10 border-emerald-500/20 text-emerald-400'
    default: return 'bg-slate-500/10 border-slate-500/20 text-slate-400'
  }
}

const hasCaches = computed(() => caches.value.length > 0)

onMounted(() => {
  loadCaches()
})

onUnmounted(() => {
  stopSync()
})
</script>

<template>
  <div class="space-y-4">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="p-2 rounded-lg bg-amber-500/10 border border-amber-500/20">
          <Lightbulb class="w-4 h-4 text-amber-400" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-white uppercase tracking-wide">AI Cache Inspector</h3>
          <p class="text-[10px] text-slate-500">Active provider-side caches (Gemini, OpenAI)</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button 
          class="btn-premium btn-premium-ghost px-3 h-8 text-[10px]" 
          @click="loadCaches"
          :disabled="loading"
        >
          <RefreshCw class="w-3 h-3 mr-1.5" :class="{ 'animate-spin': loading }" />
          Refresh
        </button>
        <button 
          class="btn-premium px-3 h-8 text-[10px]" 
          :class="autoRefresh ? 'btn-premium-primary' : 'btn-premium-ghost'"
          @click="toggleSync"
        >
          {{ autoRefresh ? 'Auto ON' : 'Auto OFF' }}
        </button>
      </div>
    </div>

    <!-- Cache Table -->
    <div v-if="hasCaches" class="bg-black/20 border border-white/5 rounded-xl overflow-hidden">
      <table class="w-full text-left">
        <thead class="bg-white/[0.02] border-b border-white/5">
          <tr>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Type</th>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Description</th>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Model</th>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Provider</th>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">TTL</th>
            <th class="py-3 px-4 text-[10px] font-bold text-slate-500 uppercase tracking-wider">Fingerprint</th>
          </tr>
        </thead>
        <tbody>
          <tr 
            v-for="cache in caches" 
            :key="cache.fingerprint || cache.name" 
            class="border-b border-white/5 hover:bg-white/[0.02] transition-colors"
          >
            <td class="py-3 px-4">
              <div class="inline-flex items-center px-2 py-1 rounded border" :class="getTypeBadgeClass(cache.type)">
                <component :is="getTypeIcon(cache.type)" class="w-3 h-3 mr-1.5" />
                <span class="text-[10px] font-bold uppercase">{{ cache.type || 'unknown' }}</span>
              </div>
            </td>
            <td class="py-3 px-4">
              <span class="text-xs font-medium text-white">{{ cache.description || cache.name }}</span>
            </td>
            <td class="py-3 px-4">
              <span class="text-[10px] font-mono text-primary/80">{{ cache.model || '-' }}</span>
            </td>
            <td class="py-3 px-4">
              <span class="text-[10px] font-bold text-slate-400 uppercase">{{ cache.provider || '-' }}</span>
            </td>
            <td class="py-3 px-4">
              <div class="flex items-center gap-1.5 text-[10px] text-slate-400">
                <Clock class="w-3 h-3 text-amber-500" />
                {{ getTimeRemaining(cache.expires_at) }}
              </div>
            </td>
            <td class="py-3 px-4">
              <span class="text-[9px] font-mono text-slate-600 truncate block max-w-[150px]" :title="cache.fingerprint">
                {{ cache.fingerprint || '-' }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Empty State -->
    <div v-else class="bg-black/20 border border-white/5 rounded-xl p-8 text-center">
      <Database class="w-8 h-8 text-slate-600 mx-auto mb-3" />
      <p class="text-sm font-medium text-slate-500">No active AI caches</p>
      <p class="text-[10px] text-slate-600 mt-1">Caches will appear here when system prompts are cached by providers.</p>
    </div>
  </div>
</template>
