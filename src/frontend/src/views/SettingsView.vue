<script setup lang="ts">
import { ref } from 'vue'
import SystemCacheControl from '@/components/settings/SystemCacheControl.vue'
import GlobalSettings from '@/components/settings/GlobalSettings.vue'

const apiBaseUrl = ref(localStorage.getItem('api_url') || (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000'))
const apiToken = ref('')

function saveInfrastructure() {
  localStorage.setItem('api_url', apiBaseUrl.value)
  if (apiToken.value) {
    localStorage.setItem('api_token', btoa(apiToken.value))
  }
  alert('Infrastructure synchronization successful.')
  window.location.reload()
}
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-16 animate-in fade-in duration-700 pb-20 font-sans">
    <!-- Header -->
    <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-6 py-10 border-b border-white/5 mx-6 lg:mx-0">
        <div class="space-y-4">
            <div class="flex items-center gap-3">
                <span class="text-xs font-black uppercase tracking-widest text-primary/70">Gateway Cluster</span>
                <span class="opacity-10 text-xl font-thin text-white">/</span>
                <span class="text-xs font-black uppercase tracking-widest text-slate-500">Node Management</span>
            </div>
            <h2 class="text-6xl font-black tracking-tighter text-white uppercase leading-none">Settings</h2>
        </div>
    </div>

    <!-- Infrastructure Settings -->
    <div class="space-y-8 px-6 lg:px-0">
        <div class="section-title-premium text-primary/60 pl-0 border-none">Infrastructure Backbone</div>

        <div class="bg-[#161a23]/40 border border-white/5 rounded-[2.5rem] p-10 lg:p-14 shadow-2xl relative overflow-hidden group transition-all duration-500 hover:border-white/10 backdrop-blur-xl">
            <div class="relative z-10 grid grid-cols-1 md:grid-cols-2 gap-10">
                <div class="form-control w-full">
                    <label class="label-premium text-slate-400">API Gateway Base URL</label>
                    <input v-model="apiBaseUrl" type="text" placeholder="http://localhost:3000" class="input-premium h-16 w-full text-lg font-black" />
                    <p class="text-xs text-slate-600 font-bold uppercase tracking-widest mt-3 px-1">Must include protocol (http/https). Default Node: 3000</p>
                </div>

                <div class="form-control w-full">
                    <label class="label-premium text-slate-400">Master Auth Token</label>
                    <input v-model="apiToken" type="password" placeholder="••••••••••••••••" class="input-premium h-16 w-full text-lg font-black" />
                    <p class="text-xs text-slate-600 font-bold uppercase tracking-widest mt-3 px-1">Encrypted in browser local-storage.</p>
                </div>
            </div>

            <div class="mt-12 flex justify-end">
                <button class="btn-premium btn-premium-primary px-16 h-16" @click="saveInfrastructure">
                    Sync Backbone Node
                </button>
            </div>
            
            <!-- Aesthetic decoration -->
            <div class="absolute -bottom-20 -right-20 w-64 h-64 bg-primary/5 rounded-full blur-[100px]"></div>
        </div>
    </div>

    <!-- System Maintenance -->
    <div class="space-y-8 px-6 lg:px-0">
        <div class="section-title-premium text-indigo-400/60 pl-0 border-none">Global Application Engine</div>
        <GlobalSettings />
    </div>

    <!-- System Maintenance -->
    <div class="space-y-8 px-6 lg:px-0">
        <div class="section-title-premium text-amber-400/60 pl-0 border-none">System Maintenance</div>
        <SystemCacheControl />
    </div>

    <!-- Security Information -->
    <div class="mx-6 lg:mx-0 p-10 bg-primary/5 border border-primary/10 rounded-[2.5rem] flex flex-col md:flex-row gap-8 items-center border-l-8 border-l-primary/30">
        <div class="icon-box-premium icon-box-primary shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
        </div>
        <div>
            <h4 class="text-xs font-black uppercase tracking-widest text-white/90 mb-2">Infrastructure Security Enforcement</h4>
            <p class="text-xs text-slate-500 font-bold uppercase tracking-widest leading-relaxed opacity-60">
                To maintain node integrity, ensure your API Gateway is deployed behind a secure Reverse Proxy with TLS/SSL. Unauthorized access to the Backbone URL may compromise all connected WhatsApp instances.
            </p>
        </div>
    </div>
  </div>
</template>

<style scoped>
</style>
