<script setup lang="ts">
import { CheckCircle2 } from 'lucide-vue-next'
import AppModal from './AppModal.vue'

const props = defineProps<{
  modelValue: boolean
  title: string
  activeTab: string
  tabs: Array<{ id: string; label: string; icon: any }>
  maxWidth?: string
  identity?: {
    name: string
    subtitle?: string
    id?: string
    icon?: any
    iconType?: 'component' | 'svg'
    iconClass?: string
  }
  footerActions?: boolean
  saveText?: string
  discardText?: string
  loading?: boolean
}>()

const emit = defineEmits(['update:modelValue', 'update:activeTab', 'save', 'cancel'])

function copyId(id: string) {
    if (!id) return
    navigator.clipboard.writeText(id)
}
</script>

<template>
  <AppModal :modelValue="modelValue" @update:modelValue="emit('update:modelValue', $event)" @close="emit('cancel')" :title="title" :maxWidth="maxWidth || 'max-w-6xl'" noPadding noScroll>
    <div class="flex flex-col flex-1 min-h-0 w-full overflow-hidden bg-[#0b0e14]">
      <div class="flex flex-col lg:flex-row flex-1 overflow-hidden">
        <!-- Sidebar Navigation -->
        <aside class="w-full lg:w-80 border-b lg:border-b-0 lg:border-r border-white/5 p-4 lg:p-10 flex flex-col flex-none bg-[#161a23] overflow-y-auto custom-scrollbar">
          
          <!-- Identity Header -->
          <div v-if="identity" class="hidden lg:flex flex-col items-center text-center mb-10 pt-4">
              <div v-if="identity.icon" class="w-20 h-20 rounded-[1.5rem] bg-primary/10 flex items-center justify-center text-primary mb-6 border border-primary/20 shadow-2xl relative">
                  <component v-if="identity.iconType !== 'svg'" :is="identity.icon" class="w-10 h-10" :class="identity.iconClass" />
                  <div v-else v-html="identity.icon" class="w-10 h-10 flex items-center justify-center" :class="identity.iconClass"></div>
                  <div class="absolute -bottom-1 -right-1 w-6 h-6 rounded-full bg-primary flex items-center justify-center text-white text-[9px] font-black border-2 border-[#161a23]">ID</div>
              </div>

              <h4 class="text-lg font-black text-white uppercase tracking-tighter mb-1 leading-tight px-4 line-clamp-3">{{ identity.name }}</h4>
              
              <div v-if="identity.id" class="flex items-center gap-2 group/id cursor-pointer select-all opacity-60 hover:opacity-100 transition-opacity" @click="copyId(identity.id)">
                  <p class="text-[9px] text-slate-500 font-mono tracking-widest uppercase">{{ identity.id.substring(0,20) }}{{ identity.id.length > 20 ? '...' : '' }}</p>
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 text-slate-700" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
              </div>
              <p v-else-if="identity.subtitle" class="text-[10px] text-slate-500 font-bold uppercase tracking-widest opacity-60">{{ identity.subtitle }}</p>
          </div>

          <!-- Tabs -->
          <div v-if="tabs && tabs.length" class="flex flex-row lg:flex-col gap-2 overflow-x-auto lg:overflow-visible custom-scrollbar-hide flex-1 pb-2 lg:pb-0 w-full max-w-[calc(100vw-60px)] lg:max-w-none">
            <button 
              v-for="tab in tabs" 
              :key="tab.id"
              @click="emit('update:activeTab', tab.id)"
              :class="['tab-button-premium whitespace-nowrap lg:whitespace-normal', { active: activeTab === tab.id }]"
            >
              <component v-if="tab.icon" :is="tab.icon" class="w-4 h-4 flex-none" />
              <span class="text-[10px]">{{ tab.label }}</span>
            </button>
          </div>

          <!-- Sidebar Bottom Box -->
          <div class="mt-auto hidden lg:block">
            <slot name="sidebar-bottom"></slot>
          </div>
        </aside>

        <!-- Main Content Area -->
        <main class="flex-1 p-6 lg:p-10 overflow-y-auto custom-scrollbar bg-[#161a23]/30 min-h-0 relative">
          <slot></slot>
        </main>
      </div>

      <!-- Footer -->
      <footer class="p-4 lg:p-8 bg-[#0b0e14] border-t border-white/5 flex flex-col sm:flex-row items-center justify-between gap-4 lg:gap-6 flex-none">
          <div class="flex items-center gap-4 w-full sm:w-auto justify-center sm:justify-start">
              <slot name="footer-start"></slot>
          </div>
          <div class="flex items-center gap-3 w-full sm:w-auto mt-4 sm:mt-0">
              <slot name="footer-actions">
                  <button @click="emit('cancel')" class="flex-1 sm:flex-none btn-premium btn-premium-ghost px-6 lg:px-10 h-10 lg:h-14 text-[10px] lg:text-xs min-w-[100px]">
                    {{ discardText || 'Discard' }}
                  </button>
                  <button @click="emit('save')" class="flex-1 sm:flex-none btn-premium btn-premium-primary px-8 lg:px-16 h-10 lg:h-14 text-[10px] lg:text-xs min-w-[140px]" :disabled="loading">
                    <span v-if="loading" class="loading loading-spinner loading-xs mr-2"></span>
                    <CheckCircle2 v-else class="w-4 h-4 mr-2" />
                    {{ saveText || 'Apply Settings' }}
                  </button>
              </slot>
          </div>
      </footer>
    </div>
  </AppModal>
</template>

<style scoped>
/* Ensure the sidebar header text doesn't overflow */
h4 {
    word-break: break-all;
}
</style>
