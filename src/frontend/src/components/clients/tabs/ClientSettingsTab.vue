<script setup lang="ts">
import { computed } from 'vue'
import { Globe } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: any
}>()

const emit = defineEmits(['update:modelValue'])

const client = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})
</script>

<template>
  <div class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
    <header>
      <h3 class="text-xl font-black text-white uppercase tracking-tight">Preferences</h3>
      <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Language, timezone and internal notes</p>
    </header>

    <div class="form-control max-w-sm">
      <label class="label-premium text-slate-400">Primary AI Language</label>
      <div class="relative group">
        <div class="absolute left-4 top-1/2 -translate-y-1/2 text-primary opacity-40 group-focus-within:opacity-100 transition-opacity">
          <Globe class="w-5 h-5" />
        </div>
        <select v-model="client.language" class="input-premium h-14 pl-12 w-full text-sm font-bold">
          <option value="es">Español (Castellano)</option>
          <option value="en">English (International)</option>
          <option value="pt">Português</option>
          <option value="fr">Français</option>
          <option value="it">Italiano</option>
          <option value="de">Deutsch</option>
        </select>
      </div>
    </div>

    <div class="form-control max-w-sm">
      <label class="label-premium text-slate-400">Timezone Override</label>
      <div class="relative group">
        <div class="absolute left-4 top-1/2 -translate-y-1/2 text-primary opacity-40 group-focus-within:opacity-100 transition-opacity">
          <Globe class="w-5 h-5" />
        </div>
        <select v-model="client.timezone" class="input-premium h-14 pl-12 w-full text-sm font-bold">
          <option value="">Use Channel Default</option>
          <option value="America/New_York">🇺🇸 (GMT-05:00) America/New_York</option>
          <option value="America/Los_Angeles">🇺🇸 (GMT-08:00) America/Los_Angeles</option>
          <option value="America/Chicago">🇺🇸 (GMT-06:00) America/Chicago</option>
          <option value="America/Lima">🇵🇪 (GMT-05:00) America/Lima</option>
          <option value="America/Bogota">🇨🇴 (GMT-05:00) America/Bogota</option>
          <option value="America/Mexico_City">🇲🇽 (GMT-06:00) America/Mexico_City</option>
          <option value="America/Santo_Domingo">🇩🇴 (GMT-04:00) America/Santo_Domingo</option>
          <option value="America/Sao_Paulo">🇧🇷 (GMT-03:00) America/Sao_Paulo</option>
          <option value="America/Buenos_Aires">🇦🇷 (GMT-03:00) America/Buenos_Aires</option>
          <option value="America/Santiago">🇨🇱 (GMT-04:00) America/Santiago</option>
          <option value="Europe/London">🇬🇧 (GMT+00:00) Europe/London</option>
          <option value="Europe/Paris">🇫🇷 (GMT+01:00) Europe/Paris</option>
          <option value="Europe/Madrid">🇪🇸 (GMT+01:00) Europe/Madrid</option>
          <option value="Europe/Berlin">🇩🇪 (GMT+01:00) Europe/Berlin</option>
          <option value="Asia/Tokyo">🇯🇵 (GMT+09:00) Asia/Tokyo</option>
          <option value="Asia/Shanghai">🇨🇳 (GMT+08:00) Asia/Shanghai</option>
          <option value="Asia/Dubai">🇦🇪 (GMT+04:00) Asia/Dubai</option>
          <option value="Australia/Sydney">🇦🇺 (GMT+10:00) Australia/Sydney</option>
        </select>
      </div>
      <p class="text-xs text-slate-600 font-bold uppercase mt-2">Individual timezone for this client. Overrides channel defaults.</p>
    </div>

    <div class="form-control max-w-sm">
      <label class="label-premium text-slate-400">Country</label>
      <div class="relative group">
        <div class="absolute left-4 top-1/2 -translate-y-1/2 text-primary opacity-40 group-focus-within:opacity-100 transition-opacity">
          <Globe class="w-5 h-5" />
        </div>
        <select v-model="client.country" class="input-premium h-14 pl-12 w-full text-sm font-bold">
          <option value="">Not specified</option>
          <option value="PE">🇵🇪 PE - Perú</option>
          <option value="DO">🇩🇴 DO - República Dominicana</option>
          <option value="US">🇺🇸 US - United States</option>
          <option value="MX">🇲🇽 MX - México</option>
          <option value="CO">🇨🇴 CO - Colombia</option>
          <option value="AR">🇦🇷 AR - Argentina</option>
          <option value="CL">🇨🇱 CL - Chile</option>
          <option value="BR">🇧🇷 BR - Brasil</option>
          <option value="ES">🇪🇸 ES - España</option>
          <option value="GB">🇬🇧 GB - United Kingdom</option>
        </select>
      </div>
    </div>

    <div class="form-control">
      <label class="label-premium text-slate-400">Internal Context / Notes</label>
      <p class="text-[10px] text-slate-500 font-bold uppercase mb-2">Private observations not visible to the client or AI</p>
      <textarea v-model="client.notes" class="input-premium py-4 w-full h-32 leading-relaxed" placeholder="Client specific context or background..."></textarea>
    </div>
  </div>
</template>
