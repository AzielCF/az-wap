<script setup lang="ts">
import { computed, ref } from 'vue'
import { Tag } from 'lucide-vue-next'

const props = defineProps<{
  modelValue: any
}>()

const emit = defineEmits(['update:modelValue'])

const client = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const newTag = ref('')

function addTag() {
  if (newTag.value && !client.value.tags.includes(newTag.value)) {
    client.value.tags.push(newTag.value)
    newTag.value = ''
  }
}

function removeTag(tag: string) {
  client.value.tags = client.value.tags.filter((t: string) => t !== tag)
}
</script>

<template>
  <div class="space-y-8 animate-in fade-in slide-in-from-right-4 duration-300">
    <header>
      <h3 class="text-xl font-black text-white uppercase tracking-tight">Contact Info</h3>
      <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Personal and communication data</p>
    </header>

    <div class="form-control">
      <label class="label-premium text-slate-400">Display Name / Alias</label>
      <input v-model="client.display_name" type="text" class="input-premium h-14 w-full text-lg font-black" placeholder="Contact Name" />
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
      <div class="form-control">
        <label class="label-premium text-slate-400">Primary Email</label>
        <input v-model="client.email" type="email" class="input-premium h-14 w-full text-sm" placeholder="contact@domain.com" />
      </div>
      <div class="form-control">
        <label class="label-premium text-slate-400">Universal Phone</label>
        <input v-model="client.phone" 
            type="tel" 
            class="input-premium h-14 w-full text-sm font-mono" 
            :class="{ 'opacity-50 cursor-not-allowed bg-black/20': client.platform_type === 'whatsapp' }"
            :readonly="client.platform_type === 'whatsapp'"
            placeholder="+XX XXX XXX XXX" />
      </div>
    </div>

    <div class="form-control">
      <label class="label-premium text-slate-400">Categorization Tags</label>
      <div class="flex gap-2 mb-4">
        <input v-model="newTag" type="text" class="input-premium h-12 flex-1 text-sm bg-black/40" placeholder="Add custom tag..." @keyup.enter="addTag" />
        <button @click="addTag" class="btn-premium btn-premium-ghost h-12 px-6">
          <Tag class="w-4 h-4" />
        </button>
      </div>
      <div class="flex flex-wrap gap-2 p-4 bg-black/40 rounded-2xl border border-white/5 min-h-[60px]">
        <span v-for="tag in client.tags" :key="tag" 
              class="px-4 py-2 text-xs font-black uppercase tracking-widest bg-primary/10 text-primary border border-primary/20 rounded-xl flex items-center gap-3">
          {{ tag }}
          <button @click="removeTag(tag)" class="hover:text-white transition-colors">&times;</button>
        </span>
        <p v-if="client.tags.length === 0" class="text-xs text-slate-700 font-bold uppercase items-center flex">No tags assigned / global pool</p>
      </div>
    </div>
  </div>
</template>
