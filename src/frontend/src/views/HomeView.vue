<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'

const router = useRouter()
const api = useApi()

interface StatItem {
  label: string
  value: string
  delta: string
  trend: 'up' | 'down' | 'neutral'
}

const stats = ref<StatItem[]>([
  { label: 'Active Sessions', value: '0', delta: '+0', trend: 'neutral' },
  { label: 'Total Workspaces', value: '0', delta: '+0', trend: 'neutral' },
  { label: 'QR Channels', value: '0', delta: '0', trend: 'neutral' },
  { label: 'Platform Health', value: '99.9%', delta: 'Online', trend: 'up' },
])

const recentActivity = ref<any[]>([
    { timestamp: 'Just now', entity: 'BotMonitor', operation: 'Global event stream active', status: 'OK' },
    { timestamp: '2m ago', entity: 'HealthCheck', operation: 'Infrastructure verification complete', status: 'VERIFIED' },
    { timestamp: '10m ago', entity: 'AuthSystem', operation: 'Master session persistence active', status: 'SECURE' }
])

async function loadStats() {
  try {
    // 1. Load Workspaces and Channels
    const ws = await api.get('/workspaces')
    if (ws && Array.isArray(ws)) {
      if (stats.value[1]) stats.value[1].value = ws.length.toString()
      
      let totalCh = 0
      for (const w of ws) {
         try {
            const chs = await api.get(`/workspaces/${w.id}/channels`)
            if (chs && Array.isArray(chs)) {
                totalCh += chs.length
            }
         } catch(e) {}
      }
      if (stats.value[2]) stats.value[2].value = totalCh.toString()
    }

    // 2. Load Real-time Monitoring Data (Health & Sessions)
    const [healthRes, sessionsRes] = await Promise.allSettled([
        api.get('/api/health/status'),
        api.get('/workspaces/active-sessions')
    ])

    // Health Logic (Real-time memory based)
    if (healthRes.status === 'fulfilled' && healthRes.value) {
        const data = healthRes.value as any
        if (data.results && Array.isArray(data.results)) {
            const systems = data.results
            const total = systems.length
            const healthy = systems.filter((s: any) => s.status === 'OK' || s.status === 'HEALTHY').length
            
            const health = total > 0 ? (healthy / total) * 100 : 100
            
            if (stats.value[3]) {
                stats.value[3].value = Math.round(health) + '%'
                
                if (health >= 99.5) {
                    stats.value[3].delta = 'Optimal'
                    stats.value[3].trend = 'up'
                } else if (health >= 90) {
                    stats.value[3].delta = 'Good'
                    stats.value[3].trend = 'neutral'
                } else {
                    stats.value[3].delta = 'Degraded'
                    stats.value[3].trend = 'down'
                }
            }
        }
    }

    // Active Sessions Logic
    if (sessionsRes.status === 'fulfilled' && Array.isArray(sessionsRes.value)) {
        if (stats.value[0]) {
            const activeCount = sessionsRes.value.length
            stats.value[0].value = activeCount.toString()
            stats.value[0].delta = activeCount > 0 ? 'Live' : 'Idle'
            stats.value[0].trend = activeCount > 0 ? 'up' : 'neutral'
        }
    }

  } catch (err) {
    console.error('Home stats load error:', err)
  }
}

onMounted(loadStats)
</script>

<template>
  <div class="space-y-16 max-w-[1400px] mx-auto pb-20 animate-in fade-in duration-700">
    <!-- Professional Hero Section -->
    <div class="relative group">
        <div class="absolute -inset-2 bg-primary/20 rounded-[3rem] blur-3xl opacity-10 group-hover:opacity-20 transition duration-1000"></div>
        <div class="relative bg-[#161a23]/40 backdrop-blur-xl rounded-[2.5rem] border border-white/5 p-10 lg:p-16 overflow-hidden flex flex-col lg:flex-row items-center justify-between shadow-2xl">
            <div class="relative z-10 max-w-2xl text-center lg:text-left">
                <div class="flex items-center justify-center lg:justify-start gap-4 mb-8">
                    <span class="w-10 h-[2px] bg-primary/40 hidden lg:block"></span>
                    <span class="text-[10px] font-black uppercase tracking-[0.4em] text-primary">Az-Wap AI WhatsApp Engine</span>
                </div>
                
                <h1 class="text-5xl lg:text-7xl font-black tracking-tighter mb-6 text-white leading-[0.9]">
                  Intelligent AI <br/> <span class="text-primary italic">WhatsApp Protocols</span>.
                </h1>
                
                <p class="text-slate-400 text-lg mb-12 max-w-lg leading-relaxed font-medium mx-auto lg:mx-0">
                  Deploy AI-powered conversational bots with human-like presence. Multi-tenant management, tool orchestration, and scalable automation.
                </p>
                
                <div class="flex flex-wrap justify-center lg:justify-start gap-6">
                  <RouterLink to="/workspaces" class="btn-premium btn-premium-primary px-12 h-16">
                    Launch Workspaces
                  </RouterLink>
                  <RouterLink to="/monitoring" class="btn-premium btn-premium-ghost px-10 h-16 border border-white/5">
                    Live Telemetry
                  </RouterLink>
                </div>
            </div>

            <!-- Aesthetic Background Overlay -->
            <div class="absolute right-0 top-0 w-1/2 h-full bg-gradient-to-l from-primary/5 to-transparent pointer-events-none"></div>
        </div>
    </div>

    <!-- Stats Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-8">
      <div v-for="stat in stats" :key="stat.label" class="bg-[#161a23]/40 border border-white/5 rounded-[2rem] p-8 hover:border-primary/20 transition-all duration-500 group shadow-2xl backdrop-blur-xl relative overflow-hidden">
        <p class="text-[10px] font-black uppercase tracking-[0.2em] text-slate-500 mb-6 group-hover:text-primary transition-colors">{{ stat.label }}</p>
        <div class="flex items-baseline justify-between relative z-10">
            <h3 class="text-5xl font-black text-white tracking-tighter">{{ stat.value }}</h3>
            <span class="badge-premium" :class="stat.trend === 'up' ? 'badge-success' : 'badge-ghost opacity-60'">
                {{ stat.delta }}
            </span>
        </div>
        <div class="absolute -bottom-4 -right-4 w-20 h-20 bg-primary/5 rounded-full blur-2xl group-hover:bg-primary/10 transition-all"></div>
      </div>
    </div>

    <!-- Operations & Monitor -->
    <div class="grid grid-cols-1 xl:grid-cols-3 gap-10">
        <div class="xl:col-span-2 space-y-8">
            <div class="section-title-premium text-primary/60 border-none pl-0">Operational Pulse</div>
            <div class="bg-[#161a23]/40 border border-white/5 rounded-[2rem] overflow-hidden shadow-2xl backdrop-blur-xl">
                <div class="overflow-x-auto">
                    <table class="table-premium w-full border-collapse">
                        <thead>
                            <tr class="text-[10px] text-slate-500 uppercase tracking-[0.2em] border-b border-white/5 bg-white/[0.02]">
                                <th class="py-6 pl-10">Event Time</th>
                                <th>Subsystem</th>
                                <th>Operational Data</th>
                                <th class="pr-10 text-right">Status</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="act in recentActivity" :key="act.operation" class="hover:bg-white/[0.03] transition-colors border-b border-white/[0.02] group">
                                <td class="py-8 pl-10">
                                    <span class="text-xs font-mono text-slate-500 group-hover:text-slate-300 transition-colors">{{ act.timestamp }}</span>
                                </td>
                                <td>
                                    <span class="text-xs font-black text-slate-200 uppercase tracking-widest">{{ act.entity }}</span>
                                </td>
                                <td>
                                    <span class="text-xs font-medium text-slate-400 group-hover:text-slate-300 transition-colors">{{ act.operation }}</span>
                                </td>
                                <td class="pr-10 text-right">
                                    <div class="badge-premium whitespace-nowrap" 
                                         :class="act.status === 'OK' || act.status === 'VERIFIED' ? 'badge-success' : 'badge-ghost opacity-60'">
                                        {{ act.status }}
                                    </div>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <!-- Sidebar Info -->
        <div class="space-y-8">
            <div class="section-title-premium text-primary/60 border-none pl-0 font-black">Cluster Identity</div>
            <div class="bg-[#161a23]/40 border border-white/5 rounded-[2.5rem] p-10 space-y-10 shadow-2xl backdrop-blur-xl group">
                <div class="flex items-center gap-6">
                    <div class="icon-box-premium icon-box-primary shrink-0 group-hover:scale-110 transition-transform duration-500">
                        <span class="text-xl font-black">AD</span>
                    </div>
                    <div>
                        <h4 class="text-base font-black text-white uppercase tracking-tight">Cloud Master</h4>
                        <p class="text-[10px] font-black text-slate-500 uppercase tracking-widest mt-1 opacity-60">Global Persistence Session</p>
                    </div>
                </div>
                
                <div class="space-y-4 pt-6">
                    <div class="p-6 rounded-[1.5rem] bg-white/[0.02] border border-white/5 flex items-center justify-between group/item cursor-pointer hover:bg-white/[0.05] transition-all shadow-lg" @click="router.push('/settings')">
                        <span class="text-[10px] font-black uppercase tracking-widest text-slate-400 group-hover/item:text-primary transition-colors">Core Backbone</span>
                        <div class="flex items-center gap-2">
                            <div class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></div>
                            <span class="text-[9px] font-black text-success uppercase">Active</span>
                        </div>
                    </div>
                    <div class="p-6 rounded-[1.5rem] bg-white/[0.02] border border-white/5 flex items-center justify-between group/item cursor-pointer hover:bg-white/[0.05] transition-all shadow-lg" @click="router.push('/monitoring')">
                        <span class="text-[10px] font-black uppercase tracking-widest text-slate-400 group-hover/item:text-primary transition-colors">Neural Monitor</span>
                        <div class="flex items-center gap-2">
                             <span class="text-[9px] font-black text-primary uppercase">Syncing</span>
                        </div>
                    </div>
                </div>

                <button class="btn-premium btn-premium-primary w-full h-16 group/btn" @click="router.push('/workspaces')">
                    Return to Console
                </button>
            </div>
        </div>
    </div>
  </div>
</template>
