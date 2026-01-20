<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '@/composables/useApi'
import AppModal from '@/components/AppModal.vue'
import { Users, Newspaper, RefreshCw, X, Search, ShieldCheck, User } from 'lucide-vue-next'

const props = defineProps<{
  channel: any
  workspaceId: string
}>()

const emit = defineEmits(['close'])

const api = useApi()
const activeTab = ref<'groups' | 'newsletters'>('groups')
const groups = ref<any[]>([])
const newsletters = ref<any[]>([])
const loading = ref(false)
const error = ref('')

async function fetchGroups() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get(`/instances/${props.channel.id}/groups`)
    groups.value = res.results || []
  } catch (err: any) {
    error.value = err.message || 'Failed to load groups'
  } finally {
    loading.value = false
  }
}

async function fetchNewsletters() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get(`/newsletter/list/${props.channel.id}`)
    newsletters.value = res.results || []
  } catch (err: any) {
    error.value = err.message || 'Failed to load newsletters'
  } finally {
    loading.value = false
  }
}

function handleTabChange(tab: 'groups' | 'newsletters') {
  activeTab.value = tab
  if (tab === 'groups' && groups.value.length === 0) fetchGroups()
  if (tab === 'newsletters' && newsletters.value.length === 0) fetchNewsletters()
}

function refresh() {
  if (activeTab.value === 'groups') fetchGroups()
  else fetchNewsletters()
}

onMounted(() => {
  fetchGroups()
})
</script>

<template>
  <AppModal :modelValue="true" @update:modelValue="emit('close')" title="Channel Inspector" maxWidth="max-w-4xl" noPadding noScroll>
    <div class="flex flex-col h-[80vh] bg-[#0b0e14]">
      <!-- Header / Tabs -->
      <div class="flex-none p-6 border-b border-white/5 bg-[#0b0e14] z-10">
         <div class="flex items-center justify-between mb-6">
             <div class="flex items-center gap-4">
                 <div class="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center text-primary ring-1 ring-white/5">
                     <Search class="w-6 h-6" />
                 </div>
                 <div>
                     <h3 class="text-xl font-black text-white uppercase tracking-tighter">Instance Inspector</h3>
                     <div class="text-[10px] text-slate-500 font-bold uppercase tracking-widest">{{ channel.name }}</div>
                 </div>
             </div>
             <button class="btn-premium btn-premium-ghost btn-premium-square" @click="emit('close')">
                 <X class="w-5 h-5" />
             </button>
         </div>

         <div class="flex gap-2 p-1 bg-black/40 rounded-xl border border-white/5 w-fit">
             <button 
                class="px-6 py-2.5 rounded-lg text-xs font-black uppercase tracking-widest transition-all flex items-center gap-2"
                :class="activeTab === 'groups' ? 'bg-primary text-black shadow-lg shadow-primary/20' : 'text-slate-500 hover:text-white hover:bg-white/5'"
                @click="handleTabChange('groups')"
             >
                <Users class="w-4 h-4" />
                Participating Groups
             </button>
             <button 
                class="px-6 py-2.5 rounded-lg text-xs font-black uppercase tracking-widest transition-all flex items-center gap-2"
                :class="activeTab === 'newsletters' ? 'bg-primary text-black shadow-lg shadow-primary/20' : 'text-slate-500 hover:text-white hover:bg-white/5'"
                @click="handleTabChange('newsletters')"
             >
                <Newspaper class="w-4 h-4" />
                Newsletters
             </button>
         </div>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto custom-scrollbar p-6 bg-[#0b0e1400]">
          
          <div v-if="loading" class="flex flex-col items-center justify-center py-20 gap-4 opacity-50">
              <span class="loading loading-ring loading-lg text-primary"></span>
              <span class="text-[10px] font-black uppercase tracking-widest text-slate-500">Scanning Network...</span>
          </div>

          <div v-else-if="error" class="p-8 border border-red-500/10 rounded-2xl bg-red-500/5 flex flex-col items-center gap-3 text-center">
              <ShieldCheck class="w-8 h-8 text-red-500 opacity-50" />
              <div class="text-red-500 font-bold text-sm">{{ error }}</div>
              <button class="btn-premium btn-premium-ghost btn-premium-sm mt-2" @click="refresh">Retry Scan</button>
          </div>

          <div v-else>
              <!-- Groups View -->
              <div v-if="activeTab === 'groups'" class="space-y-4">
                  <div class="flex items-center justify-between text-[10px] uppercase font-bold text-slate-500 tracking-widest px-2 mb-2">
                      <span>Found {{ groups.length }} Groups</span>
                      <button class="flex items-center gap-2 hover:text-white transition-colors" @click="refresh">
                          <RefreshCw class="w-3 h-3" /> Refresh
                      </button>
                  </div>
                  
                  <div v-if="groups.length === 0" class="py-20 text-center opacity-30">
                      <Users class="w-12 h-12 mx-auto mb-4 text-slate-500" />
                      <div class="text-xs font-black uppercase tracking-widest">No joined groups found</div>
                  </div>

                  <div v-for="g in groups" :key="g.JID" class="p-4 rounded-xl border border-white/5 bg-white/[0.02] hover:bg-white/[0.04] transition-colors flex items-center gap-4 group">
                      <div class="w-10 h-10 rounded-full bg-slate-800 flex items-center justify-center text-slate-400 font-black text-xs ring-1 ring-white/5">
                          {{ g.Name?.substring(0,2).toUpperCase() || 'GR' }}
                      </div>
                      <div class="flex-1 min-w-0">
                          <div class="flex items-center gap-2 mb-1">
                              <h4 class="text-sm font-bold text-white truncate">{{ g.Name }}</h4>
                              <span class="badge badge-sm badge-ghost text-[9px] font-black uppercase tracking-wider border-white/10" v-if="g.IsLocked">LOCKED</span>
                              <span class="badge badge-sm badge-ghost text-[9px] font-black uppercase tracking-wider border-white/10" v-if="g.IsAnnounce">ANNOUNCE</span>
                          </div>
                          <div class="flex items-center gap-4 text-[10px] text-slate-500 font-mono tracking-wide">
                              <span class="select-all cursor-copy hover:text-primary transition-colors">{{ g.JID }}</span>
                              <span v-if="g.Owner">Owner: {{ g.Owner.split('@')[0] }}</span>
                          </div>
                      </div>
                      <div class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">
                          {{ new Date(g.CreatedAt).toLocaleDateString() }}
                      </div>
                  </div>
              </div>

              <!-- Newsletters View -->
              <div v-if="activeTab === 'newsletters'" class="space-y-4">
                  <div class="flex items-center justify-between text-[10px] uppercase font-bold text-slate-500 tracking-widest px-2 mb-2">
                       <span>Found {{ newsletters.length }} Channels</span>
                       <button class="flex items-center gap-2 hover:text-white transition-colors" @click="refresh">
                          <RefreshCw class="w-3 h-3" /> Refresh
                      </button>
                  </div>

                  <div v-if="newsletters.length === 0" class="py-20 text-center opacity-30">
                      <Newspaper class="w-12 h-12 mx-auto mb-4 text-slate-500" />
                      <div class="text-xs font-black uppercase tracking-widest">No newsletters found</div>
                  </div>

                   <div v-for="n in newsletters" :key="n.id" class="p-4 rounded-xl border border-white/5 bg-white/[0.02] hover:bg-white/[0.04] transition-colors flex items-center gap-4 group">
                      <div class="w-10 h-10 rounded-full bg-slate-800 flex items-center justify-center text-slate-400 font-black text-xs ring-1 ring-white/5">
                           {{ n.name?.substring(0,2).toUpperCase() || 'NL' }}
                      </div>
                      <div class="flex-1 min-w-0">
                          <div class="flex items-center gap-2 mb-1">
                              <h4 class="text-sm font-bold text-white truncate">{{ n.name }}</h4>
                              <span class="badge badge-sm text-[9px] font-black uppercase tracking-wider border-transparent" :class="n.role === 'OWNER' || n.role === 'ADMIN' ? 'bg-primary/20 text-primary' : 'bg-slate-800 text-slate-400'">{{ n.role }}</span>
                          </div>
                           <div class="flex items-center gap-4 text-[10px] text-slate-500 font-mono tracking-wide">
                              <span class="select-all cursor-copy hover:text-primary transition-colors">{{ n.id }}</span>
                              <span>{{ n.subscribers }} Subs</span>
                          </div>
                      </div>
                       <div class="text-[10px] font-bold text-slate-600 uppercase tracking-widest">
                          {{ new Date(n.created_at).toLocaleDateString() }}
                      </div>
                  </div>
              </div>
          </div>
      </div>
    </div>
  </AppModal>
</template>
