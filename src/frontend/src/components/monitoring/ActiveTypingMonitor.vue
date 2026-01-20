<template>
  <div class="card bg-base-100 shadow-xl border border-base-300">
    <div class="card-body p-4">
      <div class="flex items-center justify-between mb-4">
        <h2 class="card-title text-sm font-bold flex items-center gap-2">
          <span class="relative flex h-3 w-3">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75"></span>
            <span class="relative inline-flex rounded-full h-3 w-3 bg-primary"></span>
          </span>
          Actividad en Tiempo Real
        </h2>
        <span class="badge badge-outline badge-sm">{{ activeChats.length }} chats</span>
      </div>

      <div class="space-y-3 max-h-64 overflow-y-auto pr-2 custom-scrollbar">
        <div v-if="activeChats.length === 0" class="text-center py-8 text-base-content/50 italic text-sm">
          No hay actividad de escritura en este momento...
        </div>
        
        <transition-group name="list">
          <div v-for="chat in activeChats" :key="chat.chat_jid" 
               class="flex items-center justify-between p-3 rounded-lg bg-base-200/50 border border-base-300 group hover:border-primary/50 transition-all">
            <div class="flex flex-col min-w-0">
              <span class="font-mono text-xs font-bold truncate">{{ chat.chat_jid }}</span>
              <span class="text-[10px] opacity-70 flex items-center gap-1 uppercase tracking-wider">
                <i :class="chat.media === 'audio' ? 'text-error' : 'text-primary'">
                  ●
                </i>
                {{ chat.media === 'audio' ? 'Grabando Audio' : 'Escribiendo...' }}
              </span>
            </div>
            <div class="flex flex-col items-end shrink-0">
              <span class="text-[10px] font-mono opacity-50">{{ chat.instance_id }}</span>
              <div v-if="chat.media === 'text'" class="flex gap-1 mt-1">
                <span class="dot-typing"></span>
              </div>
              <div v-else class="flex gap-0.5 mt-1">
                <div v-for="i in 3" :key="i" class="w-1 bg-error h-2 animate-pulse" :style="`animation-delay: ${i*0.2}s`"></div>
              </div>
            </div>
          </div>
        </transition-group>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'

const api = useApi()
const activeChats = ref([])
let timer = null

const fetchTypingStatus = async () => {
  try {
    const res = await api.get('/api/monitoring/typing')
    activeChats.value = res || []
  } catch (err) {
    console.error('Error fetching typing status:', err)
  }
}

onMounted(() => {
  fetchTypingStatus()
  timer = setInterval(fetchTypingStatus, 2000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: hsl(var(--bc) / 0.2);
  border-radius: 10px;
}

.list-enter-active, .list-leave-active {
  transition: all 0.3s ease;
}
.list-enter-from, .list-leave-to {
  opacity: 0;
  transform: translateX(10px);
}

.dot-typing {
  position: relative;
  width: 4px;
  height: 4px;
  border-radius: 5px;
  background-color: currentColor;
  color: hsl(var(--p));
  animation: dot-typing 1.5s infinite linear;
}

@keyframes dot-typing {
  0% { box-shadow: 8px 0 0 0 transparent, 16px 0 0 0 transparent; }
  25% { box-shadow: 8px 0 0 0 hsl(var(--p)), 16px 0 0 0 transparent; }
  50% { box-shadow: 8px 0 0 0 transparent, 16px 0 0 0 hsl(var(--p)); }
  75% { box-shadow: 8px 0 0 0 transparent, 16px 0 0 0 transparent; }
  100% { box-shadow: 8px 0 0 0 transparent, 16px 0 0 0 transparent; }
}
</style>
