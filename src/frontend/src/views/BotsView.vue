```
<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useApi } from '@/composables/useApi'
import AppTabModal from '@/components/AppTabModal.vue'
import BotsHeader from '@/components/bots/header.vue'
import GlobalSettingsModal from '@/components/bots/GlobalSettingsModal.vue'
import BotFormModal from '@/components/bots/BotFormModal.vue'
import AppModal from '@/components/AppModal.vue'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { 
  Bot, 
  Trash2, 
  Edit3, 
  Mic,
  Image,
  Video,
  FileText,
  Brain,
  Search
} from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const bots = ref<any[]>([])
const credentials = ref<any[]>([])
const availableModels = ref<Record<string, any[]>>({})

// Global AI Settings
const showGlobalSettings = ref(false)
const globalSettings = ref({
  global_system_prompt: '',
  timezone: 'UTC',
  debounce_ms: 1500,
  wait_contact_idle_ms: 5000,
  typing_enabled: true
})

 // timezones removed to component

const aiKinds = ['ai', 'gemini', 'openai', 'claude']
const aiCredentials = computed(() => credentials.value.filter(c => aiKinds.includes(c.kind)))
const chatwootCredentials = computed(() => credentials.value.filter(c => c.kind === 'chatwoot'))

// Modal state
const showAddBot = ref(false)
const editingBot = ref<any>(null)

// Confirmation State
const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'info' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

const activeTab = ref('general')
const search = ref('')

const filteredBots = computed(() => {
  if (!search.value) return bots.value
  const s = search.value.toLowerCase()
  return bots.value.filter(b => 
    b.name.toLowerCase().includes(s) || 
    b.description?.toLowerCase().includes(s)
  )
})

async function loadData() {
  loading.value = true
  try {
    const responses = (await Promise.all([
      api.get('/bots'),
      api.get('/credentials'),
      api.get('/settings/ai'),
      api.get('/bots/config/models')
    ])) as any[]
    
    const [bts, creds, settings, models] = responses
    bots.value = bts?.results || []
    credentials.value = creds?.results || []
    availableModels.value = models?.results || {}
    if (settings && settings.results) {
      globalSettings.value = {
        global_system_prompt: settings.results.global_system_prompt || '',
        timezone: settings.results.timezone || 'UTC',
        debounce_ms: settings.results.debounce_ms ?? 1500,
        wait_contact_idle_ms: settings.results.wait_contact_idle_ms ?? 5000,
        typing_enabled: settings.results.typing_enabled ?? true
      }
    }
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function saveGlobalSettings() {
  try {
    await api.put('/settings/ai', globalSettings.value)
    showGlobalSettings.value = false
  } catch (err) {
    alert('Failed to update settings.')
  }
}

async function deleteBot(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Delete Identity?',
      message: 'This bot identity will be permanently destroyed. Active instances using this bot will fallback to manual mode.',
      type: 'danger',
      confirmText: 'Delete Forever',
      onConfirm: async () => {
          try {
            await api.delete(`/bots/${id}`)
            await loadData()
          } catch (err) {
            alert('Failed to delete.')
          }
      }
  }
}

async function clearBotMemory(id: string) {
  confirmModal.value = {
      show: true,
      title: 'Flush Memory Core?',
      message: 'EXTREME WARNING: This will wipe ALL short-term memory for this bot entity across ALL active users and instances. This action cannot be undone. Are you sure you want to trigger a global amnesia event?',
      type: 'danger',
      confirmText: 'Execute Flush',
      onConfirm: async () => {
          try {
            await api.post(`/bots/${id}/memory/clear`, {})
            alert('Memory core flushed successfully.')
          } catch (err) {
            alert('Failed to clear memory.')
          }
      }
  }
}

function openEdit(bot: any) {
  editingBot.value = bot
  showAddBot.value = true
}

function openAdd() {
  editingBot.value = null
  showAddBot.value = true
}

onMounted(loadData)
</script>

<template>
  <div v-if="loading" class="flex justify-center py-40">
    <span class="loading loading-ring loading-lg text-primary"></span>
  </div>

  <div v-else class="space-y-12 animate-in fade-in duration-700 max-w-[1400px] mx-auto pb-20">
    <!-- Header -->
    <BotsHeader @openSettings="showGlobalSettings = true" @openAdd="openAdd" />

    <!-- Content Area -->
    <div class="px-6 lg:px-0">
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-6 mb-10">
            <div class="section-title-premium text-primary/60 mb-0">Reusable Bot Templates</div>
            <div class="relative group w-full md:w-80">
                <Search class="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-600 group-focus-within:text-primary transition-colors" />
                <input v-model="search" type="text" placeholder="Search templates..." class="input-premium h-12 pl-12 w-full text-sm" />
            </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-10">
            <!-- Add Card -->
            <div @click="openAdd" class="bg-[#161a23]/30 border-2 border-dashed border-white/5 hover:border-primary/30 rounded-[2.5rem] p-10 flex flex-col items-center justify-center text-center group cursor-pointer transition-all duration-500 min-h-[340px]">
                <div class="w-16 h-16 rounded-[1.5rem] bg-white/5 flex items-center justify-center text-slate-600 group-hover:bg-primary/10 group-hover:text-primary transition-all mb-6 border border-white/5 shadow-2xl">
                    <Plus class="w-8 h-8" />
                </div>
                <h4 class="text-sm font-black text-white/40 group-hover:text-white uppercase tracking-widest transition-all">Compose Template</h4>
            </div>

            <!-- Bot Cards -->
            <div v-for="bot in filteredBots" :key="bot.id" class="card-premium min-h-[340px]">
                <div class="relative z-10 flex flex-col h-full">
                    <div class="flex justify-between items-start mb-10">
                        <div class="icon-box-premium icon-box-primary">
                            <Bot class="w-8 h-8" />
                        </div>
                        <div class="flex gap-2">
                            <button class="btn-card-action" @click="openEdit(bot)">
                                <Edit3 class="w-4 h-4" />
                            </button>
                            <button class="btn-card-action btn-card-action-red" @click="deleteBot(bot.id)">
                                <Trash2 class="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                    
                    <h4 class="text-2xl font-black text-white uppercase tracking-tighter mb-4 group-hover:text-primary transition-colors leading-none truncate">{{ bot.name }}</h4>
                    <p class="text-sm text-slate-500 font-bold uppercase tracking-tight leading-relaxed line-clamp-2 h-10 opacity-60">{{ bot.description || 'Professional identity blueprint.' }}</p>
                    
                    <div class="flex gap-2 mt-6">
                        <div v-if="bot.audio_enabled !== false" class="p-1.5 rounded-lg bg-indigo-500/10 border border-indigo-500/20 text-indigo-400" title="Audio Capable">
                            <Mic class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.image_enabled !== false" class="p-1.5 rounded-lg bg-pink-500/10 border border-pink-500/20 text-pink-400" title="Vision Capable">
                            <Image class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.video_enabled" class="p-1.5 rounded-lg bg-cyan-500/10 border border-cyan-500/20 text-cyan-400" title="Video Capable">
                            <Video class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.document_enabled" class="p-1.5 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-400" title="Document Processing">
                            <FileText class="w-3.5 h-3.5" />
                        </div>
                        <div v-if="bot.memory_enabled !== false" class="p-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400" title="Long-term Memory">
                            <Brain class="w-3.5 h-3.5" />
                        </div>
                    </div>
                    
                    <div class="mt-auto pt-8 border-t border-white/5 flex items-center justify-between">
                        <div class="badge-premium badge-success gap-2">
                            <div class="w-2 h-2 rounded-full bg-success shadow-[0_0_8px_rgba(var(--su),0.5)]"></div>
                            <span>Ready</span>
                        </div>
                        <span class="text-xs font-mono text-slate-700 font-bold uppercase">{{ bot.id.substring(0,8) }}</span>
                    </div>
                </div>
                <div class="absolute -bottom-10 -right-10 w-40 h-40 bg-primary/5 rounded-full blur-[60px] group-hover:bg-primary/10 transition-colors duration-700"></div>
            </div>
        </div>
    </div>

    <!-- Global Engine Modal -->
    <GlobalSettingsModal
        v-model="showGlobalSettings"
        v-model:settings="globalSettings"
        @save="saveGlobalSettings"
    />

    <!-- Bot Identity Modal -->
    <BotFormModal
        v-model="showAddBot"
        :editing-bot="editingBot"
        :credentials="credentials"
        :available-models="availableModels"
        @saved="loadData"
        @clear-memory="clearBotMemory"
    />

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

<style scoped>
</style>
