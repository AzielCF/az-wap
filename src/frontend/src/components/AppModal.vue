<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  title?: string
  maxWidth?: string
  noPadding?: boolean
  noScroll?: boolean
}>()

const emit = defineEmits(['update:modelValue', 'close'])

function close() {
  emit('update:modelValue', false)
  emit('close')
}
</script>

<template>
  <div v-if="modelValue" class="modal modal-open backdrop-blur-md" role="dialog">
    <div class="modal-box p-0 bg-[#0f1219] border border-white/10 shadow-[0_40px_100px_rgba(0,0,0,0.9)] flex flex-col w-[calc(100%-2rem)] sm:w-full max-h-[calc(100dvh-4rem)] sm:max-h-[85vh] transition-all duration-300 pointer-events-auto" :class="maxWidth || 'max-w-md'">
      <!-- Professional Header -->
      <header v-if="title" class="px-8 py-6 border-b border-white/5 flex items-center justify-between bg-white/[0.02] flex-none">
        <h3 class="font-bold text-xl uppercase tracking-tight text-white">{{ title }}</h3>
        <button class="btn btn-ghost btn-sm btn-square text-slate-500 hover:text-white hover:bg-white/5 transition-all" @click="close">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </header>
      
      <!-- Content Area -->
      <section 
        class="flex-1 overflow-y-auto custom-scrollbar min-h-0" 
        :class="[
          noPadding ? '' : 'p-8', 
          noScroll ? 'overflow-hidden' : ''
        ]"
      >
        <div class="min-h-full">
          <slot></slot>
        </div>
      </section>
      
      <!-- Fixed Footer Area -->
      <footer v-if="$slots.actions" class="px-8 py-6 border-t border-white/5 flex justify-end items-center gap-4 bg-black/40 flex-none mt-auto">
        <slot name="actions"></slot>
      </footer>
    </div>
    <div class="modal-backdrop bg-black/85" @click="close"></div>
  </div>
</template>

<style scoped>
.modal-box {
  animation: modal-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes modal-pop {
  0% { transform: scale(0.98) translateY(10px); opacity: 0; }
  100% { transform: scale(1) translateY(0); opacity: 1; }
}

/* Custom Scrollbar */
.custom-scrollbar::-webkit-scrollbar {
  width: 6px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.2);
}
</style>
