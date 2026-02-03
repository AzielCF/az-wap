<script setup lang="ts">
import { computed } from 'vue'
import { History, Infinity as InfinityIcon } from 'lucide-vue-next'

const props = defineProps<{
    modelValue: number | null | undefined
    isOverride?: boolean // If true, allows "Inherit/Null" state. If false, 0 means default 10.
}>()

const emit = defineEmits<{
    (e: 'update:modelValue', value: number | null | undefined): void
}>()

// Internal Model Logic
// Backend meanings:
// Channel: 0 = Default (10), -1 = Unlimited, >0 = Limit
// Subscription (Override): null = Unlimited (Default for sub), >0 = Limit.
// Wait, the backend logic for subscription was: "If nil, use -1 (Unlimited)".
// So for Subscription:
//   Input: null -> UI should show "Unlimited (Default)".
//   Input: number -> UI should show Limit.
// For Channel:
//   Input: 0 -> UI should show "Default (10)".
//   Input: -1 -> UI should show "Unlimited".
//   Input: >0 -> UI should show Limit.

// Let's abstract this for the user:
// State A: Unlimited
// State B: Limited (Value)
// State C: Default/Inherit (Only relevant for Channel? No, for Channel 0 is default 10).

const mode = computed({
    get: () => {
        if (props.isOverride) {
            // Subscription Mode
            if (props.modelValue === null || props.modelValue === undefined) return 'unlimited' // Default for sub is unlimited
            if (props.modelValue > 0) return 'limited'
            return 'unlimited' 
        } else {
            // Channel Mode
            if (props.modelValue === -1) return 'unlimited'
            if (props.modelValue && props.modelValue > 0) return 'limited'
            return 'default' // 0 = default 10
        }
    },
    set: (v: string) => {
        if (props.isOverride) {
            if (v === 'unlimited') emit('update:modelValue', null) // Nil = unlimited
            if (v === 'limited') emit('update:modelValue', 10) // Start at 10
        } else {
            if (v === 'unlimited') emit('update:modelValue', -1)
            if (v === 'limited') emit('update:modelValue', 50)
            if (v === 'default') emit('update:modelValue', 0)
        }
    }
})

const limitValue = computed({
    get: () => {
        if (props.modelValue && props.modelValue > 0) return props.modelValue
        return 10 // Placeholder
    },
    set: (v: number) => {
        emit('update:modelValue', v)
    }
})

</script>

<template>
    <div class="p-6 bg-black/20 rounded-2xl border border-white/5 space-y-4">
        <div class="flex items-center gap-3 mb-2">
            <History class="w-4 h-4 text-purple-400" />
            <span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Conversation History Limit</span>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <!-- Mode Selection -->
            <div class="join w-full">
                <button 
                    v-if="!isOverride"
                    @click="mode = 'default'"
                    class="join-item btn btn-sm flex-1 font-bold border-white/5"
                    :class="mode === 'default' ? 'btn-primary text-white' : 'btn-ghost text-slate-500'"
                >
                    Default (10)
                </button>
                <button 
                    @click="mode = 'limited'"
                    class="join-item btn btn-sm flex-1 font-bold border-white/5"
                    :class="mode === 'limited' ? 'btn-primary text-white' : 'btn-ghost text-slate-500'"
                >
                    Custom Limit
                </button>
                 <button 
                    @click="mode = 'unlimited'"
                    class="join-item btn btn-sm flex-1 font-bold border-white/5"
                    :class="mode === 'unlimited' ? 'btn-primary text-white' : 'btn-ghost text-slate-500'"
                >
                    Unlimited
                    <InfinityIcon class="w-3 h-3 ml-1" />
                </button>
            </div>

            <!-- Value Input (if Limited) -->
            <div v-if="mode === 'limited'" class="space-y-2 animate-in fade-in slide-in-from-left-2">
                 <div class="flex items-center gap-3">
                    <input 
                        type="range" 
                        v-model.number="limitValue" 
                        min="2" 
                        max="100" 
                        class="range range-xs range-primary flex-1" 
                    />
                    <div class="w-16 text-center font-mono font-black text-xl text-white">
                        {{ limitValue }}
                    </div>
                 </div>
                 <p class="text-xs text-slate-500 font-bold uppercase tracking-widest text-center">Messages retained in context</p>
            </div>
            
            <div v-else-if="mode === 'unlimited'" class="flex items-center justify-center p-2 rounded-lg bg-green-500/10 border border-green-500/20 text-green-500 animate-in fade-in">
                <span class="text-xs font-black uppercase tracking-widest flex items-center gap-2">
                    <InfinityIcon class="w-4 h-4" />
                    Full Conversation Memory
                </span>
            </div>

             <div v-else-if="mode === 'default'" class="flex items-center justify-center p-2 rounded-lg bg-white/5 border border-white/5 text-slate-500 animate-in fade-in">
                <span class="text-xs font-black uppercase tracking-widest">
                    Standard Limit (10 Messages)
                </span>
            </div>
        </div>
    </div>
</template>
