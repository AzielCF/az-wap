<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { 
  Activity, 
  RefreshCw
} from 'lucide-vue-next'

import BotEventMonitor from '@/components/monitoring/BotEventMonitor.vue'
import InfrastructureMonitor from '@/components/monitoring/InfrastructureMonitor.vue'
import SessionMonitor from '@/components/monitoring/SessionMonitor.vue'

const api = useApi()
const botEvents = ref<any>({ results: [], total: 0 })
const globalStats = ref<any>({ total_processed: 0, total_errors: 0, total_dropped: 0 })
const clusterActivity = ref<any[]>([])
const activeServers = ref<any[]>([])

const botAutoRefresh = ref(true)
const infraAutoRefresh = ref(true)
let botInterval: any = null
let infraInterval: any = null

async function loadBotData() {
    try {
        const res = await api.get('/api/monitoring/events')
        botEvents.value = res || {}
    } catch (err) {
        console.error('Failed to load bot events:', err)
        botEvents.value = {}
    }
}

async function loadInfraData() {
    try {
        const [statsRes, activityRes, serversRes] = await Promise.allSettled([
            api.get('/api/monitoring/stats'),
            api.get('/api/monitoring/cluster-activity'),
            api.get('/api/monitoring/servers')
        ])
        
        if (statsRes.status === 'fulfilled') globalStats.value = statsRes.value || {}
        if (activityRes.status === 'fulfilled') clusterActivity.value = activityRes.value || []
        if (serversRes.status === 'fulfilled') activeServers.value = serversRes.value || []
    } catch (err) {
        console.error('Failed to load infra stats:', err)
    }
}

function toggleBotSync() {
  botAutoRefresh.value = !botAutoRefresh.value
  if (botAutoRefresh.value) startBotSync()
  else stopBotSync()
}

function toggleInfraSync() {
  infraAutoRefresh.value = !infraAutoRefresh.value
  if (infraAutoRefresh.value) startInfraSync()
  else stopInfraSync()
}

function startBotSync() {
  stopBotSync()
  loadBotData()
  botInterval = setInterval(loadBotData, 3000)
}

function stopBotSync() {
  if (botInterval) { clearInterval(botInterval); botInterval = null }
}

function startInfraSync() {
  stopInfraSync()
  loadInfraData()
  infraInterval = setInterval(loadInfraData, 4000)
}

function stopInfraSync() {
  if (infraInterval) { clearInterval(infraInterval); infraInterval = null }
}

onMounted(() => {
  startBotSync()
  startInfraSync()
})
onUnmounted(() => {
  stopBotSync()
  stopInfraSync()
})
</script>

<template>
  <div class="space-y-8 max-w-[1600px] mx-auto pb-20 animate-in fade-in duration-500">
    <!-- Header -->
    <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-6 border-b border-white/5 pb-8 px-6 lg:px-0">
      <div class="space-y-3">
        <div class="flex items-center gap-3">
          <Activity class="w-5 h-5 text-primary" />
          <h2 class="text-4xl font-bold tracking-tight text-white uppercase">System Monitoring</h2>
        </div>
        <p class="text-sm text-slate-500 font-medium">Real-time Bot activity and Infrastructure health metrics.</p>
      </div>
      <div class="flex items-center gap-3">
        <div class="flex items-center bg-white/[0.03] border border-white/5 rounded-xl p-1 gap-1">
            <button class="btn-premium btn-premium-ghost px-4 h-9 text-[10px]" @click="loadBotData">
                <RefreshCw class="w-3.5 h-3.5 mr-2" />
                Bot Sync
            </button>
            <button class="btn-premium btn-premium-ghost px-4 h-9 text-[10px]" @click="loadInfraData">
                <RefreshCw class="w-3.5 h-3.5 mr-2" />
                Infra Sync
            </button>
        </div>
      </div>
    </div>

    <!-- Active Sessions Table (Upper Priority) -->
    <SessionMonitor />

    <!-- Bot Monitor Log Section -->
    <BotEventMonitor :stats="botEvents" :auto-sync="botAutoRefresh" @toggle-sync="toggleBotSync" />

    <!-- Cluster Pools Monitor -->
    <div class="pt-12 border-t border-white/5">
        <InfrastructureMonitor 
          :clusterActivity="clusterActivity" 
          :activeServers="activeServers"
          :globalStats="globalStats"
          :auto-sync="infraAutoRefresh" 
          @toggle-sync="toggleInfraSync" 
        />
    </div>
  </div>
</template>
