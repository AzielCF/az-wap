<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { 
    Plus, Building2, Users2, ArrowLeft, 
    Trash2, Settings2, Unlink,
    UserPlus, Contact,
    Smartphone, X, ChevronRight, ShieldCheck,
    AlertTriangle
} from 'lucide-vue-next'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import { useApi } from '@/composables/useApi'

const props = defineProps<{
    clientId: string
    allowedBots: string[] // List of IDs whitelisted for the client
    bots: any[]          // Complete list of bot objects
    clientChannels: any[]
}>()

const api = useApi()

// State
const workspaces = ref<any[]>([])
const loading = ref(true)
const selectedWs = ref<any>(null)
const wsChannels = ref<any[]>([])
const wsGuests = ref<any[]>([])
const loadingDetails = ref(false)
const revocationMap = ref<Record<string, boolean>>({})
const validatingLid = ref(false)
const waInput = ref('')

const isLidVerified = computed(() => {
    return guestForm.value.platform_identifiers.whatsapp?.includes('@lid')
})

// Forms
const showWsForm = ref(false)
const editingWs = ref<any>(null)
const wsForm = ref({ name: '', description: '' })

const showGuestForm = ref(false)
const editingGuest = ref<any>(null)

function openGuestForm(guest: any = null) {
    formError.value = null
    editingGuest.value = guest
    if (guest) {
        guestForm.value = JSON.parse(JSON.stringify(guest))
        waInput.value = guest.platform_identifiers.whatsapp_number || guest.platform_identifiers.whatsapp || ''
    } else {
        guestForm.value = { 
            name: '', 
            bot_id: '',
            bot_template_id: '', 
            platform_identifiers: { whatsapp: '', whatsapp_number: '' } 
        }
        waInput.value = ''
    }
    originalGuestState.value = JSON.stringify(guestForm.value)
    showGuestForm.value = true
}
const guestForm = ref({ 
    name: '', 
    bot_id: '',
    bot_template_id: '', 
    platform_identifiers: { whatsapp: '', whatsapp_number: '' } as Record<string, string>
})
const originalGuestState = ref('')

const hasGuestChanges = computed(() => {
    return JSON.stringify(guestForm.value) !== originalGuestState.value
})

const showChannelPicker = ref(false)

const confirmState = ref({
    show: false,
    title: '',
    message: '',
    type: 'danger' as 'danger' | 'warning' | 'info',
    action: () => {}
})

const formError = ref<string | null>(null)

function showConfirm(title: string, message: string, type: 'danger' | 'warning' | 'info', action: () => void) {
    // Reset state first to ensure no leaks
    confirmState.value.show = false
    confirmState.value = { show: true, title, message, type, action }
}

const availableChannelsToLink = computed(() => {
    return props.clientChannels.filter(c => !wsChannels.value.find(wc => wc.id === c.id))
})

const hasWhatsAppChannel = computed(() => {
    return wsChannels.value.some(ch => ch.type === 'whatsapp')
})

const hasAnyChannel = computed(() => {
    return wsChannels.value.length > 0
})

// Bot Helpers
const botMap = computed(() => {
    const m: Record<string, any> = {}
    props.bots.forEach(b => {
        m[b.id] = b
    })
    return m
})

const availableBots = computed(() => {
    // Si allowedBots está vacío, mostramos todos (acceso global)
    if (!props.allowedBots || props.allowedBots.length === 0) return props.bots
    const allowed = props.bots.filter(b => props.allowedBots.includes(b.id))

    // Si el guest ya tiene un bot asignado pero el cliente actual perdió el acceso
    if (guestForm.value.bot_id && !props.allowedBots.includes(guestForm.value.bot_id)) {
        const revokedBot = botMap.value[guestForm.value.bot_id]
        if (revokedBot) {
            allowed.push({
                ...revokedBot,
                name: `(Revoked) ${revokedBot.name || revokedBot.display_name || revokedBot.id}`,
                display_name: `(Revoked) ${revokedBot.name || revokedBot.display_name || revokedBot.id}`
            })
        }
    }
    return allowed
})

const botVariants = computed(() => {
    if (!guestForm.value.bot_id) return []
    const bot = botMap.value[guestForm.value.bot_id]
    if (!bot || !bot.variants) return []
    return Object.entries(bot.variants).map(([id, v]: [string, any]) => ({
        id,
        ...v
    }))
})

function getBotDisplayName(id: string) {
    const bot = botMap.value[id]
    return bot ? bot.name : id
}

function isRevoked(guest: any) {
    if (!guest.bot_id) return false
    if (!props.allowedBots || props.allowedBots.length === 0) return false
    return !props.allowedBots.includes(guest.bot_id)
}

async function checkRevocations() {
    workspaces.value.forEach(async (ws) => {
        try {
            const guests = await api.get(`/clients/${props.clientId}/workspaces/${ws.id}/guests`) || []
            revocationMap.value[ws.id] = guests.some((g: any) => isRevoked(g))
        } catch (e) {
            revocationMap.value[ws.id] = false
        }
    })
}

function getTemplateName(guest: any) {
    if (!guest.bot_id && !guest.bot_template_id) return 'Inherited from Workspace'
    
    let botName = 'Unknown Bot'
    let templateName = guest.bot_template_id || 'Standard'

    if (guest.bot_id && botMap.value[guest.bot_id]) {
        const bot = botMap.value[guest.bot_id]
        botName = bot.name || bot.display_name || bot.id

        if (guest.bot_template_id && bot.variants && bot.variants[guest.bot_template_id]) {
            const variant = bot.variants[guest.bot_template_id]
            templateName = variant.name || variant.display_name || guest.bot_template_id
        }
    } else if (guest.bot_id) {
        botName = guest.bot_id
    }

    return `${botName} : ${templateName}`
}

function getDisplayNumber(guest: any) {
    if(guest.platform_identifiers?.whatsapp_number) {
        return '+' + guest.platform_identifiers.whatsapp_number
    }
    const whatsappIdentifier = guest.platform_identifiers?.whatsapp
    if(!whatsappIdentifier) return ''
    // Fallback to numeric part
    if(whatsappIdentifier.includes('@lid') || whatsappIdentifier.includes('@s.whatsapp.net')) {
        return '+' + whatsappIdentifier.split('@')[0]
    }
    return '+' + whatsappIdentifier
}

// Acciones API
async function loadWorkspaces() {
    loading.value = true
    try {
        workspaces.value = await api.get(`/clients/${props.clientId}/workspaces`) || []
        checkRevocations()
    } finally {
        loading.value = false
    }
}

async function selectWs(ws: any) {
    selectedWs.value = ws
    loadingDetails.value = true
    try {
        const [channels, guests] = await Promise.all([
            api.get(`/clients/${props.clientId}/workspaces/${ws.id}/channels`),
            api.get(`/clients/${props.clientId}/workspaces/${ws.id}/guests`)
        ])
        wsChannels.value = channels || []
        wsGuests.value = guests || []
    } finally {
        loadingDetails.value = false
    }
}

async function saveWs() {
    formError.value = null
    try {
        if (editingWs.value) {
            await api.put(`/clients/${props.clientId}/workspaces/${editingWs.value.id}`, wsForm.value)
        } else {
            await api.post(`/clients/${props.clientId}/workspaces`, wsForm.value)
        }
        await loadWorkspaces()
        showWsForm.value = false
    } catch (err: any) {
        formError.value = err.response?.data?.error || err.message || 'Unknown error'
        console.error(err)
    }
}

async function deleteWs(id: string) {
    showConfirm('Delete Workspace', 'Are you sure? All guests will lose access.', 'danger', async () => {
        try {
            await api.delete(`/clients/${props.clientId}/workspaces/${id}`)
            await loadWorkspaces()
            if (selectedWs.value?.id === id) selectedWs.value = null
        } catch (err) {
            console.error(err)
        }
    })
}

async function linkChannel(channelId: string) {
    try {
        await api.post(`/clients/${props.clientId}/workspaces/${selectedWs.value.id}/channels/${channelId}`, {})
        await selectWs(selectedWs.value)
        showChannelPicker.value = false
    } catch (err) {
        console.error(err)
    }
}

async function unlinkChannel(channelId: string) {
    showConfirm('Unlink Channel', 'Are you sure you want to unlink this channel from the workspace?', 'warning', async () => {
        try {
            await api.delete(`/clients/${props.clientId}/workspaces/${selectedWs.value.id}/channels/${channelId}`)
            await selectWs(selectedWs.value)
        } catch (err) {
            console.error(err)
        }
    })
}

async function saveGuest() {
    formError.value = null
    try {
        if (editingGuest.value) {
            await api.put(`/clients/${props.clientId}/workspaces/${selectedWs.value.id}/guests/${editingGuest.value.id}`, guestForm.value)
        } else {
            await api.post(`/clients/${props.clientId}/workspaces/${selectedWs.value.id}/guests`, guestForm.value)
        }
        await selectWs(selectedWs.value)
        showGuestForm.value = false
    } catch (err: any) {
        formError.value = err.response?.data?.error || err.message || 'Unknown error'
        console.error(err)
    }
}

async function deleteGuest(id: string) {
    showConfirm('Delete Person', 'Are you sure? This person will lose all access to configured bots.', 'danger', async () => {
        try {
            await api.delete(`/clients/${props.clientId}/workspaces/${selectedWs.value.id}/guests/${id}`)
            await selectWs(selectedWs.value)
        } catch (err) {
            console.error(err)
        }
    })
}

async function resolveLid() {
    const raw = waInput.value
    if (!raw) return
    
    // If it's already a full JID, don't re-resolve
    if (raw.includes('@')) return

    const waChannel = wsChannels.value.find(ch => ch.type === 'whatsapp' && (ch.status === 'connected' || ch.enabled))
    if (!waChannel) {
        console.warn('No active WA channel to resolve LID')
        return
    }

    validatingLid.value = true
    try {
        const phoneNumber = raw.replace(/[^\d]/g, '')
        // We use the first connected WA channel in the workspace to resolve identity
        const res = await api.get(`/workspaces/${selectedWs.value.id}/channels/${waChannel.id}/resolve-identity?identity=${phoneNumber}`) as any
        if (res.resolved_identity) {
            guestForm.value.platform_identifiers.whatsapp = res.resolved_identity
            guestForm.value.platform_identifiers.whatsapp_number = phoneNumber
            if (!guestForm.value.name && res.name) {
                guestForm.value.name = res.name
            }
        }
    } catch (err) {
        console.error('Identity resolution failed', err)
    } finally {
        validatingLid.value = false
    }
}

onMounted(loadWorkspaces)
</script>

<template>
    <div class="h-full flex flex-col overflow-hidden">
        <!-- Compact Header -->
        <header class="flex items-center justify-between p-6 border-b border-white/5">
            <div class="flex items-center gap-4">
                <button v-if="selectedWs" @click="selectedWs = null" 
                        class="btn btn-ghost btn-sm btn-circle bg-white/5">
                    <ArrowLeft class="w-4 h-4" />
                </button>
                <div>
                  <h3 class="text-lg font-black text-white uppercase tracking-tight flex items-center gap-2">
                    <Building2 class="w-5 h-5 text-primary/60" />
                    {{ selectedWs ? selectedWs.name : 'Workspaces' }}
                  </h3>
                  <p class="text-xs font-bold text-slate-500 uppercase tracking-widest mt-0.5">
                    {{ selectedWs ? 'Configuring unit access' : 'Manage organizational units' }}
                  </p>
                </div>
            </div>
            
            <button v-if="!selectedWs" @click="showWsForm = true; editingWs = null; wsForm = { name: '', description: '' }" 
                    class="btn btn-sm btn-primary rounded-xl px-4 font-black uppercase tracking-tight h-10">
                <Plus class="w-4 h-4 mr-2" /> New Workspace
            </button>
        </header>

        <!-- Main Workspace List -->
        <div v-if="loading" class="flex-1 flex items-center justify-center">
            <span class="loading loading-spinner text-primary"></span>
        </div>

        <div v-else-if="!selectedWs" class="flex-1 overflow-y-auto p-6 space-y-3">
            <div v-if="workspaces.length === 0" class="py-12 text-center border border-dashed border-white/5 rounded-3xl">
                <p class="text-xs font-black text-slate-600 uppercase tracking-[0.2em]">No workspaces defined</p>
            </div>
            
            <div v-for="ws in workspaces" :key="ws.id" 
                 class="group p-5 bg-white/[0.02] border border-white/5 rounded-2xl flex items-center justify-between hover:border-primary/20 hover:bg-white/[0.04] transition-all cursor-pointer"
                 @click="selectWs(ws)">
                <div class="flex items-center gap-4">
                    <div class="relative">
                        <div class="w-12 h-12 rounded-xl bg-black border border-white/5 flex items-center justify-center text-primary/40 group-hover:text-primary transition-colors">
                            <Building2 class="w-6 h-6" />
                        </div>
                        <div v-if="revocationMap[ws.id]" 
                             class="absolute -top-1 -right-1 w-5 h-5 bg-[#161a23] rounded-full flex items-center justify-center border border-white/5">
                            <AlertTriangle class="w-3 h-3 text-yellow-500 animate-pulse" />
                        </div>
                    </div>
                    <div>
                        <div class="flex items-center gap-2">
                            <h4 class="text-sm font-black text-white uppercase tracking-tighter">{{ ws.name }}</h4>
                            <span v-if="revocationMap[ws.id]" class="text-[8px] font-black bg-yellow-500/10 text-yellow-500 px-1.5 py-0.5 rounded border border-yellow-500/20 uppercase italic">
                                Action Required
                            </span>
                        </div>
                        <p class="text-xs text-slate-500 font-bold uppercase truncate max-w-[300px]">{{ ws.description || 'No description' }}</p>
                    </div>
                </div>
                <div class="flex items-center gap-2">
                    <button @click.stop="editingWs = ws; wsForm = { name: ws.name, description: ws.description }; showWsForm = true"
                            class="p-2 text-slate-500 hover:text-white transition-colors">
                        <Settings2 class="w-4 h-4" />
                    </button>
                    <button @click.stop="deleteWs(ws.id)"
                            class="p-2 text-slate-500 hover:text-red-400 transition-colors">
                        <Trash2 class="w-4 h-4" />
                    </button>
                    <ChevronRight class="w-5 h-5 text-slate-700 ml-2 group-hover:translate-x-1 group-hover:text-primary transition-all" />
                </div>
            </div>
        </div>

        <!-- Detail View -->
        <div v-else class="flex-1 overflow-hidden flex divide-x divide-white/5">
            <!-- Channels (Left) -->
            <div class="w-1/3 flex flex-col p-6 bg-black/10">
                <header class="flex items-center justify-between mb-6">
                    <h4 class="text-xs font-black text-white uppercase tracking-widest flex items-center gap-2">
                        <Smartphone class="w-3.5 h-3.5 text-primary" />
                        Channels
                    </h4>
                    <button @click="showChannelPicker = true" class="btn btn-xs btn-circle btn-primary">
                        <Plus class="w-3 h-3" />
                    </button>
                </header>

                <div class="flex-1 overflow-y-auto space-y-2 pr-2 custom-scrollbar">
                    <div v-for="ch in wsChannels" :key="ch.id" 
                         class="group p-3 bg-white/[0.03] border border-white/5 rounded-xl flex items-center justify-between hover:border-primary/20">
                        <div class="flex items-center gap-3">
                            <div class="w-8 h-8 rounded-lg bg-black flex items-center justify-center text-primary/60">
                                <Smartphone class="w-4 h-4" />
                            </div>
                            <h5 class="text-xs font-black text-white uppercase truncate max-w-[100px]">{{ ch.name }}</h5>
                        </div>
                        <button @click="unlinkChannel(ch.id)" class="p-2 text-slate-600 hover:text-red-500 transition-colors">
                            <Unlink class="w-3.5 h-3.5" />
                        </button>
                    </div>
                </div>
            </div>

            <!-- Guests (Right) -->
            <div class="flex-1 flex flex-col p-6">
                <header class="flex items-center justify-between mb-2">
                    <h4 class="text-xs font-black text-white uppercase tracking-widest flex items-center gap-2">
                        <Users2 class="w-3.5 h-3.5 text-primary" />
                        Guests / Personnel
                    </h4>
                        <button @click="openGuestForm()" class="btn btn-primary btn-sm rounded-xl px-4 shrink-0">
                            <UserPlus class="w-4 h-4 mr-2" />
                            Add
                        </button>
                </header>

                <!-- Workspace Level Alert -->
                <div v-if="wsGuests.some(g => isRevoked(g))" class="flex items-center gap-3 p-3 rounded-xl bg-yellow-500/10 border border-yellow-500/20 mb-4 animate-in fade-in duration-500">
                    <AlertTriangle class="w-4 h-4 text-yellow-500 shrink-0" />
                    <p class="text-[9px] font-bold text-yellow-500/80 uppercase tracking-widest leading-relaxed">
                        Detected guests with <span class="text-yellow-500">Revoked Bot Access</span>. They are currently inactive.
                    </p>
                </div>

                <div class="flex-1 overflow-y-auto space-y-2 pr-2 custom-scrollbar">
                    <div v-for="guest in wsGuests" :key="guest.id" 
                         class="group flex items-center gap-4 p-4 bg-white/[0.02] border border-white/5 rounded-2xl hover:border-primary/10">
                        <div class="w-11 h-11 rounded-xl flex items-center justify-center transition-colors"
                             :class="isRevoked(guest) ? 'bg-yellow-500/10 text-yellow-500' : 'bg-black text-slate-500 group-hover:text-primary'">
                            <AlertTriangle v-if="isRevoked(guest)" class="w-5 h-5" />
                            <Contact v-else class="w-5 h-5" />
                        </div>
                        <div class="flex-1">
                            <div class="flex items-center gap-2">
                                <h5 class="text-xs font-black text-white uppercase truncate">{{ guest.name }}</h5>
                                <div v-if="isRevoked(guest)" class="flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-yellow-500/10 text-yellow-500 border border-yellow-500/20">
                                    <AlertTriangle class="w-2.5 h-2.5" />
                                    <span class="text-[9px] font-black uppercase italic">Revoked Access</span>
                                </div>
                            </div>
                            <div class="flex items-center gap-2 mt-0.5">
                                <span class="text-xs font-bold uppercase tracking-widest" :class="isRevoked(guest) ? 'text-yellow-500/60' : 'text-slate-500'">
                                    {{ getTemplateName(guest) }}
                                </span>
                                <span v-if="guest.platform_identifiers?.whatsapp" class="text-xs px-1.5 py-0.5 rounded-md bg-green-500/10 text-green-500 font-mono italic">
                                    {{ getDisplayNumber(guest) }}
                                </span>
                            </div>
                        </div>
                        <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                            <button @click="openGuestForm(guest)" class="w-8 h-8 rounded-lg bg-white/5 flex items-center justify-center hover:bg-primary/20 hover:text-primary transition-all">
                                <Settings2 class="w-4 h-4" />
                            </button>
                            <button @click="deleteGuest(guest.id)" class="p-2 text-slate-500 hover:text-red-500">
                                <Trash2 class="w-3.5 h-3.5" />
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Overlays -->
        <div v-if="showWsForm" class="absolute inset-0 z-10 bg-[#0b0e14]/95 flex items-center justify-center p-8 animate-in fade-in duration-200">
            <div class="w-full max-w-md bg-[#161a23] border border-white/5 rounded-3xl p-8 shadow-2xl">
                <h4 class="text-lg font-black text-white uppercase tracking-tight mb-8 italic">Workspace Settings</h4>
                <div class="space-y-6 mb-8">
                    <div class="form-control">
                        <label class="label-premium">Display Name</label>
                        <input v-model="wsForm.name" type="text" class="input-premium h-12 w-full" />
                    </div>
                    <div class="form-control">
                        <label class="label-premium">Description</label>
                        <textarea v-model="wsForm.description" class="input-premium w-full p-4 min-h-[100px] resize-none"></textarea>
                    </div>
                </div>
                <div class="flex gap-3">
                    <button @click="showWsForm = false" class="btn btn-ghost flex-1 rounded-xl">Cancel</button>
                    <button @click="saveWs" class="btn btn-primary flex-1 rounded-xl" :disabled="!wsForm.name">Save</button>
                </div>
            </div>
        </div>

        <div v-if="showChannelPicker" class="absolute inset-0 z-10 bg-[#0b0e14]/95 flex items-center justify-center p-8 animate-in zoom-in-95 duration-200">
            <div class="w-full max-w-sm bg-[#161a23] border border-white/5 rounded-3xl p-8 shadow-2xl">
                <h4 class="text-sm font-black text-white uppercase tracking-widest mb-6 flex items-center justify-between">
                    Link Channel
                    <button @click="showChannelPicker = false"><X class="w-4 h-4" /></button>
                </h4>
                <div class="space-y-2 max-h-[300px] overflow-y-auto pr-1">
                    <div v-for="ch in availableChannelsToLink" :key="ch.id" 
                         @click="linkChannel(ch.id)"
                         class="p-4 bg-white/[0.03] border border-white/5 rounded-2xl flex items-center gap-4 hover:border-primary transition-all cursor-pointer">
                        <div class="w-10 h-10 rounded-xl bg-black flex items-center justify-center text-primary/60">
                            <Smartphone class="w-5 h-5" />
                        </div>
                        <div class="flex-1 font-black text-xs text-white uppercase">{{ ch.name }}</div>
                    </div>
                </div>
            </div>
        </div>

        <div v-if="showGuestForm" class="absolute inset-0 z-10 bg-[#0b0e14]/95 flex items-center justify-center p-8 animate-in slide-in-from-bottom-5 duration-200">
            <div class="w-full max-w-lg bg-[#161a23] border border-white/5 rounded-3xl p-8 shadow-2xl">
                <h4 class="text-lg font-black text-white uppercase tracking-tight mb-8 italic">Guest Identity</h4>
                
                <!-- Revocation Warning inside Form -->
                <div v-if="guestForm.bot_id && props.allowedBots && props.allowedBots.length > 0 && !props.allowedBots.includes(guestForm.bot_id)" 
                     class="flex items-start gap-4 p-4 rounded-2xl bg-yellow-500/10 border border-yellow-500/20 mb-8 animate-in slide-in-from-top-2 duration-300">
                    <div class="w-10 h-10 rounded-xl bg-yellow-500/20 flex items-center justify-center shrink-0">
                        <AlertTriangle class="w-5 h-5 text-yellow-500" />
                    </div>
                    <div class="flex-1">
                        <h4 class="text-xs font-black text-yellow-500 uppercase tracking-widest italic">Revoked Bot Warning</h4>
                        <p class="text-xs text-yellow-500/70 mt-1 leading-normal font-medium">
                            Access to this bot was restricted for this client. The guest remains configured but the <span class="font-bold underline">backend will block</span> all interactions until you re-assign a valid bot.
                        </p>
                    </div>
                </div>
                
                <!-- NEW: Unified Error Banner -->
                <div v-if="formError" 
                     class="flex items-start gap-4 p-4 rounded-2xl bg-red-500/10 border border-red-500/20 mb-8 animate-in shake duration-300">
                    <div class="w-10 h-10 rounded-xl bg-red-500/20 flex items-center justify-center shrink-0">
                        <X class="w-5 h-5 text-red-500" />
                    </div>
                    <div class="flex-1">
                        <h4 class="text-xs font-black text-red-500 uppercase tracking-widest italic">Action Failed</h4>
                        <p class="text-xs text-red-500/70 mt-1 leading-normal font-medium">
                            {{ formError }}
                        </p>
                    </div>
                    <button @click="formError = null" class="opacity-40 hover:opacity-100 transition-opacity">
                        <X class="w-4 h-4 text-red-500" />
                    </button>
                </div>

                <div class="grid grid-cols-2 gap-6 mb-8">
                    <div class="form-control col-span-2">
                        <label class="label-premium">Full Name / Alias</label>
                        <input v-model="guestForm.name" type="text" class="input-premium h-12 w-full" />
                    </div>
                    <div class="form-control">
                        <label class="label-premium">Base Bot</label>
                        <select v-model="guestForm.bot_id" class="input-premium h-12 w-full">
                            <option value="">(Inherit)</option>
                            <option v-for="bot in availableBots" :key="bot.id" :value="bot.id">
                                {{ bot.name || bot.display_name || bot.id }}
                            </option>
                        </select>
                    </div>
                    <div class="form-control">
                        <label class="label-premium">Template (Variant)</label>
                        <select v-model="guestForm.bot_template_id" class="input-premium h-12 w-full" :disabled="!guestForm.bot_id">
                            <option value="">(Standard)</option>
                            <option v-for="variant in botVariants" :key="variant.id" :value="variant.id">
                                {{ variant.name || variant.display_name || variant.id }}
                            </option>
                        </select>
                    </div>
                    <div class="form-control col-span-2" v-if="hasWhatsAppChannel">
                        <label class="label-premium">WhatsApp Number</label>
                        <div class="flex gap-2">
                            <input v-model="waInput" @input="guestForm.platform_identifiers.whatsapp = waInput; guestForm.platform_identifiers.whatsapp_number = waInput.replace(/[^\d]/g, '')" type="text" class="input-premium h-12 flex-1 font-mono" placeholder="+51..." @blur="resolveLid" />
                            <button @click="resolveLid" 
                                    :disabled="validatingLid || !waInput"
                                    class="btn btn-square h-12 w-12 rounded-lg transition-all"
                                    :class="isLidVerified ? 'bg-green-500/20 border-green-500/50 text-green-500 hover:bg-green-500/30' : 'btn-primary'">
                                <span v-if="validatingLid" class="loading loading-spinner loading-xs"></span>
                                <ShieldCheck v-else-if="isLidVerified" class="w-5 h-5" />
                                <Smartphone v-else class="w-5 h-5" />
                            </button>
                        </div>
                        
                        <!-- Visual Feedback (Green labels) -->
                        <div v-if="isLidVerified && guestForm.platform_identifiers.whatsapp !== waInput" class="mt-4 flex flex-wrap gap-x-6 gap-y-2 py-3 px-4 bg-green-500/5 border border-green-500/10 rounded-2xl animate-in fade-in slide-in-from-top-2 duration-300">
                            <div>
                                <span class="block text-[9px] font-black text-green-500/50 uppercase tracking-[0.2em] mb-1">Number</span>
                                <span class="text-xs font-black text-green-400 font-mono italic">{{ waInput }}</span>
                            </div>
                            <div class="flex items-center text-green-500/30">
                                <ChevronRight class="w-4 h-4" />
                            </div>
                            <div class="flex-1 min-w-0">
                                <span class="block text-[9px] font-black text-green-500/50 uppercase tracking-[0.2em] mb-1">LID Identity</span>
                                <span class="text-xs font-black text-green-500 font-mono truncate block">{{ guestForm.platform_identifiers.whatsapp }}</span>
                            </div>
                        </div>
                        
                        <p v-else class="text-xs text-slate-500 font-bold uppercase mt-3 tracking-tight flex items-center gap-1.5 pl-1">
                            <Plus class="w-3 h-3" /> Format: +51... (Auto-resolves to LID)
                        </p>
                    </div>
                </div>
                <div class="flex gap-3">
                    <button @click="showGuestForm = false" class="btn btn-ghost flex-1 rounded-xl">Discard</button>
                    <button @click="saveGuest" 
                            class="btn btn-primary flex-1 rounded-xl" 
                            :disabled="!guestForm.name || !hasAnyChannel || (hasWhatsAppChannel && !guestForm.platform_identifiers.whatsapp) || !hasGuestChanges">
                        Confirm
                    </button>
                </div>
            </div>
        </div>
    <!-- Confirmation Modal -->
    <ConfirmationDialog
        v-model="confirmState.show"
        :title="confirmState.title"
        :message="confirmState.message"
        :type="confirmState.type"
        @confirm="confirmState.action"
    />
    </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
    width: 3px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.05);
    border-radius: 10px;
}
</style>
