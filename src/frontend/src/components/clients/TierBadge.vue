<script setup lang="ts">
import { computed } from 'vue'
import { Crown, Star, Shield, UserCircle } from 'lucide-vue-next'

const props = defineProps<{
  tier: string
  showIcon?: boolean
}>()

const tierConfig = computed(() => {
  const t = props.tier?.toLowerCase() || 'standard'
  switch(t) {
    case 'vip': 
      return { class: 'badge-tier-vip', icon: Crown, label: 'VIP' }
    case 'premium': 
      return { class: 'badge-tier-premium', icon: Star, label: 'Premium' }
    case 'enterprise': 
      return { class: 'badge-tier-enterprise', icon: Shield, label: 'Enterprise' }
    default: 
      return { class: 'badge-tier-standard', icon: UserCircle, label: 'Standard' }
  }
})
</script>

<template>
  <span class="badge-premium flex items-center gap-1.5" :class="tierConfig.class">
    <component v-if="showIcon" :is="tierConfig.icon" class="w-3.5 h-3.5" />
    {{ tierConfig.label }}
  </span>
</template>
