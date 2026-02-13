<!--
AZ-WAP - Open Source WhatsApp Web API
Copyright (C) 2025-2026 Aziel Cruzado <contacto@azielcruzado.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
-->

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, RouterLink, RouterView } from 'vue-router'
import { useApi } from '@/composables/useApi'

// Setup
const router = useRouter()
const isReady = ref(false)
const appVersion = ref('v0.0.0') // Default to zero state
const { get } = useApi()

onMounted(async () => {
    // 1. Fetch Version ASAP
    try {
        const info: any = await get('/app/version')
        if (info && info.version) {
            appVersion.value = info.version
        }
    } catch(e) { /* ignore */ }

    // Wait for router
    await router.isReady()
    
    // Optional: Validate session if we have a token and are not on public route
    const token = localStorage.getItem('api_token')
    if (token && !router.currentRoute.value.meta.isPublic) {
      try {
        await get('/health/status')
      } catch (e: any) {
        // If 401, clear token and redirect
        if (e.response && e.response.status === 401) {
          localStorage.removeItem('api_token')
          if (router.currentRoute.value.name !== 'login') {
            router.push('/login')
          }
        }
      }
    }

    // Small artificial delay for smoothness if loading was too fast
    await new Promise(r => setTimeout(r, 500))
    isReady.value = true
})
</script>

<template>
  <div v-if="!isReady" class="min-h-screen bg-[#0b0e14] flex flex-col items-center justify-center relative overflow-hidden">
     <!-- Background Effects -->
    <div class="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
        <div class="absolute top-[30%] left-[30%] w-[40%] h-[40%] bg-primary/5 rounded-full blur-[100px] animate-pulse"></div>
    </div>
    
    <!-- Logo Loader -->
    <div class="relative z-10 flex flex-col items-center gap-6 animate-in fade-in zoom-in duration-700">
        <div class="w-24 h-24 rounded-[2rem] bg-[#161a23] border border-white/5 flex items-center justify-center p-6 shadow-2xl relative">
            <div class="absolute inset-0 rounded-[2rem] border border-primary/20 animate-pulse"></div>
            <img src="/src/assets/azwap.svg" class="w-full h-full object-contain" alt="Loading..." />
        </div>
        <div class="flex flex-col items-center gap-2">
            <h2 class="text-xs font-black uppercase tracking-widest text-white">Az-Wap Enterprise</h2>
            <div class="flex gap-1">
                <span class="w-1 h-1 bg-primary rounded-full animate-bounce" style="animation-delay: 0ms"></span>
                <span class="w-1 h-1 bg-primary rounded-full animate-bounce" style="animation-delay: 150ms"></span>
                <span class="w-1 h-1 bg-primary rounded-full animate-bounce" style="animation-delay: 300ms"></span>
            </div>
        </div>
    </div>
  </div>
  
  <div v-else-if="$route.meta.isPublic" class="min-h-screen bg-[#0b0e14]">
    <RouterView />
  </div>

  <div v-else class="drawer lg:drawer-open font-sans">
    <input id="main-drawer" type="checkbox" class="drawer-toggle" />
    
    <div class="drawer-content flex flex-col bg-[#0b0e14] h-screen overflow-hidden">
      <!-- Navbar (Mobile only toggle + Title) -->
      <div class="navbar bg-[#161a23] lg:hidden shadow-md px-4 shrink-0 border-b border-white/5">
        <label for="main-drawer" class="btn btn-ghost drawer-button lg:hidden">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h7" />
          </svg>
        </label>
        <div class="flex-1">
          <img src="/src/assets/azwap.svg" class="h-6 ml-2 object-contain" alt="Az-Wap" />
        </div>
      </div>

      <!-- Page Content -->
      <main class="p-6 lg:p-10 flex-1 overflow-auto">
        <RouterView v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" />
          </transition>
        </RouterView>
      </main>

      <!-- Footer -->
      <footer class="footer footer-center p-6 bg-[#0b0e14] text-slate-500 text-xs font-bold uppercase tracking-widest border-t border-white/5">
        <aside>
          <p>© 2026 <a href="https://azielcruzado.com" target="_blank" rel="noopener noreferrer" class="text-primary">AzielCF</a> - AI WhatsApp Automation Engine - <a href="https://github.com/AzielCF/az-wap" target="_blank" rel="noopener noreferrer" class="hover:text-primary transition-colors">Source Code</a></p>
        </aside>
      </footer>
    </div>

    <!-- Sidebar -->
    <div class="drawer-side z-30 shadow-2xl">
      <label for="main-drawer" aria-label="close sidebar" class="drawer-overlay"></label>
      <div class="w-72 h-screen bg-[#161a23] text-slate-300 border-r border-white/5 flex flex-col">
        <!-- Logo Area (Fixed Top) -->
        <div class="px-8 pt-10 pb-6 border-b border-white/5 flex flex-col items-center gap-3 shrink-0">
           <img src="/src/assets/azwap.svg" class="w-full max-w-[130px] h-auto object-contain" alt="Az-Wap Enterprise" />
           <div class="flex flex-col items-center gap-1.5">
              <span class="text-xs font-bold text-slate-500 uppercase tracking-widest opacity-80">Enterprise Engine</span>
              <div class="px-3 py-1 rounded-full bg-white/5 border border-white/10 flex items-center gap-2 shadow-inner">
                  <div class="w-1 h-1 rounded-full bg-primary animate-pulse shadow-[0_0_6px_rgba(var(--p),0.6)]"></div>
                  <span class="text-xs font-black text-slate-400 tracking-widest">{{ appVersion }}</span>
              </div>
           </div>
        </div>

        <!-- Navigation (Scrollable Center) -->
        <div class="flex-1 overflow-y-auto custom-scrollbar py-6">
            <ul class="flex flex-col gap-2 px-4 w-full">
          <li>
            <RouterLink to="/" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>
              <span>Dashboard</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/workspaces" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"/></svg>
              <span>Workspaces</span>
            </RouterLink>
          </li>
          <div class="divider opacity-30 my-4"></div>
          <li>
            <RouterLink to="/monitoring" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" /></svg>
              <span>Monitoring</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/bots" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>
              <span>Bot Templates</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/mcp" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
              <span>MCP Capability</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/health" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
              <span>Platform Health</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/credentials" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" /></svg>
              <span>Credentials</span>
            </RouterLink>
          </li>
          <li>
            <RouterLink to="/clients" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" /></svg>
              <span>Clients</span>
            </RouterLink>
          </li>
          <div class="divider opacity-30 my-4"></div>
          <li>
            <RouterLink to="/settings" class="py-3.5 px-5 rounded-lg flex items-center gap-4 transition-all hover:bg-white/5 text-xs font-bold uppercase tracking-wider" active-class="bg-primary text-white shadow-xl shadow-primary/20">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
              <span>Settings</span>
            </RouterLink>
          </li>
            </ul>
        </div>

        <!-- Bottom Cluster Node Info (Fixed Bottom) -->
        <div class="p-8 bg-black/30 border-t border-white/5 shrink-0">
          <div class="flex items-center gap-4">
            <div class="bg-primary/20 text-primary rounded-lg w-10 h-10 flex items-center justify-center ring-1 ring-primary/30 shadow-inner">
                 <span class="text-xs font-bold uppercase tracking-widest">AZL</span>
            </div>
            <div>
              <p class="text-xs font-bold text-white uppercase tracking-tight">Cloud Master</p>
               <p class="text-xs text-success font-bold uppercase tracking-widest animate-pulse">Running Pulse</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
.page-enter-active, .page-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.page-enter-from { opacity: 0; transform: translateY(8px); }
.page-leave-to { opacity: 0; }
</style>
