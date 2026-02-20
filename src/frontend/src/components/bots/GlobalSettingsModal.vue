<script setup lang="ts">
import { ref } from 'vue'
import AppModal from '@/components/AppModal.vue'
import { Zap, Globe, Clock, Activity, Type } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: boolean
  settings: {
    global_system_prompt: string
    timezone: string
    debounce_ms: number
    wait_contact_idle_ms: number
    typing_enabled: boolean
  }
}>()

const emit = defineEmits(['update:modelValue', 'update:settings', 'save'])

const timezones = [
  { value: 'UTC', label: '(Use server default / UTC)' },
  { value: 'America/Bogota', label: 'America/Bogota' },
  { value: 'America/Lima', label: 'America/Lima' },
  { value: 'America/Mexico_City', label: 'America/Mexico_City' },
  { value: 'America/Santo_Domingo', label: 'America/Santo_Domingo (República Dominicana)' },
  { value: 'America/Santiago', label: 'America/Santiago' },
  { value: 'America/Argentina/Buenos_Aires', label: 'America/Argentina/Buenos_Aires' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles' },
  { value: 'America/New_York', label: 'America/New_York' },
  { value: 'Europe/Madrid', label: 'Europe/Madrid' },
  { value: 'Europe/London', label: 'Europe/London' }
]

function emitSettingUpdate(key: string, value: any) {
    emit('update:settings', { ...props.settings, [key]: value })
}
</script>

<template>
    <AppModal 
      :model-value="modelValue" 
      @update:model-value="emit('update:modelValue', $event)" 
      title="Engine Global Override" 
      maxWidth="max-w-xl"
    >
        <div class="space-y-8 py-4">
            <div class="form-control">
                <div class="flex items-center gap-2 mb-2">
                    <Zap class="w-3 h-3 text-primary" />
                    <label class="label-premium mb-0">Master System Prompt</label>
                </div>
                <textarea :value="settings.global_system_prompt" @input="emitSettingUpdate('global_system_prompt', ($event.target as HTMLTextAreaElement).value)" rows="5" class="input-premium w-full min-h-[120px] leading-relaxed text-sm" placeholder="Universal laws..."></textarea>
            </div>
            <div class="form-control">
                <div class="flex items-center gap-2 mb-2">
                    <Globe class="w-3 h-3 text-slate-400" />
                    <label class="label-premium mb-0">AI Timezone (IANA)</label>
                </div>
                <select :value="settings.timezone" @change="emitSettingUpdate('timezone', ($event.target as HTMLSelectElement).value)" class="select-premium h-14 w-full text-sm font-bold uppercase">
                    <option v-for="tz in timezones" :key="tz.value" :value="tz.value">{{ tz.label }}</option>
                </select>
            </div>
            <div class="grid grid-cols-2 gap-6">
                <div class="form-control">
                    <div class="flex items-center gap-2 mb-2">
                        <Clock class="w-3 h-3 text-primary" />
                        <label class="label-premium mb-0">Response Delay (ms)</label>
                    </div>
                    <input :value="settings.debounce_ms" @input="emitSettingUpdate('debounce_ms', Number(($event.target as HTMLInputElement).value))" type="number" class="input-premium h-14 w-full font-mono text-sm" />
                </div>
                <div class="form-control">
                    <div class="flex items-center gap-2 mb-2">
                        <Activity class="w-3 h-3 text-primary" />
                        <label class="label-premium mb-0">Idle Check (ms)</label>
                    </div>
                    <input :value="settings.wait_contact_idle_ms" @input="emitSettingUpdate('wait_contact_idle_ms', Number(($event.target as HTMLInputElement).value))" type="number" class="input-premium h-14 w-full font-mono text-sm" />
                </div>
            </div>
            <label class="flex items-center justify-between h-14 bg-[#161a23] border border-white/10 rounded-xl px-6 cursor-pointer hover:border-success/40 transition-colors">
                <div class="flex items-center gap-3">
                    <Type class="w-4 h-4 text-success" />
                    <span class="text-xs font-black uppercase tracking-widest text-slate-400">Emulate Typing</span>
                </div>
                <input type="checkbox" :checked="settings.typing_enabled" @change="emitSettingUpdate('typing_enabled', ($event.target as HTMLInputElement).checked)" class="toggle toggle-success" />
            </label>
        </div>
        <template #actions>
            <button class="btn-premium btn-premium-ghost px-8" @click="emit('update:modelValue', false)">Discard</button>
            <button class="btn-premium btn-premium-success px-12" @click="emit('save')">Propagate Changes</button>
        </template>
    </AppModal>
</template>
