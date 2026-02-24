<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Info, AlertOctagon, X } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: boolean
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  type?: 'danger' | 'warning' | 'info'
}>()

const emit = defineEmits(['update:modelValue', 'confirm', 'cancel'])

const styles = computed(() => {
  switch (props.type) {
    case 'danger':
      return {
        icon: AlertOctagon,
        iconColor: 'text-red-500',
        borderColor: 'border-red-500/30',
        glowColor: 'shadow-red-900/20',
        btnColor: 'bg-red-600 hover:bg-red-500 text-white',
        titleColor: 'text-red-500'
      }
    case 'warning':
      return {
        icon: AlertTriangle,
        iconColor: 'text-amber-500',
        borderColor: 'border-amber-500/30',
        glowColor: 'shadow-amber-900/20',
        btnColor: 'bg-amber-600 hover:bg-amber-500 text-black',
        titleColor: 'text-amber-500'
      }
    default:
      return {
        icon: Info,
        iconColor: 'text-indigo-500',
        borderColor: 'border-white/10',
        glowColor: 'shadow-indigo-900/20',
        btnColor: 'bg-indigo-600 hover:bg-indigo-500 text-white',
        titleColor: 'text-white'
      }
  }
})

function close() {
  emit('update:modelValue', false)
  emit('cancel')
}

function confirm() {
  emit('confirm')
  emit('update:modelValue', false)
}
</script>

<template>
  <div v-if="modelValue" class="modal modal-open modal-bottom sm:modal-middle backdrop-blur-sm z-[9999] transition-all duration-300" role="dialog">
    <div class="modal-box p-0 bg-[#0c0f16] border shadow-2xl overflow-hidden max-w-md relative group" :class="[styles.borderColor, styles.glowColor]">
      <!-- Refined decoration -->
      <div class="absolute top-0 inset-x-0 h-[1px] bg-gradient-to-r from-transparent via-white/20 to-transparent opacity-50"></div>
      
      <div class="p-8 md:p-10 flex flex-col items-center text-center">
        <div class="w-20 h-20 rounded-3xl bg-white/[0.03] flex items-center justify-center mb-8 border border-white/10 shadow-inner group-hover:scale-110 transition-transform duration-500" :class="styles.iconColor">
           <component :is="styles.icon" class="w-10 h-10" />
        </div>
        
        <h3 class="text-2xl font-black uppercase tracking-tight mb-4" :class="styles.titleColor">{{ title || 'Confirmation' }}</h3>
        
        <p class="text-slate-400 text-sm md:text-base leading-relaxed font-medium mb-10 max-w-[280px] md:max-w-xs">
            {{ message }}
        </p>

        <div class="flex flex-col sm:grid sm:grid-cols-2 gap-3 w-full">
            <button @click="close" class="btn h-14 bg-white/5 hover:bg-white/10 border-white/5 text-slate-400 hover:text-white font-bold uppercase tracking-wider text-xs rounded-2xl transition-all order-2 sm:order-1">
                {{ cancelText || 'Cancel' }}
            </button>
            <button @click="confirm" class="btn h-14 border-none font-black uppercase tracking-widest text-xs rounded-2xl shadow-xl transition-all hover:brightness-110 active:scale-95 order-1 sm:order-2" :class="styles.btnColor">
                {{ confirmText || 'Confirm' }}
            </button>
        </div>
      </div>
    </div>
    <div class="modal-backdrop bg-[#050608]/80 cursor-default" @click="close"></div>
  </div>
</template>

<style scoped>
.modal-box {
  animation: modal-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modal-pop {
  0% { transform: scale(0.95) translateY(10px); opacity: 0; }
  100% { transform: scale(1) translateY(0); opacity: 1; }
}
</style>
