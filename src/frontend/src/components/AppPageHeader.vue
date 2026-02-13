<script setup lang="ts">
defineProps<{
  title: string
}>()
</script>

<template>
  <header class="flex flex-col lg:flex-row lg:items-end justify-between gap-10 py-10 border-b border-white/5 mx-6 lg:mx-0">
    <div class="space-y-6 flex-1 w-full">
      <!-- Breadcrumb / Badge Area -->
      <div v-if="$slots.breadcrumb" class="flex items-center gap-3">
        <slot name="breadcrumb"></slot>
      </div>
      
      <!-- Main Title -->
      <h2 class="text-4xl lg:text-6xl font-black tracking-tighter text-white uppercase leading-none break-words">
        {{ title }}
      </h2>

      <!-- Optional Subtitle -->
      <div v-if="$slots.subtitle" class="text-sm text-slate-500 font-bold uppercase tracking-widest pl-1">
        <slot name="subtitle"></slot>
      </div>
    </div>
    
    <!-- Action Buttons -->
    <div v-if="$slots.actions" class="flex flex-col sm:flex-row gap-4 w-full lg:w-auto">
      <slot name="actions"></slot>
    </div>
  </header>
</template>
