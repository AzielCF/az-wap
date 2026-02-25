<script setup lang="ts">
import AppPageHeader from '@/components/AppPageHeader.vue'
import { Plus, Settings, Cpu } from 'lucide-vue-next'

const emit = defineEmits<{
  (e: 'openSettings'): void
  (e: 'openAdd'): void
}>()
</script>

<template>
    <AppPageHeader title="Bots">
      <template #breadcrumb>
        <Cpu class="w-4 h-4 text-primary shrink-0" />
        <span class="text-xs font-black uppercase tracking-widest text-primary">Automation Logic</span>
        <span class="opacity-30 text-xs font-black text-slate-500">/</span>
        <span class="text-xs font-bold uppercase tracking-widest text-slate-500">Identity Blueprints</span>
      </template>

      <template #actions>
        <button class="btn-premium btn-premium-ghost px-8" @click="emit('openSettings')">
           <Settings class="w-4 h-4" />
           Engine Configuration
        </button>
        <button class="btn-premium btn-premium-primary px-16 h-14 w-full lg:w-auto" @click="emit('openAdd')">
             <Plus class="w-5 h-5 mr-2" />
             Compose Template
        </button>
      </template>
    </AppPageHeader>
</template>