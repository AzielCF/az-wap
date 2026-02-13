<template>
  <div class="card bg-base-100 shadow-xl border border-base-300">
    <div class="card-body p-0">
      <div class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/50">
        <h2 class="card-title text-sm font-bold flex items-center gap-2 uppercase tracking-widest text-primary">
          <Activity class="w-4 h-4" />
          Active Sessions & Typing
        </h2>
        <div class="flex items-center gap-3">
          <span class="badge badge-primary badge-sm font-mono">{{ activeCount }} Active</span>
          <span class="badge badge-outline badge-sm">{{ typingCount }} Typing</span>
        </div>
      </div>

      <div class="overflow-x-auto min-h-[120px]">
        <table class="table table-compact w-full">
          <thead class="bg-base-200/30">
            <tr>
              <th class="text-xs uppercase opacity-50 tracking-widest">Channel / Instance</th>
              <th class="text-xs uppercase opacity-50 tracking-widest">Contact (Chat JID)</th>
              <th class="text-xs uppercase opacity-50 tracking-widest">Bot State</th>
              <th class="text-xs uppercase opacity-50 tracking-widest">Human Activity</th>
              <th class="text-xs uppercase opacity-50 text-right tracking-widest">Updated</th>
              <th class="text-xs uppercase opacity-50 text-center tracking-widest">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="mergedSessions.length === 0">
              <td colspan="6" class="text-center py-10 text-base-content/40 italic">
                No active sessions at this time...
              </td>
            </tr>
            <tr v-for="session in mergedSessions" :key="session.key" 
                class="hover:bg-base-200/20 transition-colors"
                :class="{ 'active-glow': session.is_bot_active }">
              <td class="font-mono text-xs font-bold">{{ session.channel_id }}</td>
              <td class="font-mono text-xs">{{ session.chat_id }}</td>
              <td>
                <div class="flex items-center gap-2">
                  <span v-if="session.is_bot_active" class="relative flex h-2 w-2">
                    <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-success opacity-75"></span>
                    <span class="relative inline-flex rounded-full h-2 w-2 bg-success"></span>
                  </span>
                  <span class="badge badge-sm" :class="getStatusClass(session.state)">
                    {{ getStatusLabel(session.state) }}
                  </span>
                </div>
              </td>
              <td>
                <div v-if="session.typing" class="flex items-center gap-2 bg-primary/10 px-2 py-1 rounded-full border border-primary/20 transition-all duration-300">
                  <div class="flex gap-1 transform scale-75">
                    <span class="w-2 h-2 rounded-full bg-primary animate-bounce"></span>
                    <span class="w-2 h-2 rounded-full bg-primary animate-bounce delay-100" style="animation-delay: 0.1s"></span>
                    <span class="w-2 h-2 rounded-full bg-primary animate-bounce delay-200" style="animation-delay: 0.2s"></span>
                  </div>
                  <span class="text-xs font-black text-primary uppercase tracking-widest">
                    {{ session.typing.media === 'audio' ? 'Recording Audio' : 'Typing...' }}
                  </span>
                </div>
                <span v-else class="text-xs opacity-20 italic px-2">No activity</span>
              </td>
              <td class="text-right">
                <div class="flex flex-col items-end">
                  <span class="text-xs font-mono font-bold text-success" v-if="session.expires_in > 0">
                    Closes in: {{ formatTimeLeft(session.expires_in) }}
                  </span>
                  <span class="text-xs font-mono opacity-30">{{ session.time }}</span>
                </div>
              </td>
              <td class="text-center">
                <button @click="handleAction(session)" 
                        class="btn btn-ghost btn-xs border-transparent transition-all duration-300"
                        :class="session.is_pending_kill ? 'text-amber-500 animate-pulse bg-amber-500/10' : 'text-error/40 hover:text-error hover:bg-error/10'"
                        :title="session.is_pending_kill ? 'Cancel pending termination' : (session.state === 'processing' ? 'Queue termination after processing' : 'Terminate session manually')">
                  <component :is="session.is_pending_kill ? Ban : Skull" class="w-3.5 h-3.5" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Termination Dialog -->
    <ConfirmationDialog 
      v-model="confirmModal.show"
      :title="confirmModal.title"
      :message="confirmModal.message"
      :type="confirmModal.type"
      :confirmText="confirmModal.confirmText"
      @confirm="confirmModal.onConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { Activity, Skull, Ban } from 'lucide-vue-next'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'

const api = useApi()
const botSessions = ref<any[]>([])
const typingStatus = ref<any[]>([])
const pendingKills = ref<Set<string>>(new Set())
let timer: any = null

const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'danger' as 'danger' | 'warning' | 'info',
    confirmText: 'Terminate',
    onConfirm: () => {}
})

const activeCount = computed(() => botSessions.value.length)
const typingCount = computed(() => typingStatus.value.length)

// Combinar ambas fuentes de datos
const mergedSessions = computed(() => {
  const map = new Map()

  // 1. Agregar sesiones activas del bot (Estado real del motor)
  botSessions.value.forEach(s => {
    const chatID = s.chat_id
    map.set(chatID, {
      key: s.key,
      channel_id: s.channel_id,
      chat_id: chatID,
      is_bot_active: s.state === 'debouncing' || s.state === 'processing',
      state: s.state,
      is_pending_kill: pendingKills.value.has(chatID),
      expires_in: s.expires_in,
      typing: null,
      time: new Date().toLocaleTimeString()
    })
  })

  // 2. Agregar o actualizar con estado de typing (Real-time events)
  typingStatus.value.forEach(t => {
    const chatID = t.chat_id
    if (map.has(chatID)) {
      map.get(chatID).typing = t
    } else {
      map.set(chatID, {
        key: `${t.channel_id}|${chatID}`,
        channel_id: t.channel_id,
        chat_id: chatID,
        is_bot_active: false,
        is_pending_kill: pendingKills.value.has(chatID),
        state: 'idle',
        expires_in: 0,
        typing: t,
        time: new Date(t.updated_at).toLocaleTimeString()
      })
    }
  })

  return Array.from(map.values())
})

const getStatusLabel = (state: string) => {
  switch(state) {
    case 'debouncing': return 'Grouping messages'
    case 'processing': return 'Bot Thinking...'
    case 'waiting': return 'Waiting (Alive)'
    default: return 'Idle'
  }
}

const getStatusClass = (state: string) => {
  switch(state) {
    case 'debouncing': return 'badge-warning font-black'
    case 'processing': return 'badge-primary animate-pulse'
    case 'waiting': return 'badge-success opacity-80'
    default: return 'badge-ghost opacity-50'
  }
}

const formatTimeLeft = (seconds: number) => {
  if (!seconds || seconds <= 0) return ''
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

const fetchData = async () => {
  try {
    const [sessions, typing] = await Promise.all([
      api.get('/workspaces/active-sessions'),
      api.get('/api/monitoring/typing')
    ])
    botSessions.value = sessions || []
    typingStatus.value = typing || []

    // Process pending kills: if session is no longer processing, kill it
    botSessions.value.forEach(s => {
      if (pendingKills.value.has(s.chat_id) && s.state !== 'processing') {
        pendingKills.value.delete(s.chat_id)
        killSession(s)
      }
    })
  } catch (err) {
    console.error('Error fetching dynamic status:', err)
  }
}

const killSession = async (session: any) => {
  try {
    await api.delete(`/api/monitoring/sessions/${session.channel_id}/${encodeURIComponent(session.chat_id)}`)
    await fetchData()
  } catch (err) {
    console.error('Error killing session:', err)
  }
}

const confirmKill = (session: any) => {
    confirmModal.value = {
        show: true,
        title: 'Terminate Active Session?',
        message: `DANGER: You are about to forcibly kill the active worker and session state for ${session.chat_id}. This will halt current processing and trigger an automated shutdown message. Do you want to proceed?`,
        type: 'danger',
        confirmText: 'Terminate Session',
        onConfirm: () => killSession(session)
    }
}

const handleAction = (session: any) => {
    if (pendingKills.value.has(session.chat_id)) {
        pendingKills.value.delete(session.chat_id)
        return
    }

    if (session.state === 'processing') {
        confirmModal.value = {
            show: true,
            title: 'Queue Termination?',
            message: `The bot is currently processing a response for ${session.chat_id}. Would you like to queue this session for termination immediately after it finishes?`,
            type: 'warning',
            confirmText: 'Queue Termination',
            onConfirm: () => pendingKills.value.add(session.chat_id)
        }
    } else {
        confirmKill(session)
    }
}

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 2000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.animate-ping {
  animation: ping 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
}
@keyframes ping {
  75%, 100% {
    transform: scale(2);
    opacity: 0;
  }
}

.active-glow {
  box-shadow: 0 0 10px rgba(34, 197, 94, 0.4);
  animation: glow-pulse 2s infinite ease-in-out;
}

@keyframes glow-pulse {
  0%, 100% { background: rgba(34, 197, 94, 0.1); box-shadow: 0 0 5px rgba(34, 197, 94, 0.2); }
  50% { background: rgba(34, 197, 94, 0.2); box-shadow: 0 0 15px rgba(34, 197, 94, 0.5); }
}
</style>
