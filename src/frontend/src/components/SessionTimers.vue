<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
    timeout: number | null
    warning: number | null
    isOverride?: boolean
    inheritedTimeout?: number
    inheritedWarning?: number
}>()

const emit = defineEmits<{
    (e: 'update:timeout', value: number | null): void
    (e: 'update:warning', value: number | null): void
}>()

// Active values (what is actually applied)
const activeTimeout = computed(() => {
    if (props.timeout && props.timeout > 0) return props.timeout
    if (props.isOverride && props.inheritedTimeout) return props.inheritedTimeout
    return 4 // Fallback
})

const activeWarning = computed(() => {
    if (props.warning && props.warning > 0) return props.warning
    if (props.isOverride) {
        if (props.inheritedWarning) return props.inheritedWarning
    }
    // Auto-calc fallback (75%)
    let val = Math.floor(activeTimeout.value * 0.75)
    return Math.max(1, val)
})

const isCustomized = computed({
    get: () => !!(props.timeout && props.timeout > 0),
    set: (val: boolean) => {
        if (val) {
            // Enabling customization: start with inherited values or defaults
            emit('update:timeout', activeTimeout.value)
            emit('update:warning', activeWarning.value)
        } else {
            // Disabling: revert to null (inherit)
            emit('update:timeout', null)
            emit('update:warning', null)
        }
    }
})

// Proxy for the Slider UI (Always returns a number for the slider to render)
const sliderTimeout = computed({
    get: () => activeTimeout.value,
    set: (val: number) => {
        if (isCustomized.value) {
            emit('update:timeout', val)
            
            // Sync Warning
            const newMin = Math.floor(val * 0.8)
            const newMax = val - 1
            const currentWarning = props.warning || 0
             
             if (currentWarning < newMin) emit('update:warning', newMin)
             else if (currentWarning > newMax) emit('update:warning', newMax)
        }
    }
})

const sliderWarning = computed({
    get: () => activeWarning.value,
    set: (val: number) => {
        if (isCustomized.value) emit('update:warning', val)
    }
})


// Computed bounds
const minTimeout = computed(() => 4) // Always enforce min 4 for validity
const safeZoneWidth = computed(() => ((activeWarning.value / activeTimeout.value) * 100) + '%')

const warningPercentage = computed(() => (100 - ((activeWarning.value / activeTimeout.value) * 100)).toFixed(0))
const remainingMin = computed(() => activeTimeout.value - activeWarning.value)

const minWarning = computed(() => Math.floor(activeTimeout.value * 0.8))
const maxWarning = computed(() => activeTimeout.value - 1)
</script>

<template>
    <div class="p-8 bg-black/20 rounded-2xl border border-white/5 space-y-8">
        <!-- Visual Representation -->
        <div class="relative h-12 w-full bg-slate-800/50 rounded-xl overflow-hidden flex items-center group">
             <!-- Safe Zone -->
            <div class="h-full transition-all duration-300 flex items-center justify-center relative border-r border-black/20" 
                 :class="!isCustomized && isOverride ? 'bg-slate-600/20' : 'bg-primary/20'"
                 :style="{ width: safeZoneWidth }">
                <span v-if="isCustomized || !isOverride" class="text-xs font-black uppercase tracking-widest z-10 whitespace-nowrap px-2"
                      :class="!isCustomized && isOverride ? 'text-slate-500' : 'text-primary'">Safe Zone</span>
            </div>
            <!-- Warning Zone -->
            <div class="h-full transition-all duration-300 flex items-center justify-center relative flex-1"
                 :class="!isCustomized && isOverride ? 'bg-slate-600/10' : 'bg-amber-500/20'">
                    <span v-if="isCustomized || !isOverride" class="text-xs font-black uppercase tracking-widest z-10 whitespace-nowrap px-2"
                          :class="!isCustomized && isOverride ? 'text-slate-600' : 'text-amber-500'">Warning Zone</span>
                    
                    <div v-if="isCustomized || !isOverride" class="absolute inset-0 bg-amber-500/10 animate-pulse"></div>
            </div>
            
            <div class="absolute inset-0 flex justify-between px-4 pointer-events-none">
                <div class="h-full w-px bg-white/5"></div>
                <div class="h-full w-px bg-white/5"></div>
                <div class="h-full w-px bg-white/5"></div>
                <div class="h-full w-px bg-white/5"></div>
            </div>

            <div v-if="!isCustomized && isOverride" class="absolute inset-0 flex items-center justify-center pointer-events-none">
                 <span class="text-xs font-black text-white/20 uppercase tracking-[0.5em]">Active Channel Configuration</span>
            </div>
        </div>

        <!-- Customization Toggle (Only in Override Mode) -->
        <div v-if="isOverride" class="flex items-center justify-between pb-4 border-b border-white/5">
            <div>
                <h5 class="text-xs font-bold text-white uppercase tracking-widest">Configuration Source</h5>
                <p class="text-xs text-slate-500 font-medium uppercase mt-1">
                    {{ isCustomized ? 'Custom Override Active' : 'Inheriting from Channel Defaults' }}
                </p>
            </div>
            <div class="flex items-center gap-3">
                 <span class="text-xs font-bold uppercase tracking-widest" :class="!isCustomized ? 'text-primary' : 'text-slate-600'">Inherit</span>
                 <input type="checkbox" v-model="isCustomized" class="toggle toggle-primary toggle-sm" />
                 <span class="text-xs font-bold uppercase tracking-widest" :class="isCustomized ? 'text-primary' : 'text-slate-600'">Customize</span>
            </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-8" :class="{ 'opacity-60 grayscale pointer-events-none select-none': isOverride && !isCustomized }">
            <!-- Session Duration Slider -->
            <div class="form-control space-y-3">
                <div class="flex justify-between items-end">
                    <label class="label-premium flex items-center gap-2">
                        Max Session Duration
                    </label>
                    <span class="text-xl font-mono font-black transition-colors text-white">
                        {{ sliderTimeout }} 
                        <span class="text-xs font-bold uppercase text-slate-500">min</span>
                    </span>
                </div>
                <input 
                    type="range" 
                    v-model.number="sliderTimeout" 
                    :min="minTimeout" 
                    max="60" 
                    class="range range-xs" 
                    :class="!isCustomized && isOverride ? 'range-ghost' : 'range-primary'"
                />
                 <div class="flex justify-between text-xs font-bold text-slate-600 uppercase tracking-widest px-1">
                    <span>{{ minTimeout }} min</span>
                    <span>60 min</span>
                </div>
            </div>

            <!-- Warning Time Slider -->
            <div class="form-control space-y-3">
                <div class="flex justify-between items-end">
                     <label class="label-premium flex items-center gap-2">
                        Warning Minute
                    </label>
                    <span class="text-xl font-mono font-black transition-colors" :class="!isCustomized && isOverride ? 'text-slate-400' : 'text-amber-500'">
                        {{ sliderWarning }} 
                         <span class="text-xs font-bold uppercase text-slate-500">min</span>
                    </span>
                </div>
                
                <div class="flex justify-between text-xs font-bold uppercase tracking-widest mb-1">
                    <span class="text-slate-500">Trigger: Last {{ warningPercentage }}%</span> 
                    <span class="text-amber-500">{{ remainingMin }} min remaining</span>
                </div>

                <input 
                    type="range" 
                    v-model.number="sliderWarning" 
                    :min="minWarning" 
                    :max="maxWarning" 
                    class="range range-warning range-xs" 
                     :class="!isCustomized && isOverride ? 'range-ghost' : 'range-warning'"
                />
                 <div class="flex justify-between text-xs font-bold text-slate-600 uppercase tracking-widest px-1">
                    <span>Earliest ({{ minWarning }})</span>
                    <span>Latest ({{ maxWarning }})</span>
                </div>
                 <p class="text-xs text-amber-900/60 font-bold uppercase mt-2 tracking-widest pl-1 border-l-2 border-amber-500/20 pl-2">
                    Trigger range restricted to last 20% of session.
                </p>
            </div>
        </div>
    </div>
</template>
