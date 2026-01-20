<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { 
    HardDrive, 
    RefreshCw, 
    Settings, 
    Trash2, 
    Check, 
    Database,
    Clock,
    Save
} from 'lucide-vue-next'

const api = useApi()
const loading = ref(false)
const saving = ref(false)
const showSettings = ref(false)

const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'info' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

const stats = ref<{
    total_size: number;
    human_size: string;
}>({
    total_size: 0,
    human_size: '0 B'
})

const settings = ref({
    enabled: false,
    max_age_days: 30,
    max_size_mb: 1024,
    cleanup_interval_mins: 60
})

async function loadStats() {
    loading.value = true
    try {
        const res: any = await api.get('/api/cache/stats')
        if (res && res.results) stats.value = res.results
    } catch (err) {
        console.error('Failed to load cache stats', err)
    } finally {
        loading.value = false
    }
}

async function loadSettings() {
    try {
        const res: any = await api.get('/api/cache/settings')
        if (res && res.results) settings.value = res.results
    } catch (err) {
        console.error('Failed to load cache settings', err)
    }
}

async function saveSettings() {
    saving.value = true
    try {
        await api.put('/api/cache/settings', settings.value)
        showSettings.value = false
    } catch (err) {
        console.error('Failed to save cache settings', err)
    } finally {
        saving.value = false
    }
}

async function clearCache() {
  confirmModal.value = {
      show: true,
      title: 'Prune System Cache?',
      message: 'This will remove all temporary files, including media and documents. While safe, it might cause re-downloads for some users. Continue?',
      type: 'warning',
      confirmText: 'Prune Cache',
      onConfirm: async () => {
          try {
            loading.value = true
            await api.post('/api/cache/clear', {})
            await loadStats()
          } catch (err) {
            console.error('Failed to clear cache', err)
          } finally {
            loading.value = false
          }
      }
  }
}

function openSettings() {
    loadSettings()
    showSettings.value = true
}

onMounted(() => {
    loadStats()
})
</script>

<template>
    <div class="space-y-6">
        <!-- Main Card -->
        <div class="bg-[#161a23] border border-white/5 rounded-xl p-6 shadow-sm">
            <div class="flex items-center justify-between mb-6">
                <div class="flex items-center gap-3">
                    <div class="w-10 h-10 rounded-lg bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center">
                        <HardDrive class="w-5 h-5 text-indigo-400" />
                    </div>
                    <div>
                        <h3 class="text-lg font-bold text-white tracking-tight">System Cache</h3>
                        <p class="text-xs text-slate-500 font-medium">Global temporary storage management</p>
                    </div>
                </div>
                <div class="flex gap-2">
                    <button @click="loadStats" class="btn-premium btn-premium-ghost btn-premium-icon" :disabled="loading">
                        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
                    </button>
                    <button @click="openSettings" class="btn-premium btn-premium-ghost btn-premium-icon">
                        <Settings class="w-4 h-4" />
                    </button>
                    <button @click="clearCache" class="btn-premium btn-premium-error btn-premium-icon" :disabled="loading">
                        <Trash2 class="w-4 h-4" />
                    </button>
                </div>
            </div>

            <div class="bg-[#11141b] rounded-lg border border-white/5 p-6 flex flex-col items-center justify-center text-center">
                <div class="text-4xl font-black text-white tracking-tighter mb-1">{{ stats.human_size }}</div>
                <div class="text-xs font-bold uppercase tracking-widest text-slate-500">Total Disk Usage</div>
            </div>
            
            <div class="mt-4 flex items-center gap-2 px-2">
                <div class="w-2 h-2 rounded-full" :class="settings.enabled ? 'bg-emerald-500 animate-pulse' : 'bg-slate-700'"></div>
                <span class="text-xs font-mono text-slate-500">
                    Auto-Maintenance: <span :class="settings.enabled ? 'text-emerald-400' : 'text-slate-500'">{{ settings.enabled ? 'ACTIVE' : 'DISABLED' }}</span>
                </span>
            </div>
        </div>

        <!-- Settings Modal (Inline) -->
        <div v-if="showSettings" class="bg-[#161a23] border border-indigo-500/20 rounded-xl p-6 animate-in fade-in zoom-in duration-200 ring-1 ring-indigo-500/20 shadow-2xl relative overflow-hidden">
            <div class="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-indigo-500 to-purple-500"></div>
            
            <div class="flex items-center justify-between mb-6">
                 <div class="flex items-center gap-3">
                    <Settings class="w-5 h-5 text-indigo-400" />
                    <h3 class="text-lg font-bold text-white">Maintenance Config</h3>
                </div>
                <button @click="showSettings = false" class="text-slate-500 hover:text-white transition-colors text-xs font-bold uppercase">Close</button>
            </div>

            <div class="space-y-4">
                <!-- Enable Switch -->
                <div class="flex items-center justify-between p-4 bg-[#11141b] rounded-lg border border-white/5">
                    <div class="flex items-center gap-3">
                        <Database class="w-5 h-5 text-indigo-400" />
                        <div>
                            <div class="text-sm font-bold text-white">Background Cleanup</div>
                            <div class="text-xs text-slate-500">Automatically delete old files to free up space</div>
                        </div>
                    </div>
                    <input type="checkbox" v-model="settings.enabled" class="toggle toggle-primary toggle-sm" />
                </div>

                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div class="form-control">
                        <label class="label">
                            <span class="label-text text-xs font-bold uppercase text-slate-500">Max File Age (Days)</span>
                        </label>
                        <input type="number" v-model.number="settings.max_age_days" class="input-premium w-full text-sm" placeholder="30" />
                    </div>
                    <div class="form-control">
                        <label class="label">
                            <span class="label-text text-xs font-bold uppercase text-slate-500">Max Cache Size (MB)</span>
                        </label>
                         <input type="number" v-model.number="settings.max_size_mb" class="input-premium w-full text-sm" placeholder="1024" />
                    </div>
                     <div class="form-control md:col-span-2">
                        <label class="label">
                            <span class="label-text text-xs font-bold uppercase text-slate-500">Run Interval (Minutes)</span>
                        </label>
                         <div class="relative">
                            <Clock class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-600" />
                            <input type="number" v-model.number="settings.cleanup_interval_mins" class="input-premium pl-9 w-full text-sm" placeholder="60" />
                        </div>
                    </div>
                </div>

                <div class="pt-4 flex justify-end gap-3">
                    <button class="btn btn-sm btn-ghost text-slate-400 hover:text-white" @click="showSettings = false">Cancel</button>
                    <button class="btn btn-sm bg-indigo-600 hover:bg-indigo-500 text-white border-0" @click="saveSettings" :disabled="saving">
                        <Save class="w-4 h-4 mr-1.5" />
                        Save Configuration
                    </button>
                </div>
            </div>
        </div>
    </div>
    <ConfirmationDialog 
        v-model="confirmModal.show"
        :title="confirmModal.title"
        :message="confirmModal.message"
        :type="confirmModal.type"
        :confirmText="confirmModal.confirmText"
        @confirm="confirmModal.onConfirm"
    />
</template>
