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
  <div v-if="modelValue" class="modal modal-open backdrop-blur-md z-[9999]" role="dialog">
    <div class="modal-box p-0 bg-[#0f1219] border shadow-2xl overflow-hidden max-w-sm relative group" :class="[styles.borderColor, styles.glowColor]">
      <!-- Decoration -->
      <div class="absolute top-0 inset-x-0 h-1 bg-gradient-to-r from-transparent via-white/10 to-transparent opacity-50"></div>
      
      <div class="p-8 flex flex-col items-center text-center">
        <div class="w-16 h-16 rounded-2xl bg-white/5 flex items-center justify-center mb-6 border border-white/5" :class="styles.iconColor">
           <component :is="styles.icon" class="w-8 h-8" />
        </div>
        
        <h3 class="text-xl font-black uppercase tracking-tight mb-3" :class="styles.titleColor">{{ title || 'Confirmation' }}</h3>
        
        <p class="text-slate-400 text-sm leading-relaxed font-medium mb-8">
            {{ message }}
        </p>

        <div class="grid grid-cols-2 gap-4 w-full">
            <button @click="close" class="btn h-12 bg-white/5 hover:bg-white/10 border-transparent text-slate-400 hover:text-white font-bold uppercase tracking-wider text-xs rounded-xl">
                {{ cancelText || 'Cancel' }}
            </button>
            <button @click="confirm" class="btn h-12 border-none font-black uppercase tracking-widest text-xs rounded-xl shadow-lg transition-transform active:scale-95" :class="styles.btnColor">
                {{ confirmText || 'Confirm' }}
            </button>
        </div>
      </div>
    </div>
    <div class="modal-backdrop bg-black/90" @click="close"></div>
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
