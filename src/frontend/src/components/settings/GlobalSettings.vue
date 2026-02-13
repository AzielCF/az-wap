<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { HardDrive, Save, Loader2, Cpu } from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const saving = ref(false)
const saveStatus = ref<'success' | 'error' | null>(null)
const settings = ref({
    whatsapp_setting_max_download_size: 50 * 1024 * 1024,
    ai_typing_enabled: true,
    ai_global_system_prompt: '',
    ai_timezone: 'UTC',
    ai_debounce_ms: 3500,
    ai_wait_contact_idle_ms: 10000
})

async function loadSettings() {
    try {
        const res = await api.get('/app/settings')
        // El backend devuelve llaves como 'ai_global_system_prompt', aseguramos que se mapeen bien
        settings.value = { ...settings.value, ...res }
    } catch (err) {
        console.error('Failed to load settings', err)
    } finally {
        loading.value = false
    }
}

async function saveSettings() {
    saving.value = true
    saveStatus.value = null
    try {
        await api.put('/app/settings', settings.value)
        saveStatus.value = 'success'
        // Recargar para confirmar que el servidor tiene los datos correctos
        await loadSettings()
        
        // Quitar el mensaje de éxito tras 3 segundos
        setTimeout(() => {
            if (saveStatus.value === 'success') saveStatus.value = null
        }, 3000)
    } catch (err) {
        console.error('Failed to save settings', err)
        saveStatus.value = 'error'
    } finally {
        saving.value = false
    }
}

onMounted(loadSettings)
</script>

<template>
    <div class="bg-[#161a23] border border-white/10 rounded-3xl p-8 lg:p-12 shadow-2xl relative overflow-hidden group transition-all duration-500 hover:border-white/20">
        <!-- Toast Notification -->
        <div v-if="saveStatus" class="absolute top-6 right-6 z-50 animate-in fade-in slide-in-from-top-4 duration-300">
            <div v-if="saveStatus === 'success'" class="bg-emerald-500/20 border border-emerald-500/50 backdrop-blur-xl px-6 py-3 rounded-2xl flex items-center gap-3">
                <div class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
                <span class="text-emerald-400 text-[10px] font-black uppercase tracking-widest">Settings Synchronized</span>
            </div>
            <div v-if="saveStatus === 'error'" class="bg-rose-500/20 border border-rose-500/50 backdrop-blur-xl px-6 py-3 rounded-2xl flex items-center gap-3">
                <div class="w-2 h-2 rounded-full bg-rose-500 animate-pulse"></div>
                <span class="text-rose-400 text-[10px] font-black uppercase tracking-widest">Failed to sync settings</span>
            </div>
        </div>

        <div v-if="loading" class="flex items-center justify-center py-12">
            <Loader2 class="w-8 h-8 animate-spin text-primary" />
        </div>
        
        <div v-else class="relative z-10 space-y-10">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-10">
                <!-- Max Download Size -->
                <div class="form-control w-full space-y-4">
                    <div class="flex items-center gap-3 mb-2">
                        <HardDrive class="w-5 h-5 text-primary" />
                        <label class="text-[11px] font-black uppercase tracking-widest text-slate-400">WhatsApp Max Download Size</label>
                    </div>
                    <div class="flex items-end gap-3">
                        <div class="relative flex-1">
                            <input 
                                type="number" 
                                :value="Math.round(settings.whatsapp_setting_max_download_size / (1024 * 1024))" 
                                @input="settings.whatsapp_setting_max_download_size = ($event.target as HTMLInputElement).valueAsNumber * 1024 * 1024"
                                class="input input-bordered w-full bg-white/5 border-white/10 focus:border-primary focus:outline-none h-16 text-white text-xl font-bold font-mono pl-6" 
                            />
                            <div class="absolute right-6 top-1/2 -translate-y-1/2 text-slate-600 font-black text-xs uppercase tracking-widest pointer-events-none">MB</div>
                        </div>
                    </div>
                    <p class="text-[10px] text-slate-500 font-bold uppercase tracking-wider leading-relaxed opacity-70">
                        Default safety limit for all channels. Individual channel limits override this value.
                    </p>
                </div>

                <!-- AI Typing Toggle -->
                <div class="form-control w-full space-y-4">
                    <div class="flex items-center gap-3 mb-2">
                        <Cpu class="w-5 h-5 text-indigo-400" />
                        <label class="text-[11px] font-black uppercase tracking-widest text-slate-400">AI Global Presence (Typing)</label>
                    </div>
                    <label class="flex items-center justify-between bg-white/5 border border-white/10 rounded-2xl p-6 cursor-pointer hover:bg-white/10 transition-all h-16 group/toggle">
                        <span class="text-xs font-bold uppercase tracking-widest text-slate-300 group-hover/toggle:text-white transition-colors">Simulate Human Typing</span>
                        <input type="checkbox" v-model="settings.ai_typing_enabled" class="toggle toggle-primary" />
                    </label>
                    <p class="text-[10px] text-slate-500 font-bold uppercase tracking-wider leading-relaxed opacity-70">
                        When enabled, the bot will show "typing..." status before responding to appear more human.
                    </p>
                </div>
            </div>

            <!-- AI Global System Prompt -->
            <div class="form-control w-full space-y-4">
                <div class="flex items-center gap-3 mb-2">
                    <Cpu class="w-5 h-5 text-purple-400" />
                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-400">Global AI System Prompt</label>
                </div>
                <textarea 
                    v-model="settings.ai_global_system_prompt"
                    class="textarea textarea-bordered w-full bg-white/5 border-white/10 focus:border-primary focus:outline-none min-h-[120px] text-slate-300 font-medium leading-relaxed"
                    placeholder="Enter global instructions for all bots..."
                ></textarea>
                <p class="text-[10px] text-slate-500 font-bold uppercase tracking-wider leading-relaxed opacity-70">
                    This prompt is prepended to all bot interactions unless overridden at the channel level.
                </p>
            </div>

            <!-- Timezone -->
            <div class="form-control w-full space-y-4 max-w-md">
                <div class="flex items-center gap-3 mb-2">
                    <label class="text-[11px] font-black uppercase tracking-widest text-slate-400">Default AI Timezone</label>
                </div>
                <input 
                    type="text" 
                    v-model="settings.ai_timezone"
                    placeholder="e.g. America/New_York"
                    class="input input-bordered w-full bg-white/5 border-white/10 focus:border-primary focus:outline-none h-14 text-white font-mono" 
                />
            </div>

            <div class="pt-6 border-t border-white/5 flex justify-end">
                <button 
                    @click="saveSettings" 
                    :disabled="saving"
                    class="btn btn-primary rounded-2xl px-12 h-14 border-none uppercase text-[10px] font-black tracking-[0.2em] transition-all shadow-xl shadow-primary/20 hover:scale-[1.02] flex items-center gap-3"
                >
                    <Save v-if="!saving" class="w-4 h-4" />
                    <Loader2 v-else class="w-4 h-4 animate-spin" />
                    {{ saving ? 'Syncing...' : 'Commit Global Settings' }}
                </button>
            </div>
        </div>
        
        <!-- Aesthetic decoration -->
        <div class="absolute -top-20 -left-20 w-64 h-64 bg-primary/5 rounded-full blur-[100px]"></div>
    </div>
</template>
