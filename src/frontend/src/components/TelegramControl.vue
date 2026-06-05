<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useApi } from '@/composables/useApi'
import { Send, ShieldAlert } from 'lucide-vue-next'

const props = defineProps<{
  channel: any
  workspaceId: string
}>()

const api = useApi()
const status = ref({
  connected: false,
  loggedIn: false,
  loading: true,
  botName: ''
})

let interval: any = null
const botToken = ref('')

async function fetchStatus(manual = false) {
  try {
    const data = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/telegram/status`)
    status.value.connected = data.is_connected
    status.value.loggedIn = data.is_logged_in
    if (data.bot_name) status.value.botName = data.bot_name
    status.value.loading = false
  } catch (err) {
    console.error(err)
  }
}

async function connectBot() {
  if (!botToken.value) return
  status.value.loading = true
  
  try {
    const res = await api.post(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/telegram/token`, {
        token: botToken.value
    })
    
    if (res.status === 'connected') {
        status.value.loggedIn = true
        status.value.botName = res.bot_name || 'Telegram Bot'
        botToken.value = ''
        await fetchStatus()
    }
  } catch (err) {
    alert('Failed to connect bot. Please check your token and try again.')
    console.error(err)
  } finally {
    status.value.loading = false
  }
}

async function logout() {
  if (!confirm("Are you sure you want to disconnect this Telegram bot?")) return
  
  try {
    await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/telegram/logout`)
    status.value.loggedIn = false
    status.value.botName = ''
    await fetchStatus()
  } catch (err) {
    console.error('[LOGOUT] Error:', err)
    alert('Logout error: ' + (err as any)?.message)
  }
}

function refresh() {
    status.value.loading = true
    fetchStatus(true)
}

onMounted(() => {
  fetchStatus()
  interval = setInterval(fetchStatus, 4000)
})

onBeforeUnmount(() => {
  if (interval) clearInterval(interval)
})
</script>

<template>
  <div class="space-y-8 animate-in fade-in duration-500">
    <!-- Header Controls -->
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between border-b border-white/5 pb-6 gap-4">
        <div class="space-y-1">
            <h4 class="text-xs font-black text-white uppercase tracking-widest">Telegram Connection</h4>
            <p class="text-xs text-slate-500 font-bold uppercase tracking-widest font-mono">NODE: {{ channel.id.substring(0,12) }}</p>
        </div>
        <div class="flex gap-3">
            <div class="badge-premium" :class="status.connected ? 'badge-success' : 'badge-ghost opacity-40'">
                {{ status.connected ? 'BOT CONNECTED' : 'BOT OFFLINE' }}
            </div>
        </div>
    </div>

    <!-- Status Display -->
    <div class="relative group">
        <!-- Decoration -->
        <div class="absolute -inset-1 bg-gradient-to-r from-primary/10 to-[#0088cc]/10 rounded-3xl blur opacity-20 group-hover:opacity-40 transition-opacity"></div>
        
        <div class="relative flex flex-col items-center py-10 bg-[#0b0e14] border border-white/10 rounded-3xl overflow-hidden shadow-2xl">
            <!-- Logged In View -->
            <div v-if="status.loggedIn" class="flex flex-col items-center py-6 animate-in zoom-in duration-500 w-full px-10">
                <div class="relative mb-6">
                    <div class="w-24 h-24 sm:w-32 sm:h-32 rounded-full overflow-hidden border-4 border-[#0088cc]/20 shadow-2xl bg-black/40 flex items-center justify-center relative">
                        <Send class="w-12 h-12 text-[#0088cc]" />
                    </div>
                    
                    <!-- Status Dot -->
                    <div class="absolute bottom-1 right-1 sm:bottom-2 sm:right-2 w-5 h-5 sm:w-6 sm:h-6 rounded-full bg-[#0b0e14] flex items-center justify-center border border-white/5">
                        <div class="w-2.5 h-2.5 sm:w-3 sm:h-3 rounded-full bg-[#0088cc] animate-pulse"></div>
                    </div>
                </div>

                <div class="text-center">
                    <h3 class="text-xl sm:text-2xl font-black text-white uppercase tracking-tighter mb-1">{{ status.botName || 'Telegram Bot' }}</h3>
                    <p class="text-[10px] sm:text-xs text-slate-500 font-bold uppercase tracking-widest">Bot Authorized & Syncing</p>
                </div>
            </div>

            <!-- Ready / Waiting View -->
            <div v-else class="flex flex-col items-center py-8 sm:py-12 select-none w-full px-6">
                <div class="w-16 h-16 sm:w-20 sm:h-20 mb-6 flex items-center justify-center bg-[#0088cc]/10 rounded-full ring-1 ring-[#0088cc]/20 text-[#0088cc]">
                    <Send class="w-8 h-8" />
                </div>
                <!-- Warning Box / Token Notice -->
                <div class="p-4 sm:p-6 bg-amber-500/5 border border-amber-500/10 rounded-2xl flex flex-col sm:flex-row gap-4 w-full max-w-sm shrink-0 items-center justify-center ring-1 ring-amber-500/20 text-center">
                    <ShieldAlert class="w-5 h-5 text-amber-500 shrink-0" />
                    <div class="text-[10px] sm:text-xs text-amber-500/80 leading-relaxed font-bold uppercase tracking-wide">
                        Please provide a BotToken to activate your Telegram Endpoint.
                    </div>
                </div>
            </div>

            <!-- Status Indicator Overlay (Bottom) -->
            <div v-if="status.loading" class="absolute inset-0 bg-[#0b0e14]/60 backdrop-blur-[2px] flex items-center justify-center z-10 transition-all">
                <span class="loading loading-ring loading-lg text-[#0088cc]"></span>
            </div>
        </div>
    </div>

    <!-- Action Bar -->
    <div class="grid grid-cols-1 gap-4">
        <!-- Login Methods -->
        <div v-if="!status.loggedIn" class="space-y-6">
            <div class="space-y-4 animate-in fade-in slide-in-from-bottom-2">
                 <div class="form-control">
                    <label class="label-premium px-1 text-[#0088cc]">Bot Father Token</label>
                    <input v-model="botToken" type="password" placeholder="123456789:ABCdefGHI..." class="input-premium bg-black/40 text-center font-mono text-xs sm:text-sm tracking-widest w-full h-14 sm:h-16" />
                    <p class="text-[10px] sm:text-xs text-slate-600 font-bold uppercase tracking-widest mt-2 px-1">Ensure the token is correct to avoid connection errors</p>
                </div>
                <button @click="connectBot" class="btn-premium w-full h-14 sm:h-16 bg-[#0088cc]/10 hover:bg-[#0088cc]/20 border border-[#0088cc]/20 text-[#0088cc] shadow-lg shadow-[#0088cc]/5" :disabled="status.loading || !botToken">
                    Authenticate Bot
                </button>
            </div>
        </div>

        <!-- Logout Action -->
        <div v-if="status.loggedIn" class="flex flex-col gap-4">
            <button @click="logout" class="btn-premium btn-premium-ghost text-red-500/60 hover:text-red-500 border border-red-500/10 hover:bg-red-500/10 w-full h-14 sm:h-16">
                Disconnect Bot
            </button>
            <div class="p-4 sm:p-5 bg-success/5 border border-success/10 rounded-2xl flex items-center justify-center gap-3">
               <div class="w-1.5 h-1.5 rounded-full bg-[#0088cc] animate-pulse"></div>
               <p class="text-[10px] sm:text-xs text-success/70 font-black uppercase tracking-widest text-center">Service Synchronized • Real-time Sync Active</p>
            </div>
        </div>

        <button class="btn-premium btn-premium-ghost h-12 w-full text-xs opacity-40 hover:opacity-100" @click="refresh" :disabled="status.loading">
            Force Status Sync
        </button>
    </div>
  </div>
</template>
