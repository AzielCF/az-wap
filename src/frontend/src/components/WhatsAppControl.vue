<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import QRCode from 'qrcode'

const props = defineProps<{
  channel: any
  workspaceId: string
}>()

const api = useApi()
const status = ref({
  connected: false,
  loggedIn: false,
  isPaused: false,
  isHibernating: false,
  qr: null as string | null,
  loading: true,
  profilePhoto: null as string | null
})

let interval: any = null

const loginMethod = ref<'qr' | 'code'>('qr')
const phoneNumber = ref('')
const pairingCode = ref<string | null>(null)
const qrImage = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const uploadingPhoto = ref(false)

// Watch for QR code changes to render image
watch(() => status.value.qr, async (newVal) => {
    console.log("[QR DEBUG] Watcher triggered. Value:", newVal ? newVal.substring(0, 20) + "..." : "null")
    if (newVal && !newVal.startsWith('http')) {
        try {
            qrImage.value = await QRCode.toDataURL(newVal, { 
                width: 300, 
                margin: 2,
                color: {
                    dark: '#000000',
                    light: '#ffffff'
                }
            })
            console.log("[QR DEBUG] Image generated successfully")
        } catch (err) {
            console.error('Failed to generate QR', err)
        }
    } else if (newVal) {
        qrImage.value = newVal
    } else {
        qrImage.value = ''
    }
})

async function loginWithCode() {
  if (!phoneNumber.value) return
  status.value.loading = true
  pairingCode.value = null
  
  try {
    const res = await api.post(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/whatsapp/login-code`, {
        phone_number: phoneNumber.value
    })
    
    if (res.code) {
        pairingCode.value = res.code
    }
  } catch (err) {
    alert('Failed to get pairing code. Check if number is correct and try again.')
    console.error(err)
  } finally {
    status.value.loading = false
  }
}

async function fetchStatus(manual = false) {
  try {
    const query = manual ? '?resume=true' : ''
    const data = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/whatsapp/status${query}`)
    
    const wasLoggedIn = status.value.loggedIn
    status.value.connected = data.is_connected
    status.value.loggedIn = data.is_logged_in
    status.value.isHibernating = data.is_hibernating || false
    status.value.isPaused = data.is_paused || false
    status.value.loading = false

    // Fetch photo once if we just logged in or if it's missing
    if (status.value.loggedIn && (!wasLoggedIn || !status.value.profilePhoto)) {
        fetchProfilePhoto()
    }
  } catch (err) {
    console.error(err)
  }
}

async function fetchProfilePhoto() {
    try {
        const res = await api.get(`/instances/${props.channel.id}/profile/photo`)
        if (res.results && res.results.url) {
            status.value.profilePhoto = res.results.url
        }
    } catch (err) {
        // Fail silently
    }
}

async function updateProfilePhoto(event: Event) {
    const target = event.target as HTMLInputElement
    const file = target.files?.[0]
    if (!file) return

    uploadingPhoto.value = true
    const formData = new FormData()
    formData.append('avatar', file)

    try {
        await api.post(`/instances/${props.channel.id}/profile/photo`, formData)
        // Give it more time for the network to propagate and CDN to refresh
        setTimeout(fetchProfilePhoto, 5000)
    } catch (err) {
        alert('Failed to update profile photo')
    } finally {
        uploadingPhoto.value = false
        if (fileInput.value) fileInput.value.value = ''
    }
}

async function login() {
  status.value.loading = true
  try {
    const res = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/whatsapp/login`)
    if (res.results && res.results.qr_link) {
      status.value.qr = res.results.qr_link
    }
  } catch (err) {
    alert('Failed to initialize login')
  } finally {
    status.value.loading = false
  }
}

async function logout() {
  console.log('[LOGOUT] Button clicked!')
  try {
    console.log('[LOGOUT] Calling API...', `/workspaces/${props.workspaceId}/channels/${props.channel.id}/whatsapp/logout`)
    const result = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/whatsapp/logout`)
    console.log('[LOGOUT] API Response:', result)
    status.value.qr = null
    status.value.loggedIn = false
    await fetchStatus()
  } catch (err) {
    console.error('[LOGOUT] Error:', err)
    alert('Logout error: ' + (err as any)?.message)
  }
}

function refresh() {
    status.value.loading = true
    fetchStatus(true) // Manual sync triggers resume if hibernating
}

onMounted(() => {
  fetchStatus()
  interval = setInterval(fetchStatus, 4000)
})

onBeforeUnmount(() => {
  if (interval) clearInterval(interval)
})
</script>

<template>
  <div class="space-y-8 animate-in fade-in duration-500">
    <!-- Header Controls -->
    <div class="flex items-center justify-between border-b border-white/5 pb-6">
        <div class="space-y-1">
            <h4 class="text-xs font-black text-white uppercase tracking-widest">Connection Protocol</h4>
            <p class="text-xs text-slate-500 font-bold uppercase tracking-widest font-mono">NODE: {{ channel.id.substring(0,12) }}</p>
        </div>
        <div class="flex gap-3">
            <div v-if="status.isHibernating" class="badge-premium border-primary/20 text-primary animate-pulse">
                SLEEPING / HIBERNATING
            </div>
            <div v-if="status.isPaused && status.loggedIn" class="badge-premium badge-amber">
                MOTOR PAUSED
            </div>
            <div class="badge-premium" :class="status.connected ? 'badge-success' : 'badge-ghost opacity-40'">
                {{ status.connected ? 'NETWORK UP' : 'NETWORK DOWN' }}
            </div>
        </div>
    </div>

    <!-- QR / Status Display -->
    <div class="relative group">
        <!-- Decoration -->
        <div class="absolute -inset-1 bg-gradient-to-r from-primary/10 to-success/10 rounded-3xl blur opacity-20 group-hover:opacity-40 transition-opacity"></div>
        
        <div class="relative flex flex-col items-center py-10 bg-[#0b0e14] border border-white/10 rounded-3xl overflow-hidden shadow-2xl">
            <!-- QR View -->
            <div v-if="status.qr && !status.loggedIn" class="flex flex-col items-center animate-in zoom-in slide-in-from-bottom-4 duration-500">
                <div class="bg-white p-4 rounded-2xl shadow-[0_0_30px_rgba(255,255,255,0.1)] mb-6 ring-1 ring-white/20">
                    <img v-if="qrImage" :src="qrImage" class="w-60 h-60 object-contain" alt="Scan QR Code" />
                    <div v-else class="w-60 h-60 flex items-center justify-center text-slate-400">
                        <!-- Assuming Loader2 is available or a simple spinner is desired -->
                        <span class="loading loading-spinner loading-lg"></span>
                    </div>
                </div>
                <div class="flex items-center gap-3 px-6 py-2 bg-white/5 rounded-full border border-white/10">
                    <div class="w-2 h-2 rounded-full bg-primary animate-ping"></div>
                    <span class="text-xs font-black text-slate-300 uppercase tracking-widest">Scanning Active</span>
                </div>
            </div>

            <!-- Logged In View -->
            <div v-else-if="status.loggedIn" class="flex flex-col items-center py-6 animate-in zoom-in duration-500 w-full px-10">
                <div class="relative group/avatar mb-6">
                    <div class="w-32 h-32 rounded-full overflow-hidden border-4 border-success/20 shadow-2xl bg-black/40 flex items-center justify-center relative">
                        <img v-if="status.profilePhoto" :src="status.profilePhoto" class="w-full h-full object-cover transition-transform group-hover/avatar:scale-110" alt="Profile" />
                        <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-16 w-16 text-success/40" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" /></svg>
                        
                        <!-- Upload Overlay -->
                        <div v-if="!uploadingPhoto" @click="fileInput?.click()" class="absolute inset-0 bg-black/60 opacity-0 group-hover/avatar:opacity-100 transition-opacity flex flex-col items-center justify-center cursor-pointer">
                            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white mb-1" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                            <span class="text-xs font-black text-white uppercase tracking-widest">Update</span>
                        </div>
                        <div v-else class="absolute inset-0 bg-black/60 flex items-center justify-center">
                            <span class="loading loading-spinner loading-sm text-primary"></span>
                        </div>
                    </div>
                    
                    <input type="file" ref="fileInput" class="hidden" accept="image/*" @change="updateProfilePhoto" />

                    <!-- Status Dot -->
                    <div class="absolute bottom-2 right-2 w-6 h-6 rounded-full bg-[#0b0e14] flex items-center justify-center border border-white/5">
                        <div class="w-3 h-3 rounded-full bg-success animate-pulse"></div>
                    </div>
                </div>

                <div class="text-center">
                    <h3 class="text-2xl font-black text-white uppercase tracking-tighter mb-1">Instance Linked</h3>
                    <p class="text-xs text-slate-500 font-bold uppercase tracking-widest">Protocol Sync Complete</p>
                </div>

                <div v-if="status.isPaused" class="mt-6 px-4 py-1.5 bg-warning/10 border border-warning/20 rounded-lg w-full text-center">
                   <p class="text-xs text-warning font-bold uppercase tracking-widest">Bot is Paused • Resume to restore real-time sync</p>
                </div>
            </div>

            <!-- Ready / Waiting View -->
            <div v-else class="flex flex-col items-center py-12 opacity-30 select-none">
                <div class="w-20 h-20 mb-6 flex items-center justify-center bg-white/5 rounded-full ring-1 ring-white/10">
                    <i class="whatsapp icon big" style="color: white !important"></i>
                </div>
                 <p class="text-xs font-black text-white uppercase tracking-widest">Ready for Auth</p>
            </div>

            <!-- Status Indicator Overlay (Bottom) -->
            <div v-if="status.loading" class="absolute inset-0 bg-[#0b0e14]/60 backdrop-blur-[2px] flex items-center justify-center z-10 transition-all">
                <span class="loading loading-ring loading-lg text-primary"></span>
            </div>
        </div>
    </div>

    <!-- Action Bar -->
    <div class="grid grid-cols-1 gap-4">
        <!-- Login Methods -->
        <div v-if="!status.loggedIn" class="space-y-6">
            <!-- Toggle Method -->
            <div class="flex p-1.5 bg-black/40 rounded-[1.25rem] border border-white/5 shadow-2xl">
                <button class="flex-1 py-3 text-xs font-black uppercase tracking-widest rounded-xl transition-all cursor-pointer" 
                    :class="loginMethod === 'qr' ? 'bg-primary text-white shadow-xl shadow-primary/20' : 'text-slate-500 hover:text-slate-300'"
                    @click="loginMethod = 'qr'; status.qr = null; pairingCode = null">
                    Digital QR
                </button>
                <button class="flex-1 py-3 text-xs font-black uppercase tracking-widest rounded-xl transition-all cursor-pointer" 
                    :class="loginMethod === 'code' ? 'bg-primary text-white shadow-xl shadow-primary/20' : 'text-slate-500 hover:text-slate-300'"
                    @click="loginMethod = 'code'; status.qr = null; pairingCode = null">
                    Pairing Code
                </button>
            </div>

            <!-- QR Action -->
            <button v-if="loginMethod === 'qr' && !status.qr" @click="login" class="btn-premium btn-premium-primary w-full h-16" :disabled="status.loading">
                Initialize Gateway
            </button>

            <!-- Code Action -->
            <div v-if="loginMethod === 'code' && !pairingCode" class="space-y-4 animate-in fade-in slide-in-from-bottom-2">
                 <div class="form-control">
                    <label class="label-premium px-1">Infrastructure Phone Number</label>
                    <input v-model="phoneNumber" type="tel" placeholder="e.g. 51999999999" class="input-premium bg-black/40 text-center font-mono text-xl tracking-widest w-full h-16" />
                    <p class="text-xs text-slate-600 font-bold uppercase tracking-widest mt-2 px-1">Include country code without special characters</p>
                </div>
                <button @click="loginWithCode" class="btn-premium btn-premium-primary w-full h-16" :disabled="status.loading || !phoneNumber">
                    Request Secure Code
                </button>
            </div>
            
            <!-- Code Display -->
            <div v-if="pairingCode" class="flex flex-col items-center gap-6 animate-in zoom-in duration-300">
                <div class="flex gap-3">
                     <span v-for="(char, i) in pairingCode" :key="i" class="w-11 h-16 flex items-center justify-center bg-black/60 border border-white/10 rounded-xl text-3xl font-black font-mono text-primary shadow-2xl ring-1 ring-primary/20">
                        {{ char }}
                     </span>
                </div>
                <div class="p-6 bg-primary/5 border border-primary/20 rounded-2xl">
                    <p class="text-xs text-primary/80 font-bold uppercase tracking-widest text-center max-w-sm leading-relaxed">
                        WhatsApp > Linked Devices > Link a Device > Link with phone number instead
                    </p>
                </div>
                <button @click="pairingCode = null" class="text-xs font-black uppercase tracking-widest text-slate-600 hover:text-primary transition-colors cursor-pointer">Abort & Reset</button>
            </div>
        </div>

        <!-- Logout Action -->
        <div v-if="status.loggedIn" class="flex flex-col gap-4">
            <button @click="logout" class="btn-premium btn-premium-ghost text-red-500/60 hover:text-red-500 border border-red-500/10 hover:bg-red-500/10 w-full h-16">
                Terminate Device session
            </button>
            <div class="p-5 bg-success/5 border border-success/10 rounded-2xl flex items-center justify-center gap-3">
               <div class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></div>
               <p class="text-xs text-success/70 font-black uppercase tracking-widest text-center">Protocol Synchronized • Session Healthy</p>
            </div>
        </div>

        <button class="btn-premium btn-premium-ghost h-12 w-full text-xs opacity-40 hover:opacity-100" @click="refresh" :disabled="status.loading">
            Force Status Sync
        </button>
    </div>
  </div>
</template>
