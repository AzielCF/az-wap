<script setup lang="ts">
import { ref, computed, h, nextTick } from 'vue'
import { Search, X, Bot, Globe, Layout, ChevronDown } from 'lucide-vue-next'

const props = defineProps<{
    modelValue: string | number
    items: Array<{ id: string | number; name: string; type?: string; [key: string]: any }>
    placeholder?: string
    label?: string
    iconType?: 'bot' | 'channel' | 'workspace' | 'default'
    resourceLabel?: string
    nullable?: boolean
    color?: string
}>()

const emit = defineEmits(['update:modelValue', 'select'])

const searchQuery = ref('')
const showDropdown = ref(false)

const filteredItems = computed(() => {
    if (!searchQuery.value) return props.items.slice(0, 100)
    const q = searchQuery.value.toLowerCase()
    return props.items.filter(item => 
        String(item.name).toLowerCase().includes(q) || 
        String(item.id).toLowerCase().includes(q)
    ).slice(0, 50)
})

const selectedItem = computed(() => {
    return props.items.find(i => String(i.id) === String(props.modelValue))
})

function selectItem(item: any) {
    emit('update:modelValue', item.id)
    emit('select', item)
    showDropdown.value = false
    searchQuery.value = ''
}

function clearSelection() {
    emit('update:modelValue', '')
    emit('select', null)
}

function focusInput() {
    showDropdown.value = true
    // Logic to auto-focus input if we had a ref could go here
}

// Icon rendering helpers
const getIcon = () => {
    if (props.iconType === 'bot') return Bot
    if (props.iconType === 'channel') return Globe
    if (props.iconType === 'workspace') return Layout
    return Bot // default
}

const activeColorClass = computed(() => {
    if (props.color === 'indigo') return 'text-indigo-400 group-hover:text-indigo-300'
    if (props.color === 'primary') return 'text-primary group-hover:text-primary-focus'
    return 'text-primary'
})

const activeBgClass = computed(() => {
    if (props.color === 'indigo') return 'bg-indigo-500/10'
    if (props.color === 'primary') return 'bg-primary/10'
    return 'bg-primary/10'
})

const activeBorderClass = computed(() => {
    if (props.color === 'indigo') return 'border-indigo-500/20'
    if (props.color === 'primary') return 'border-primary/20'
    return 'border-primary/20'
})

</script>

<template>
    <div class="form-control w-full">
        <label v-if="label" class="label-premium text-slate-400">{{ label }}</label>
        
        <!-- Selected State -->
        <div v-if="selectedItem" 
             class="flex items-center justify-between p-5 rounded-[2rem] mb-2 animate-in zoom-in-95 backdrop-blur-sm border"
             :class="[activeBgClass, activeBorderClass]">
            <div class="flex items-center gap-4">
                <div class="w-12 h-12 rounded-2xl flex items-center justify-center shadow-lg transition-colors"
                     :class="[props.color === 'indigo' ? 'bg-indigo-600 text-white shadow-indigo-500/20' : 'bg-primary text-white shadow-primary/20']">
                    <component :is="getIcon()" class="w-6 h-6" />
                </div>
                <div>
                    <div class="font-black text-white uppercase text-sm tracking-tight">{{ selectedItem.name }}</div>
                    <div class="text-[9px] font-mono uppercase" :class="[props.color === 'indigo' ? 'text-indigo-400' : 'text-primary/60']">
                        {{ resourceLabel || 'Selected' }}
                    </div>
                </div>
            </div>
            <button @click="clearSelection" class="p-3 text-slate-600 hover:text-white transition-colors">
                <X class="w-5 h-5" />
            </button>
        </div>

        <!-- Search Input State -->
        <div v-else class="relative group">
            <Search class="absolute left-5 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-600 z-10 transition-colors" 
                    :class="[showDropdown ? (props.color === 'indigo' ? 'text-indigo-400' : 'text-primary') : '']" />
            
            <input 
                v-model="searchQuery" 
                @focus="showDropdown = true"
                type="text" 
                :placeholder="placeholder || 'Search...'" 
                class="input-premium h-16 pl-14 w-full text-base font-bold relative z-0" 
            />

            <!-- Dropdown Results -->
            <div v-if="showDropdown" class="absolute z-50 top-full left-0 right-0 mt-3 bg-[#12161f] border border-white/10 rounded-[2rem] shadow-2xl p-3 max-h-64 overflow-y-auto backdrop-blur-xl animate-in fade-in slide-in-from-top-2 duration-200">
                
                <!-- Null option -->
                <button v-if="nullable" @click="clearSelection" class="w-full p-4 hover:bg-white/5 rounded-2xl text-left border border-white/5 mb-2 group transition-all">
                    <div class="text-[10px] font-black uppercase text-slate-500 group-hover:text-white transition-colors">
                        {{ nullable === true ? 'None / Reset Selection' : 'Reset Selection' }}
                    </div>
                </button>

                <!-- List Items -->
                <div v-if="filteredItems.length === 0" class="p-6 text-center">
                    <span class="text-xs text-slate-600 font-bold uppercase tracking-widest">No results found</span>
                </div>

                <button v-for="item in filteredItems" :key="item.id" 
                        @click="selectItem(item)"
                        class="w-full flex items-center gap-4 p-4 rounded-2xl transition-all text-left border border-transparent group mb-1"
                        :class="[props.color === 'indigo' ? 'hover:bg-indigo-500/10 hover:border-indigo-500/20' : 'hover:bg-primary/10 hover:border-primary/20']">
                    
                    <div class="w-10 h-10 rounded-xl bg-black/40 flex items-center justify-center text-slate-600 transition-colors"
                         :class="[props.color === 'indigo' ? 'group-hover:text-indigo-400' : 'group-hover:text-primary']">
                        <component :is="getIcon()" class="w-5 h-5" />
                    </div>
                    
                    <div class="flex-1 min-w-0">
                        <div class="text-xs font-black uppercase text-white tracking-wide truncate">{{ item.name }}</div>
                        <div class="text-[9px] font-mono text-slate-500 truncate uppercase mt-0.5">ID: {{ String(item.id).substring(0,8) }}</div>
                    </div>
                </button>
            </div>
            
            <!-- Backdrop to close dropdown -->
            <div v-if="showDropdown" @click="showDropdown = false" class="fixed inset-0 z-[-1]" style="z-index: -1;"></div>
        </div>
        <div v-if="showDropdown" @click="showDropdown = false" class="fixed inset-0 z-40 bg-black/0 cursor-default"></div> 
        <!-- Note: The z-index handling here is tricky for modals. I'll rely on click-outside logic or simpler z-index stacking. -->
    </div>
</template>
