<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import { HardDrive, Save, Loader2, Cpu } from 'lucide-vue-next'

const api = useApi()
const loading = ref(true)
const saving = ref(false)
const settings = ref({
    whatsapp_max_download_size: 50 * 1024 * 1024,
    ai_typing_enabled: true
})

async function loadSettings() {
    try {
        const res = await api.get('/app/settings')
        settings.value = res
    } catch (err) {
        console.error('Failed to load settings', err)
    } finally {
        loading.value = false
    }
}

async function saveSettings() {
    saving.value = true
    try {
        await api.put('/app/settings', settings.value)
    } catch (err) {
        console.error('Failed to save settings', err)
    } finally {
        saving.value = false
    }
}

onMounted(loadSettings)
</script>

<template>
    <div class="bg-[#161a23] border border-white/10 rounded-3xl p-8 lg:p-12 shadow-2xl relative overflow-hidden group transition-all duration-500 hover:border-white/20">
        <div v-if="loading" class="flex items-center justify-center py-12">
            <Loader2 class="w-8 h-8 animate-spin text-primary" />
        </div>
        
        <div v-else class="relative z-10 space-y-10">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-10">
                <!-- Max Download Size -->
                <div class="form-control w-full space-y-4">
                    <div class="flex items-center gap-3 mb-2">
                        <HardDrive class="w-5 h-5 text-primary" />
                        <label class="text-[11px] font-black uppercase tracking-widest text-slate-400">Global Max Download Size</label>
                    </div>
                    <div class="flex items-end gap-3">
                        <div class="relative flex-1">
                            <input 
                                type="number" 
                                :value="Math.round(settings.whatsapp_max_download_size / (1024 * 1024))" 
                                @input="settings.whatsapp_max_download_size = ($event.target as HTMLInputElement).valueAsNumber * 1024 * 1024"
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

            <div class="pt-6 border-t border-white/5 flex justify-end">
                <button 
                    @click="saveSettings" 
                    :disabled="saving"
                    class="btn btn-primary rounded-2xl px-12 h-14 border-none uppercase text-[10px] font-black tracking-[0.2em] transition-all shadow-xl shadow-primary/20 hover:scale-[1.02] flex items-center gap-3"
                >
                    <Save v-if="!saving" class="w-4 h-4" />
                    <Loader2 v-else class="w-4 h-4 animate-spin" />
                    {{ saving ? 'Saving Application State...' : 'Commit Global Settings' }}
                </button>
            </div>
        </div>
        
        <!-- Aesthetic decoration -->
        <div class="absolute -top-20 -left-20 w-64 h-64 bg-primary/5 rounded-full blur-[100px]"></div>
    </div>
</template>
