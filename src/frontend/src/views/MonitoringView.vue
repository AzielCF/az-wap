<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { 
  Activity, 
  RefreshCw
} from 'lucide-vue-next'

import AppPageHeader from '@/components/AppPageHeader.vue'
import BotEventMonitor from '@/components/monitoring/BotEventMonitor.vue'
import InfrastructureMonitor from '@/components/monitoring/InfrastructureMonitor.vue'
import SessionMonitor from '@/components/monitoring/SessionMonitor.vue'
import AICacheInspector from '@/components/monitoring/AICacheInspector.vue'

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
    <AppPageHeader title="System Monitoring">
      <template #breadcrumb>
          <Activity class="w-5 h-5 text-primary" />
      </template>
      
      <template #subtitle>
        Real-time Bot activity and Infrastructure health metrics.
      </template>

      <template #actions>
        <div class="flex items-center bg-white/[0.03] border border-white/5 rounded-xl p-1 gap-1">
            <button class="btn-premium btn-premium-ghost px-4 h-9 text-xs" @click="loadBotData">
                <RefreshCw class="w-3.5 h-3.5 mr-2" />
                Bot Sync
            </button>
            <button class="btn-premium btn-premium-ghost px-4 h-9 text-xs" @click="loadInfraData">
                <RefreshCw class="w-3.5 h-3.5 mr-2" />
                Infra Sync
            </button>
        </div>
      </template>
    </AppPageHeader>

    <!-- Active Sessions Table (Upper Priority) -->
    <SessionMonitor />

    <!-- Bot Monitor Log Section -->
    <BotEventMonitor :stats="botEvents" :auto-sync="botAutoRefresh" @toggle-sync="toggleBotSync" />

    <!-- AI Cache Inspector -->
    <div class="pt-8 border-t border-white/5">
      <AICacheInspector />
    </div>

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
