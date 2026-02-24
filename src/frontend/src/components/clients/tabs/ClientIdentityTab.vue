<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircle2, RefreshCw } from 'lucide-vue-next'
import TierBadge from '../TierBadge.vue'

const props = defineProps<{
  modelValue: any
  editingClient: any
  lidVerified: boolean
  validatingLid: boolean
  lastValidatedPhone: string
}>()

const emit = defineEmits(['update:modelValue', 'extractLid'])

const client = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})
</script>

<template>
  <div class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
    <header>
      <h3 class="text-xl font-black text-white uppercase tracking-tight">Identification</h3>
      <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Platform parameters and account tier</p>
    </header>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
      <div class="form-control">
        <label class="label-premium text-slate-400">Platform ID (LID)</label>
        <div class="flex gap-2">
          <input v-model="client.platform_id" 
                 type="text" 
                 class="input-premium h-14 flex-1 text-sm font-mono" 
                 placeholder="Identification signal..." />
          <button v-if="client.platform_type === 'whatsapp'" 
                  type="button"
                  class="btn-premium px-6 h-14" 
                  :class="lidVerified ? 'btn-premium-ghost text-green-500 border-green-500/20' : 'btn-premium-ghost'"
                  @click="emit('extractLid')"
                  :disabled="validatingLid">
              <CheckCircle2 v-if="lidVerified && !validatingLid" class="w-4 h-4" />
              <RefreshCw v-else class="w-4 h-4" :class="{ 'animate-spin': validatingLid }" />
          </button>
        </div>
        <div class="flex items-center justify-between mt-2">
          <p class="text-xs text-slate-600 font-bold uppercase">Unique identifier for the selected platform.</p>
          <p v-if="lidVerified" class="text-xs text-green-500 font-black uppercase flex items-center gap-1 animate-in fade-in slide-in-from-right-2">
               <CheckCircle2 class="w-2.5 h-2.5" /> 
               Number: {{ lastValidatedPhone }} <span class="mx-1 opacity-40">→</span> LID: {{ client.platform_id }}
          </p>
        </div>
      </div>

      <div class="form-control">
        <label class="label-premium text-slate-400">Target Platform</label>
        <select v-model="client.platform_type" class="input-premium h-14 w-full text-sm" :disabled="!!editingClient">
          <option value="whatsapp">WhatsApp Protocol</option>
          <option value="telegram">Telegram Bot API</option>
          <option value="webchat">Internal WebChat</option>
        </select>
      </div>
    </div>

    <div class="form-control">
      <label class="label-premium text-slate-400">Authorization Tier</label>
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <button v-for="t in ['standard', 'premium', 'vip', 'enterprise']" :key="t"
                @click="client.tier = t" 
                class="h-16 rounded-2xl border-2 transition-all flex items-center justify-center gap-2 cursor-pointer hover:scale-[1.02]"
                :class="client.tier === t ? 'border-primary bg-primary/10 shadow-lg' : 'border-white/5 bg-black/40 text-slate-500'">
            <TierBadge :tier="t" :show-icon="true" />
        </button>
      </div>
    </div>
  </div>
</template>
