<template>
  <div v-if="sessions.length > 0" class="w-full bg-base-900/50 backdrop-blur-md border-b border-primary/20 py-2 overflow-hidden animate-in slide-in-from-top duration-500">
    <div class="max-w-[1600px] mx-auto px-6 flex items-center gap-4">
      <div class="flex items-center gap-2 shrink-0">
        <span class="relative flex h-2 w-2">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2 w-2 bg-success"></span>
        </span>
        <span class="text-[10px] font-black uppercase tracking-widest text-success/80">Sesiones Vivas</span>
      </div>
      
      <div class="flex gap-2 overflow-x-auto no-scrollbar py-1">
        <div v-for="session in sessions" :key="session" 
             class="flex items-center gap-2 px-3 py-1.5 rounded-full bg-success/5 border border-success/30 animate-pulse-slow">
          <div class="w-1.5 h-1.5 rounded-full bg-success shadow-[0_0_8px_rgba(34,197,94,0.8)]"></div>
          <span class="text-[10px] font-mono font-bold text-success/90 whitespace-nowrap">{{ formatSession(session) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'

const api = useApi()
const sessions = ref([])
let timer = null

const fetchSessions = async () => {
  try {
    const res = await api.get('/api/workspaces/active-sessions')
    sessions.value = res || []
  } catch (err) {
    console.error('Error fetching active sessions:', err)
  }
}

const formatSession = (key) => {
  // key format: channelID|chatID|senderID
  const parts = key.split('|')
  if (parts.length >= 2) {
    return `${parts[1]} (${parts[0]})`
  }
  return key
}

onMounted(() => {
  fetchSessions()
  timer = setInterval(fetchSessions, 2000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}
.no-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

@keyframes pulse-slow {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.8; transform: scale(0.98); }
}
.animate-pulse-slow {
  animation: pulse-slow 3s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
</style>
