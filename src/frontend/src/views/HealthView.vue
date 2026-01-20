<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { RefreshCw } from 'lucide-vue-next'

const api = useApi()
const healthStatus = ref<any>(null)
const summaryValue = ref(100)
const summaryText = ref('Checking systems...')

async function loadHealth() {
  try {
    // Corrected path to include /api since InitRestHealth creates /api/health group
    const data = await api.get('/api/health/status')
    healthStatus.value = data || null
    
    if (data && data.results) {
        const systems = data.results
        const total = Object.keys(systems).length
        const healthy = Object.values(systems).filter((s: any) => s.status === 'HEALTHY').length
        summaryValue.value = total > 0 ? Math.round((healthy / total) * 100) : 100
        summaryText.value = healthy === total ? 'All systems operational' : `${total - healthy} systems with warnings`
    }
  } catch (err) {
    console.error('Failed to load health status:', err)
  }
}

onMounted(loadHealth)
</script>

<template>
  <div class="space-y-8 animate-in fade-in duration-700 max-w-[1400px] mx-auto pb-20">
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-10 border-b border-white/5 mx-6 lg:mx-0">
      <div class="space-y-4 flex-1">
        <div class="flex items-center gap-3">
          <span class="text-[10px] font-black uppercase tracking-[0.25em] text-primary/70">Structural Integrity</span>
          <span class="opacity-10 text-xl font-thin text-white">/</span>
          <span class="text-[10px] font-black uppercase tracking-[0.25em] text-slate-500">Global Cluster Health</span>
        </div>
        <h2 class="text-6xl font-black tracking-tighter text-white uppercase leading-none">Health Monitor</h2>
      </div>
      <button class="btn-premium btn-premium-primary px-16 h-14" @click="loadHealth" :disabled="api.loading.value">
        <span v-if="api.loading.value" class="loading loading-spinner loading-xs mr-3"></span>
        <RefreshCw v-else class="w-5 h-5 mr-3" />
        Re-Scan Infrastructure
      </button>
    </div>

    <div v-if="!healthStatus && api.loading.value" class="flex justify-center py-40">
      <span class="loading loading-ring loading-lg text-primary"></span>
    </div>

    <div v-else-if="healthStatus && healthStatus.results" class="grid grid-cols-1 lg:grid-cols-3 gap-8">
      <!-- Summary Card -->
      <div class="card bg-neutral-900 lg:col-span-3 border border-white/10 shadow-2xl overflow-hidden relative group">
        <div class="card-body p-10 flex-row items-center gap-10 relative z-10">
          <div class="radial-progress text-primary transition-all duration-1000 group-hover:scale-110" :style="{ '--value': summaryValue, '--size': '8rem', '--thickness': '12px' }" role="progressbar">
            <span class="text-white text-2xl font-black italic tracking-tighter">{{ summaryValue }}%</span>
          </div>
          <div class="space-y-2">
            <h3 class="text-3xl font-black italic tracking-tighter uppercase text-white">{{ summaryText }}</h3>
            <p class="text-white/40 text-sm font-medium leading-relaxed max-w-lg">Analytical verification of {{ Object.keys(healthStatus.results).length }} core subsystems. All clusters are reporting via Node-01 gateway.</p>
            <div class="flex items-center gap-4 mt-6">
                <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full bg-success"></span>
                    <span class="text-[10px] font-black uppercase tracking-widest text-success">Secure Node</span>
                </div>
                <div class="flex items-center gap-2">
                    <span class="w-2 h-2 rounded-full bg-primary animate-pulse"></span>
                    <span class="text-[10px] font-black uppercase tracking-widest text-primary">Streaming Data</span>
                </div>
            </div>
          </div>
        </div>
        <div class="absolute inset-0 bg-gradient-to-r from-primary/10 to-transparent pointer-events-none"></div>
      </div>

      <!-- Detail list -->
      <div v-for="(res, key) in healthStatus.results" :key="key" class="group bg-base-100/50 backdrop-blur-xl border border-white/5 rounded-[2.5rem] p-8 hover:bg-base-100 transition-all duration-500 relative overflow-hidden">
         <div class="relative z-10">
            <div class="flex items-center justify-between mb-8">
               <div class="flex items-center gap-3">
                  <div class="w-1.5 h-6 rounded-full" :class="res.status === 'HEALTHY' ? 'bg-success' : 'bg-error'"></div>
                  <h3 class="font-black text-xs tracking-[0.2em] uppercase text-white/50 group-hover:text-primary transition-colors">{{ String(key).split('.')[1] || key }}</h3>
               </div>
               <div class="badge badge-sm font-black text-[9px] tracking-widest" :class="res.status === 'HEALTHY' ? 'badge-success shadow-lg shadow-success/20' : 'badge-error shadow-lg shadow-error/20'">{{ res.status }}</div>
            </div>
            
            <div class="space-y-4">
               <p class="text-sm text-white/40 font-medium leading-relaxed">{{ res.message }}</p>
               
               <div v-if="res.error" class="text-[10px] text-error bg-error/5 p-4 rounded-2xl font-mono break-all border border-error/10 leading-relaxed italic">
                  {{ res.error }}
               </div>
               
               <div class="flex items-center justify-between pt-6 border-t border-white/5">
                  <span class="text-[9px] text-white/20 font-black uppercase tracking-[0.3em]">{{ String(key).split('.')[0] }}</span>
                  <div class="flex items-center gap-1.5 opacity-40">
                      <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                      <span class="text-[10px] font-mono tracking-tighter">{{ res.latency_ms || 0 }}ms</span>
                  </div>
               </div>
            </div>
         </div>
         <!-- Decorative bottom line -->
         <div class="absolute bottom-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-primary/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
      </div>
    </div>
  </div>
</template>
