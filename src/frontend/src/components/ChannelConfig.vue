<script setup lang="ts">
import { ref, onMounted, computed, watch, h } from 'vue'
import { useApi } from '@/composables/useApi'
import ConfirmationDialog from '@/components/ConfirmationDialog.vue'
import AppTabModal from '@/components/AppTabModal.vue'
import { Search, UserCheck, ShieldAlert, CheckCircle2, Loader2, Image, Mic, Film, FileText, HardDrive, Sticker, Plus, X, ShieldCheck, Brain, Zap, BarChart3, TrendingUp, DollarSign, Settings, Bot, Users, Activity, Calendar, Terminal, Edit3, Trash2 } from 'lucide-vue-next'
import ResourceSelector from '@/components/ResourceSelector.vue'
import SessionTimers from '@/components/SessionTimers.vue'
import HistoryLimitConfig from '@/components/HistoryLimitConfig.vue'

const WhatsAppIcon = (props: any) => h('svg', { 
  viewBox: "0 0 24 24", 
  fill: "currentColor",
  class: props.class 
}, [
  h('path', { d: "M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413Z" })
])

const props = defineProps<{
  channel: any
  workspaceId: string
  bots: any[]
  credentials: any[]
}>()

const emit = defineEmits(['saved', 'cancel', 'refresh'])
const api = useApi()

const localChannel = ref(JSON.parse(JSON.stringify(props.channel)))

const config = ref<any>({
  name: props.channel.name || '',
  webhook_url: '',
  webhook_secret: '',
  bot_id: '',
  skip_tls_verification: false,
  auto_reconnect: true,
  chatwoot: {
    enabled: false,
    account_id: 0,
    inbox_id: 0,
    token: '',
    url: '',
    bot_token: '',
    inbox_identifier: '',
  },
  access_mode: 'private',
  allow_images: true,
  allow_audio: true,
  allow_video: true,
  allow_documents: true,
  allow_stickers: true,
  voice_notes_only: false,
  allowed_extensions: [],
  max_download_size: 50 * 1024 * 1024,
  default_language: 'en',
  timezone: '',
  inactivity_warning: {
      enabled: true,
      default_lang: 'en',
      templates: {
          en: '',
          es: '',
          fr: '',
          ru: ''
      }
  },
  session_timeout: 4,
  inactivity_warning_time: 0,
  session_closing: {
      enabled: true,
      default_lang: 'es',
      templates: {
          en: '',
          es: '',
          fr: '',
          ru: ''
      }
  }
})

const extensionInput = ref('')

function addExtension() {
  const ext = extensionInput.value.trim().toLowerCase().replace(/^\./, '')
  if (ext && !config.value.allowed_extensions.includes(ext)) {
    config.value.allowed_extensions.push(ext)
  }
  extensionInput.value = ''
}

function removeExtension(ext: string) {
  config.value.allowed_extensions = config.value.allowed_extensions.filter((e: string) => e !== ext)
}


const accessRules = ref<any[]>([])
const showAllIdentities = ref(false)
const newRule = ref({
    identity: '',
    action: 'ALLOW',
    label: ''
})

const verifying = ref(false)
const resolvedInfo = ref<any>(null)
const isAddingRule = ref(false)

const filteredRules = computed(() => {
    if (showAllIdentities.value) return accessRules.value
    
    if (config.value.access_mode === 'private') {
        return accessRules.value.filter(r => r.action === 'ALLOW')
    } else {
        return accessRules.value.filter(r => r.action === 'DENY')
    }
})

const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'info' as 'danger' | 'warning' | 'info',
    confirmText: 'Confirm',
    onConfirm: () => {}
})

function loadInitialConfig() {
  if (props.channel) {
    config.value.name = props.channel.name || ''
    if (props.channel.config) {
        const c = JSON.parse(JSON.stringify(props.channel.config))
        if (!c.chatwoot) {
            c.chatwoot = { enabled: false, account_id: 0, inbox_id: 0, token: '', url: '', bot_token: '', inbox_identifier: '', credential_id: '', webhook_url: '' }
        }
        Object.assign(config.value, c)
        // Ensure defaults if missing (newly added fields)
        if (config.value.allow_images === undefined) config.value.allow_images = true
        if (config.value.allow_audio === undefined) config.value.allow_audio = true
        if (config.value.allow_video === undefined) config.value.allow_video = true
        if (config.value.allow_documents === undefined) config.value.allow_documents = true
        if (config.value.allow_stickers === undefined) config.value.allow_stickers = true
        if (config.value.voice_notes_only === undefined) config.value.voice_notes_only = false
        if (!config.value.allowed_extensions) config.value.allowed_extensions = []
        if (!config.value.max_download_size) config.value.max_download_size = 50 * 1024 * 1024
        if (!config.value.default_language) config.value.default_language = 'en'
        if (!config.value.timezone) config.value.timezone = ''
        if (!config.value.inactivity_warning) {
            config.value.inactivity_warning = {
                enabled: true,
                default_lang: 'en',
                templates: { en: '', es: '', fr: '' }
            }
        }
        if (config.value.session_timeout === undefined) config.value.session_timeout = 4
        if (config.value.inactivity_warning_time === undefined || config.value.inactivity_warning_time === 0) config.value.inactivity_warning_time = 3
        if (config.value.max_history_limit === undefined) config.value.max_history_limit = 10
    }
  }
  
  const apiBase = (localStorage.getItem('api_url') || window.location.origin).replace(/\/$/, '')
  config.value.chatwoot.webhook_url = `${apiBase}/workspaces/${props.workspaceId}/channels/${props.channel.id}/chatwoot/webhook`

  loadAccessRules()
  refreshChannelData()
}

async function refreshChannelData() {
    try {
        const res = await api.get(`/workspaces/${props.workspaceId}/channels`)
        if (res && Array.isArray(res)) {
            const fresh = res.find((c: any) => c.id === props.channel.id)
            if (fresh) {
                localChannel.value = fresh
            }
        }
    } catch (err) {
        console.warn('Failed to refresh channel data for cost visualization', err)
    }
}

async function loadAccessRules() {
    try {
        const res = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/access-rules`)
        accessRules.value = res || []
    } catch (err) {
        console.error('Failed to load access rules', err)
    }
}

async function verifyIdentity() {
    if (!newRule.value.identity) return
    verifying.value = true
    resolvedInfo.value = null
    try {
        // Intentional delay for "extra drama" and visual scan effect
        await new Promise(resolve => setTimeout(resolve, 1200))
        
        const res = await api.get(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/resolve-identity?identity=${newRule.value.identity}`)
        resolvedInfo.value = res
        if (res.name && !newRule.value.label) {
            newRule.value.label = res.name
        }
        // NOT updating identity here to avoid "rebote" in UI
        return true
    } catch (err: any) {
        console.error('Verify failed', err)
        // Mostrar directamente el mensaje del servidor
        resolvedInfo.value = { error: true, message: String(err.message || 'Unknown error').toUpperCase() }
        return false
    } finally {
        verifying.value = false
    }
}

const getBotName = (botId: string | number | undefined) => {
	if (!botId) return 'UNKNOWN'
	const b = props.bots.find(b => b.id === String(botId))
	return b ? b.name : String(botId).substring(0, 8)
}

const sanitizeIdentity = (id: string) => {
    if (!id) return ''
    return id.replace(/[^\d+]/g, '').replace(/^\+/, '')
}

async function addAccessRule() {
    if (!newRule.value.identity || verifying.value) return
    
    // DRAMA: Verify first - server will detect duplicates
    const success = await verifyIdentity()
    if (!success) {
        // resolvedInfo already has the correct error message from verifyIdentity
        return
    }

    // Dramatic pause after success so user can see what was found
    await new Promise(resolve => setTimeout(resolve, 800))

    try {
        const action = config.value.access_mode === 'private' ? 'ALLOW' : 'DENY'
        
        // Use the resolved LID/JID for the database but keep UI clean
        // If no label, use the original typed number so it shows up in history
        const finalPayload = {
            ...newRule.value,
            identity: resolvedInfo.value?.resolved_identity || newRule.value.identity,
            label: newRule.value.label || newRule.value.identity,
            action: action
        }

        await api.post(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/access-rules`, finalPayload)
        newRule.value = { identity: '', action: 'ALLOW', label: '' }
        resolvedInfo.value = null
        isAddingRule.value = false
        await loadAccessRules()
    } catch (err: any) {
        console.error('Add rule failed', err)
        const errText = String(err.message || '')
        if (errText.toLowerCase().includes('already') || errText.toLowerCase().includes('duplicate')) {
            resolvedInfo.value = { error: true, message: 'THIS IDENTITY IS ALREADY PROTECTED!' }
        } else {
            resolvedInfo.value = { error: true, message: 'FAILED TO SAVE IDENTITY' }
        }
    }
}

async function deleteAllRules() {
    confirmModal.value = {
        show: true,
        title: 'Purge All Access Rules?',
        message: 'This will permanently remove all whitelist and blacklist entries for this channel.',
        type: 'danger',
        confirmText: 'Purge Everything',
        onConfirm: async () => {
             try {
                await api.delete(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/access-rules`)
                await loadAccessRules()
            } catch (err) {
                console.error('Delete all failed', err)
            }
        }
    }
}

async function deleteAccessRule(rid: string) {
    try {
        await api.delete(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/access-rules/${rid}`)
        await loadAccessRules()
    } catch (err) {
        console.error('Delete rule failed', err)
    }
}

async function save() {
  try {
    const payload = JSON.parse(JSON.stringify(config.value))
    const newName = payload.name
    delete payload.name 
    if (payload.chatwoot) delete payload.chatwoot.webhook_url
    
    // Update Instance Name
    await api.put(`/workspaces/${props.workspaceId}/channels/${props.channel.id}`, {
        name: newName,
        type: props.channel.type
    })

    // Update Instance Config
    await api.put(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/config`, payload)
    
    emit('saved')
  } catch (err) {
    console.error('Save failed', err)
  }
}

async function clearChannelMemory() {
    confirmModal.value = {
        show: true,
        title: 'Flush Channel Context?',
        message: 'This will wipe all short-term conversation memory for this specific channel. The bot will forget recent context with users in this channel only.',
        type: 'warning',
        confirmText: 'Flush Context',
        onConfirm: async () => {
            try {
                await api.post(`/workspaces/${props.workspaceId}/channels/${props.channel.id}/bot-memory/clear`, {})
                console.log('Channel memory flushed')
            } catch (err) {
                console.error('Flush failed', err)
            }
        }
    }
}

function copyWebhook() {
  if (config.value.chatwoot.webhook_url) {
    navigator.clipboard.writeText(config.value.chatwoot.webhook_url)
  }
}

function copyId() {
    navigator.clipboard.writeText(props.channel.id)
}

const activeTab = ref('general')
const subscribers = ref<any[]>([])
const loadingSubscribers = ref(false)

const editingSub = ref<any>(null)
const editForm = ref({
    custom_bot_id: '',
    custom_system_prompt: '',
    expires_at: '',
    session_timeout: null as number | null,
    inactivity_warning_time: null as number | null,
    max_history_limit: null as number | null
})

function startEditSubscription(item: any) {
    editingSub.value = item
    const sub = item.subscription
    editForm.value = {
        custom_bot_id: sub.custom_bot_id || '',
        custom_system_prompt: sub.custom_system_prompt || '',
        expires_at: sub.expires_at ? sub.expires_at.split('T')[0] : '',
        session_timeout: sub.session_timeout || null,
        inactivity_warning_time: sub.inactivity_warning_time || null,
        max_history_limit: sub.max_history_limit !== undefined ? sub.max_history_limit : null
    }
}

async function saveSubscription() {
    if (!editingSub.value) return
    try {
        const payload = {
            custom_bot_id: editForm.value.custom_bot_id || null, 
            custom_system_prompt: editForm.value.custom_system_prompt || null,
            expires_at: editForm.value.expires_at ? new Date(editForm.value.expires_at).toISOString() : null,
            clear_expires_at: !editForm.value.expires_at,
            session_timeout: editForm.value.session_timeout && editForm.value.session_timeout > 0 ? editForm.value.session_timeout : null,
            inactivity_warning_time: editForm.value.inactivity_warning_time && editForm.value.inactivity_warning_time > 0 ? editForm.value.inactivity_warning_time : null,
            max_history_limit: editForm.value.max_history_limit,
            clear_session_timeout: !editForm.value.session_timeout,
            clear_inactivity_warning: !editForm.value.inactivity_warning_time,
            clear_max_history_limit: editForm.value.max_history_limit === null || editForm.value.max_history_limit === undefined
        }
        await api.put(`/clients/${editingSub.value.client.id}/subscriptions/${editingSub.value.subscription.id}`, payload)
        editingSub.value = null
        await loadSubscribers()
    } catch(e) {
        console.error(e)
        alert('Failed to update subscription')
    }
}

async function removeSubscription(item: any) {
    confirmModal.value = {
        show: true,
        title: 'Revoke Subscription?',
        message: 'This will remove the bot assignment for this client immediately. They will revert to default channel behavior.',
        type: 'danger',
        confirmText: 'Revoke',
        onConfirm: async () => {
             try {
                await api.delete(`/clients/${item.client.id}/subscriptions/${item.subscription.id}`)
                await loadSubscribers()
            } catch (err) {
                console.error(err)
            }
        }
    }
}

async function loadSubscribers() {
    loadingSubscribers.value = true
    try {
        const res = await api.get(`/channels/${props.channel.id}/subscribers`) as any
        subscribers.value = res.data || []
    } catch (err) {
        console.error('Failed to load subscribers', err)
    } finally {
        loadingSubscribers.value = false
    }
}

watch(activeTab, (newTab) => {
    if (newTab === 'subscribers') {
        loadSubscribers()
    }
}, { immediate: true })

onMounted(loadInitialConfig)
</script>

<template>
    <AppTabModal
        :modelValue="true"
        :title="'Instance Configuration'"
        v-model:activeTab="activeTab"
        :tabs="[
            { id: 'general', label: 'General', icon: CheckCircle2 },
            { id: 'investments', label: 'Investments', icon: BarChart3 },
            { id: 'connectivity', label: 'Connectivity', icon: Zap },
            { id: 'security', label: 'Identity Guard', icon: ShieldCheck },
            { id: 'subscribers', label: 'Subscribers', icon: Users },
            { id: 'integrations', label: 'Integrations', icon: HardDrive }
        ]"
        :identity="{
            name: channel.name,
            id: channel.id,
            icon: channel.type === 'whatsapp' ? WhatsAppIcon : Settings,
            iconType: channel.type === 'whatsapp' ? 'component' : 'component',
            iconClass: channel.type === 'whatsapp' ? 'text-[#25D366]' : ''
        }"
        @save="save"
        @cancel="emit('cancel')"
    >
      <template #sidebar-bottom>
          <div class="storage-box-premium">
              <div class="flex flex-col gap-1">
                  <span class="text-xs font-black uppercase text-primary/60 tracking-widest">Global Investment</span>
                  <div class="flex items-baseline gap-2">
                      <span class="text-2xl font-black text-white tracking-tighter">${{ (localChannel.accumulated_cost || 0).toFixed(4) }}</span>
                      <span class="text-xs font-bold text-slate-500 uppercase">USD</span>
                  </div>
              </div>
          </div>
      </template>

      <template #footer-start>
          <div class="flex items-center gap-2">
              <div class="w-2 h-2 rounded-full bg-primary animate-pulse shadow-[0_0_8px_rgba(var(--p),0.5)]"></div>
              <span class="text-xs font-black text-primary uppercase tracking-[0.2em]">Active Telemetry Session</span>
          </div>
      </template>
      
      <!-- TAB: General -->
      <div v-if="activeTab === 'general'" class="space-y-10">
        <div class="section-title-premium text-primary/60">General Settings</div>

        <section class="grid grid-cols-1 gap-8">
            <div class="form-control w-full">
                <label class="label-premium">Friendly Name</label>
                <input v-model="config.name" type="text" placeholder="e.g. Sales WhatsApp" class="input-premium h-14 w-full text-lg font-black" />
            </div>
        </section>

        <section class="space-y-6">
            <div class="form-control w-full">
                <ResourceSelector
                    v-model="config.bot_id"
                    :items="bots"
                    label="Assigned Bot Engine"
                    placeholder="Search for a bot engine..."
                    iconType="bot"
                    resourceLabel="Active Engine"
                    :nullable="true"
                    color="primary"
                />
            </div>

            <div class="form-control w-full">
                <label class="label-premium">Default Response Language</label>
                <select v-model="config.default_language" class="select-premium h-14 w-full">
                    <option value="es">Español</option>
                    <option value="en">English (US)</option>
                    <option value="pt">Português</option>
                    <option value="fr">Français</option>
                    <option value="it">Italiano</option>
                    <option value="de">Deutsch</option>
                    <option value="ru">Русский</option>
                </select>
                <p class="text-xs text-slate-600 font-bold uppercase mt-2 tracking-widest pl-2">Fallback language for non-registered clients or general channel vibe.</p>
            </div>

            <div class="form-control w-full">
                <label class="label-premium">Default Timezone</label>
                <select v-model="config.timezone" class="select-premium h-14 w-full">
                    <option value="">System Default (UTC)</option>
                    <option value="America/New_York">🇺🇸 (GMT-05:00) America/New_York</option>
                    <option value="America/Los_Angeles">🇺🇸 (GMT-08:00) America/Los_Angeles</option>
                    <option value="America/Chicago">🇺🇸 (GMT-06:00) America/Chicago</option>
                    <option value="America/Lima">🇵🇪 (GMT-05:00) America/Lima</option>
                    <option value="America/Bogota">🇨🇴 (GMT-05:00) America/Bogota</option>
                    <option value="America/Mexico_City">🇲🇽 (GMT-06:00) America/Mexico_City</option>
                    <option value="America/Santo_Domingo">🇩🇴 (GMT-04:00) America/Santo_Domingo</option>
                    <option value="America/Sao_Paulo">🇧🇷 (GMT-03:00) America/Sao_Paulo</option>
                    <option value="America/Buenos_Aires">🇦🇷 (GMT-03:00) America/Buenos_Aires</option>
                    <option value="America/Santiago">🇨🇱 (GMT-04:00) America/Santiago</option>
                    <option value="Europe/London">🇬🇧 (GMT+00:00) Europe/London</option>
                    <option value="Europe/Paris">🇫🇷 (GMT+01:00) Europe/Paris</option>
                    <option value="Europe/Madrid">🇪🇸 (GMT+01:00) Europe/Madrid</option>
                    <option value="Europe/Berlin">🇩🇪 (GMT+01:00) Europe/Berlin</option>
                    <option value="Asia/Tokyo">🇯🇵 (GMT+09:00) Asia/Tokyo</option>
                    <option value="Asia/Shanghai">🇨🇳 (GMT+08:00) Asia/Shanghai</option>
                    <option value="Asia/Dubai">🇦🇪 (GMT+04:00) Asia/Dubai</option>
                    <option value="Australia/Sydney">🇦🇺 (GMT+10:00) Australia/Sydney</option>
                </select>
                <p class="text-xs text-slate-600 font-bold uppercase mt-2 tracking-widest pl-2">Timezone for AI tools and time-sensitive operations. Clients can override this individually.</p>
            </div>

            <!-- Inactivity Warning Section -->
            <div class="divider opacity-5"></div>

            <section class="space-y-6">
                <div>
                    <h4 class="text-sm font-bold text-white uppercase tracking-[0.2em] border-l-2 border-primary pl-4">Session Lifecycle Timers</h4>
                    <p class="text-xs text-slate-500 font-medium uppercase mt-1">Control session duration and warning alerts</p>
                </div>

                <SessionTimers 
                    v-model:timeout="config.session_timeout"
                    v-model:warning="config.inactivity_warning_time"
                />
            </section>
            
            <section class="space-y-6">
                <div>
                     <h4 class="text-sm font-bold text-white uppercase tracking-[0.2em] border-l-2 border-primary pl-4">Memory & Context</h4>
                     <p class="text-xs text-slate-500 font-medium uppercase mt-1">Configure conversation depth</p>
                </div>
                <HistoryLimitConfig v-model="config.max_history_limit" />
            </section>

            <section class="space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h4 class="text-sm font-bold text-white uppercase tracking-[0.2em] border-l-2 border-primary pl-4">Session Lifecycle Warning</h4>
                        <p class="text-xs text-slate-500 font-medium uppercase mt-1">Notify users before their session expires due to inactivity</p>
                    </div>
                    <input type="checkbox" v-model="config.inactivity_warning.enabled" class="toggle toggle-primary toggle-sm" />
                </div>

                <div v-if="config.inactivity_warning.enabled" class="space-y-6 p-6 bg-black/20 rounded-2xl border border-white/5 animate-in fade-in slide-in-from-top-4">
                    <div class="form-control">
                        <label class="label-premium">Warning Default Language</label>
                        <select v-model="config.inactivity_warning.default_lang" class="select-premium h-12 w-full text-sm">
                            <option value="en">English (US)</option>
                            <option value="es">Español</option>
                            <option value="fr">Français</option>
                            <option value="ru">Русский</option>
                        </select>
                    </div>

                    <div class="space-y-4">
                        <label class="text-xs font-bold text-slate-500 uppercase tracking-widest px-1">Custom Message Template ({{ config.inactivity_warning.default_lang === 'en' ? 'ENGLISH' : config.inactivity_warning.default_lang === 'es' ? 'SPANISH' : config.inactivity_warning.default_lang === 'fr' ? 'FRENCH' : 'RUSSIAN' }})</label>
                        
                        <div class="space-y-2">
                             <textarea 
                                v-model="config.inactivity_warning.templates[config.inactivity_warning.default_lang]"
                                :placeholder="config.inactivity_warning.default_lang === 'es' ? '¿Sigues ahí? Cerraré la sesión pronto...' : 'Are you still there? Session closing soon...'"
                                class="textarea textarea-bordered bg-black/40 border-white/5 focus:border-primary/40 text-sm h-24 w-full resize-none transition-all"
                            ></textarea>
                            <p class="text-xs text-slate-500 font-medium uppercase px-1">Use this to personalize the warning for the primary audience.</p>
                        </div>
                    </div>
                </div>
            </section>

             <!-- Session Closing Message Section -->
             <section class="space-y-6">
                <div class="flex items-center justify-between">
                    <div>
                        <h4 class="text-sm font-bold text-white uppercase tracking-[0.2em] border-l-2 border-primary pl-4">Session Closing Message</h4>
                        <p class="text-xs text-slate-500 font-medium uppercase mt-1">Final message sent when session is terminated due to inactivity</p>
                    </div>
                    <input type="checkbox" v-model="config.session_closing.enabled" class="toggle toggle-primary toggle-sm" />
                </div>

                <div v-if="config.session_closing.enabled" class="space-y-6 p-6 bg-black/20 rounded-2xl border border-white/5 animate-in fade-in slide-in-from-top-4">
                    <div class="form-control">
                        <label class="label-premium">Closing Message Language</label>
                        <select v-model="config.session_closing.default_lang" class="select-premium h-12 w-full text-sm">
                            <option value="en">English (US)</option>
                            <option value="es">Español</option>
                            <option value="fr">Français</option>
                            <option value="ru">Русский</option>
                        </select>
                    </div>

                    <div class="space-y-4">
                        <label class="text-xs font-bold text-slate-500 uppercase tracking-widest px-1">Custom Message Template ({{ config.session_closing.default_lang === 'en' ? 'ENGLISH' : config.session_closing.default_lang === 'es' ? 'SPANISH' : config.session_closing.default_lang === 'fr' ? 'FRENCH' : 'RUSSIAN' }})</label>
                        
                        <div class="space-y-2">
                             <textarea 
                                v-model="config.session_closing.templates[config.session_closing.default_lang]"
                                :placeholder="config.session_closing.default_lang === 'es' ? 'La sesión ha sido cerrada por inactividad.' : 'Session closed due to inactivity.'"
                                class="textarea textarea-bordered bg-black/40 border-white/5 focus:border-primary/40 text-sm h-24 w-full resize-none transition-all"
                            ></textarea>
                        </div>
                    </div>
                </div>
            </section>

            
            <div class="p-6 bg-white/[0.02] rounded-2xl border border-white/5 flex items-center justify-between">
                <div>
                  <h4 class="text-xs font-black text-white uppercase tracking-widest leading-none mb-1">Instance Memory</h4>
                  <p class="text-xs text-slate-500 uppercase font-bold tracking-tight">Flush short-term context for this specific channel</p>
                </div>
                <button @click="clearChannelMemory" class="btn-premium btn-premium-ghost text-red-400 hover:bg-red-500/10 border border-red-500/10 btn-premium-sm px-6">
                    Clear Context
                </button>
            </div>
        </section>
      </div>




      <div v-if="activeTab === 'investments'" class="space-y-10">
        <header>
          <div class="flex items-center gap-4">
             <div class="p-3 bg-primary/10 rounded-2xl border border-primary/20">
                <TrendingUp class="w-6 h-6 text-primary" />
             </div>
              <div>
                 <h3 class="text-2xl font-black text-white uppercase tracking-tight">Token Investments</h3>
                 <p class="text-xs text-slate-500 font-medium uppercase tracking-widest mt-1">AI Expenditure Analysis & Model Performance Monitoring</p>
              </div>
           </div>
         </header>
 
         <!-- Dynamic Investment Overview -->
         <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
             <div class="p-6 bg-black/40 rounded-[1.5rem] border border-white/5 space-y-2">
                 <span class="text-xs font-medium uppercase tracking-[0.2em] text-slate-500">Total Expenditure</span>
                 <div class="flex items-baseline gap-2">
                     <span class="text-2xl font-light text-slate-400">$</span>
                     <span class="text-3xl font-black text-white tracking-tighter">{{ (localChannel.accumulated_cost || 0).toFixed(6) }}</span>
                 </div>
             </div>
             <div class="p-6 bg-black/40 rounded-[1.5rem] border border-white/5 space-y-2">
                 <span class="text-xs font-medium uppercase tracking-[0.2em] text-slate-500">Primary Model</span>
                 <div class="flex items-center gap-2">
                     <Zap class="w-4 h-4 text-primary" />
                     <span class="text-sm font-bold text-white uppercase tracking-widest truncate">{{ localChannel.cost_breakdown ? Object.keys(localChannel.cost_breakdown)[0]?.split(':')[1] || 'PENDING' : 'N/A' }}</span>
                 </div>
             </div>
             <div class="p-6 bg-primary/5 rounded-[1.5rem] border border-primary/20 space-y-2 relative overflow-hidden">
                 <div class="absolute -right-4 -bottom-4 opacity-5">
                     <DollarSign class="w-16 h-16" />
                 </div>
                 <span class="text-xs font-medium uppercase tracking-[0.2em] text-primary/40">Efficiency Rating</span>
                 <div class="flex items-center gap-2 relative z-10">
                     <span class="text-base font-bold text-white uppercase tracking-widest">Optimized Core</span>
                 </div>
             </div>
        </div>

        <!-- Distribution Breakdown -->
        <section class="space-y-6">
            <div class="flex items-center justify-between border-l-2 border-primary pl-4">
                <div>
                   <h4 class="text-sm font-bold text-white uppercase tracking-[0.2em]">Detailed Expenditure Log</h4>
                   <p class="text-xs text-slate-500 font-medium uppercase mt-1">Real-time breakdown of AI model usage and costs</p>
                </div>
                <div class="px-3 py-1 bg-white/5 rounded-lg border border-white/10 flex items-center gap-2">
                    <div class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></div>
                    <span class="text-xs font-bold text-slate-500 uppercase tracking-widest">Update Frequency: Live</span>
                </div>
            </div>

            <div v-if="localChannel.cost_breakdown && Object.keys(localChannel.cost_breakdown).length > 0" class="grid grid-cols-1 gap-3">
                 <div v-for="(val, key) in localChannel.cost_breakdown" :key="key" class="p-5 bg-white/[0.02] border border-white/5 rounded-[1.5rem] flex items-center justify-between group hover:bg-white/[0.04] transition-all">
                     <div class="flex items-center gap-5">
                         <div class="w-12 h-12 rounded-2xl bg-black/60 flex items-center justify-center ring-1 ring-white/10 group-hover:ring-primary/40 transition-all shadow-2xl">
                             <Zap class="w-5 h-5" :class="val > 0.01 ? 'text-primary' : 'text-slate-600'" />
                         </div>
                         <div class="flex flex-col">
                             <span class="text-base text-white font-bold uppercase tracking-tight">{{ getBotName(String(key).split(':')[0] || '') }}</span>
                             <span class="text-xs font-medium text-slate-500 uppercase tracking-wider mt-0.5">{{ String(key).split(':')[1] }}</span>
                         </div>
                     </div>
                     <div class="flex items-center gap-10">
                        <div class="flex flex-col items-end">
                            <div class="flex items-baseline gap-1">
                                <span class="text-xs text-slate-600 font-medium uppercase">$</span>
                                <span class="text-lg font-mono text-white font-medium">{{ val.toFixed(6) }}</span>
                            </div>
                            <span class="text-xs text-slate-500 font-bold uppercase tracking-widest">Accumulated</span>
                        </div>
                        <div class="flex flex-col gap-1.5 min-w-[120px]">
                            <div class="flex justify-between items-center text-xs font-bold uppercase text-slate-600 tracking-tighter">
                                <span>Core Weight</span>
                                <span>{{ Math.round((val / (localChannel.accumulated_cost || 0.000001)) * 100) }}%</span>
                            </div>
                            <div class="w-32 h-1.5 bg-black/40 rounded-full overflow-hidden border border-white/5">
                                <div class="h-full bg-primary/80 shadow-[0_0_10px_rgba(var(--p),0.5)]" :style="{ width: `${Math.min(100, (val / (localChannel.accumulated_cost || 0.000001)) * 100)}%` }"></div>
                            </div>
                        </div>
                     </div>
                 </div>
            </div>
            <div v-else class="py-24 text-center bg-black/10 rounded-[2rem] border border-dashed border-white/5">
                <BarChart3 class="w-10 h-10 text-slate-700 mx-auto mb-4 opacity-20" />
                <p class="text-sm font-bold text-slate-600 uppercase tracking-widest">Awaiting Financial Telemetry</p>
            </div>
        </section>
      </div>

      <!-- TAB: Connectivity -->
      <div v-if="activeTab === 'connectivity'" class="space-y-10">
        <header>
          <h3 class="text-2xl font-black text-white uppercase tracking-tight">Connectivity & Media</h3>
          <p class="text-xs text-slate-500 font-medium uppercase tracking-widest mt-1">Configure protocol behaviors and physical media filtering</p>
        </header>

        <section class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div class="p-6 bg-white/[0.02] rounded-2xl border border-white/5 space-y-4">
               <span class="text-xs font-bold text-slate-500 uppercase tracking-widest px-2">Protocol Flags</span>
               <div class="space-y-4">
                <label class="flex items-center justify-between cursor-pointer group">
                    <span class="text-sm font-semibold uppercase tracking-widest text-slate-400 group-hover:text-slate-200 transition-colors">Skip TLS Verification</span>
                    <input type="checkbox" v-model="config.skip_tls_verification" class="checkbox checkbox-primary checkbox-sm" />
                </label>
                <label class="flex items-center justify-between cursor-pointer group">
                    <span class="text-sm font-semibold uppercase tracking-widest text-slate-400 group-hover:text-slate-200 transition-colors">Auto Reconnect</span>
                    <input type="checkbox" v-model="config.auto_reconnect" class="checkbox checkbox-primary checkbox-sm" />
                </label>
               </div>
            </div>

            <div class="p-6 bg-white/[0.02] rounded-2xl border border-white/5 space-y-4">
                <span class="text-xs font-bold text-slate-500 uppercase tracking-widest px-2">Storage Policy</span>
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <HardDrive class="w-5 h-5 text-primary" />
                        <span class="text-sm font-semibold uppercase tracking-widest text-slate-300">Max Download Size</span>
                    </div>
                    <div class="flex items-center gap-2">
                        <input 
                            type="number" 
                            :value="Math.round(config.max_download_size / (1024 * 1024))" 
                            @input="config.max_download_size = ($event.target as HTMLInputElement).valueAsNumber * 1024 * 1024"
                            class="input-premium w-20 h-9 text-center text-sm font-mono font-medium" 
                            min="1"
                        />
                        <span class="text-xs font-bold text-slate-500">MB</span>
                    </div>
                </div>
                <p class="text-xs text-slate-600 font-medium uppercase leading-tight tracking-wider">Larger files will be rejected automatically.</p>
            </div>
        </section>

        <section class="space-y-6">
            <h4 class="text-sm font-bold text-slate-500 uppercase tracking-[0.2em] border-l-2 border-primary pl-4">Media Content Filter</h4>
            <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
                <label v-for="type in [
                    { id: 'allow_images', label: 'Images', icon: Image, ext: 'jpg, png, webp' },
                    { id: 'allow_audio', label: 'Audio', icon: Mic, ext: 'mp3, ogg, amr' },
                    { id: 'allow_video', label: 'Videos', icon: Film, ext: 'mp4, avi, mov' },
                    { id: 'allow_documents', label: 'Docs', icon: FileText, ext: 'pdf, docx, txt' },
                    { id: 'allow_stickers', label: 'Stickers', icon: Sticker, ext: 'webp only' }
                ]" :key="type.id" class="flex flex-col gap-3 p-5 bg-white/[0.02] rounded-2xl cursor-pointer group hover:bg-white/[0.04] transition-all border border-white/5">
                    <div class="flex items-center justify-between">
                        <component :is="type.icon" class="w-5 h-5 text-slate-500 group-hover:text-primary transition-colors" />
                        <input type="checkbox" v-model="config[type.id]" class="checkbox checkbox-primary checkbox-sm" />
                    </div>
                    <div>
                        <span class="text-sm font-bold uppercase tracking-widest text-white">{{ type.label }}</span>
                        <p class="text-xs text-slate-600 font-medium uppercase mt-1">{{ type.ext }}</p>
                    </div>
                </label>
            </div>

            <div class="p-6 bg-black/40 rounded-2xl border border-white/5 space-y-6">
                <div class="flex items-center justify-between">
                    <div class="flex items-center gap-3">
                        <div class="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center">
                            <Mic class="w-6 h-6 text-primary" />
                        </div>
                        <div>
                            <span class="text-sm font-bold uppercase tracking-widest text-white">Voice Notes Only (PTT)</span>
                            <p class="text-xs text-slate-500 font-medium uppercase mt-1">Discharge standard audio files</p>
                        </div>
                    </div>
                    <input type="checkbox" v-model="config.voice_notes_only" class="toggle toggle-primary toggle-sm" />
                </div>
                
                <div class="divider opacity-5"></div>

                <div class="space-y-3">
                    <div class="flex items-center justify-between">
                        <span class="text-xs font-bold uppercase tracking-widest text-primary">Allowed Extensions Override</span>
                        <span class="text-xs text-slate-700 font-bold uppercase tracking-tighter">{{ config.allowed_extensions.length || 'System Default' }}</span>
                    </div>
                    <div class="flex flex-wrap gap-2 p-3 bg-black/20 rounded-xl border border-white/5 min-h-[48px]">
                        <div v-for="ext in config.allowed_extensions" :key="ext" class="flex items-center gap-2 px-3 py-1.5 bg-primary/10 border border-primary/20 rounded-lg">
                            <span class="text-xs font-bold text-primary uppercase leading-none">{{ ext }}</span>
                            <button @click="removeExtension(ext)" class="text-primary/40 hover:text-primary transition-colors">
                                <X class="w-3 h-3" />
                            </button>
                        </div>
                        <input 
                            v-model="extensionInput" 
                            @keydown.enter.prevent="addExtension" 
                            type="text" 
                            placeholder="+ Add Extension..." 
                            class="bg-transparent border-none outline-none text-sm text-white flex-1 min-w-[120px] font-medium placeholder:text-slate-700"
                        />
                    </div>
                </div>
            </div>
        </section>
      </div>
      
      <!-- TAB: Security -->
      <div v-if="activeTab === 'security'" class="space-y-10">
        <header>
          <h3 class="text-2xl font-black text-white uppercase tracking-tight">Identity Guard</h3>
          <p class="text-xs text-slate-500 font-bold uppercase tracking-widest mt-1">Manage global access rules for this instance</p>
        </header>

        <section class="p-8 rounded-[2.5rem] bg-white/[0.01] border border-white/5 space-y-10">
            <!-- Selector de Política de Acceso -->
            <div class="flex flex-col md:flex-row items-start md:items-center justify-between gap-6 p-6 bg-black/20 rounded-[2rem] border border-white/5">
                <div class="flex items-center gap-5">
                    <div class="w-14 h-14 rounded-2xl bg-primary/10 flex items-center justify-center ring-1 ring-primary/20 shadow-inner">
                        <ShieldAlert class="w-7 h-7 text-primary" />
                    </div>
                    <div>
                        <h4 class="text-base font-bold text-white uppercase tracking-wider">Access Strategy</h4>
                        <p class="text-xs font-medium text-slate-500 mt-1 uppercase tracking-tight max-w-xs">
                            {{ config.access_mode === 'private' ? 'Protective Shield: Only Whitelisted entities can interact.' : 'Global Intake: Everyone can talk except blocked signals.' }}
                        </p>
                    </div>
                </div>
                <div class="flex p-1.5 bg-black/40 rounded-2xl border border-white/10 shadow-2xl">
                    <button class="px-8 py-2.5 text-xs font-bold uppercase tracking-widest rounded-xl transition-all cursor-pointer" 
                        :class="config.access_mode === 'private' ? 'bg-primary text-white shadow-[0_0_20px_rgba(var(--p),0.3)]' : 'text-slate-500 hover:text-slate-300'" 
                        @click="config.access_mode = 'private'">Private</button>
                    <button class="px-8 py-2.5 text-xs font-bold uppercase tracking-widest rounded-xl transition-all cursor-pointer" 
                        :class="config.access_mode === 'public' ? 'bg-primary text-white shadow-[0_0_20px_rgba(var(--p),0.3)]' : 'text-slate-500 hover:text-slate-300'" 
                        @click="config.access_mode = 'public'">Public</button>
                </div>
            </div>

            <!-- Listado de Reglas -->
            <div class="space-y-6 pt-4">
                <div class="flex items-center justify-between px-4">
                    <div class="flex items-center gap-3">
                         <h5 class="text-xs font-bold text-slate-500 uppercase tracking-[0.2em]">Guard Manifest</h5>
                         <span class="px-2 py-0.5 bg-white/5 rounded-md text-xs font-bold text-slate-400 border border-white/5">{{ accessRules.length }} Active</span>
                    </div>
                    <div class="flex items-center gap-4">
                        <button v-if="accessRules.length > 0" @click="deleteAllRules" class="text-xs font-bold text-error/60 hover:text-error uppercase tracking-widest transition-all cursor-pointer mr-2">Purge All</button>
                        <button @click="isAddingRule = !isAddingRule" class="btn btn-primary btn-sm h-10 px-6 rounded-xl text-xs font-bold uppercase tracking-widest shadow-lg shadow-primary/10">
                            {{ isAddingRule ? 'Cancel' : 'Add New Signal' }}
                        </button>
                    </div>
                </div>

                <!-- Formulario de Entrada (Expandible) -->
                <Transition enter-active-class="transition duration-500 ease-out" enter-from-class="opacity-0 -translate-y-4 scale-95" leave-active-class="transition duration-300 ease-in" leave-to-class="opacity-0 -translate-y-4 scale-95">
                    <div v-if="isAddingRule" class="relative group mx-2">
                        <div class="absolute -inset-1 bg-gradient-to-r from-primary/20 to-indigo-500/20 rounded-[2rem] blur opacity-25 group-focus-within:opacity-50 transition duration-1000"></div>
                        
                        <div class="relative bg-black/60 p-8 rounded-[2rem] border border-white/5 space-y-6 shadow-2xl">
                            <div v-if="verifying" class="absolute inset-0 bg-primary/5 pointer-events-none overflow-hidden z-10 rounded-[2rem]">
                                <div class="absolute top-0 left-0 w-full h-[2px] bg-primary/60 shadow-[0_0_20px_rgba(var(--p),1)] animate-scan"></div>
                            </div>

                            <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
                                <div class="space-y-3">
                                    <label class="text-xs font-bold text-slate-500 uppercase tracking-widest px-1">Signal Identification</label>
                                    <div class="relative">
                                        <div class="absolute left-4 top-1/2 -translate-y-1/2 text-slate-600">
                                            <Zap class="w-4 h-4" />
                                        </div>
                                        <input v-model="newRule.identity" type="text" 
                                            :placeholder="channel.type === 'whatsapp' ? 'Phone (e.g. 51987...)' : 'System ID'" 
                                            class="input-premium h-14 pl-12 text-base font-medium placeholder:text-slate-700 bg-black/40"
                                            :class="{'border-primary/50 ring-2 ring-primary/10': verifying, 'border-error/40': resolvedInfo?.error}"
                                            @keyup.enter="addAccessRule"
                                            @input="resolvedInfo = null" />
                                    </div>
                                </div>

                                <div class="space-y-3">
                                    <label class="text-xs font-bold text-slate-500 uppercase tracking-widest px-1">Alias / Reference</label>
                                    <div class="relative">
                                        <div class="absolute left-4 top-1/2 -translate-y-1/2 text-slate-600">
                                            <UserCheck class="w-4 h-4" />
                                        </div>
                                        <input v-model="newRule.label" type="text" placeholder="e.g. VIP Customer" class="input-premium h-14 pl-12 text-base font-medium placeholder:text-slate-700 bg-black/40" @keyup.enter="addAccessRule" />
                                    </div>
                                </div>
                            </div>

                            <!-- Feedback de Verificación -->
                            <Transition enter-active-class="transition duration-300 ease-out" enter-from-class="opacity-0 -translate-y-2" leave-active-class="transition duration-200 ease-in" leave-from-class="opacity-100" leave-to-class="opacity-0 translate-y-2">
                                <div v-if="resolvedInfo" :key="resolvedInfo.error ? 'err' : 'ok'" class="px-2">
                                    <div v-if="resolvedInfo.error" class="flex items-center gap-4 p-5 bg-error/10 border border-error/20 rounded-2xl animate-shake">
                                        <div class="w-10 h-10 rounded-xl bg-error/20 flex items-center justify-center">
                                            <ShieldAlert class="w-5 h-5 text-error" />
                                        </div>
                                        <div>
                                            <span class="text-sm font-bold text-white uppercase tracking-tight block">Signal Rejected</span>
                                            <span class="text-xs font-medium text-error/80 uppercase tracking-tighter">{{ resolvedInfo.message }}</span>
                                        </div>
                                    </div>
                                    <div v-else class="flex items-center justify-between p-5 bg-success/5 border border-success/20 rounded-2xl">
                                        <div class="flex items-center gap-4">
                                            <div class="w-10 h-10 rounded-xl bg-success/20 flex items-center justify-center">
                                                <CheckCircle2 class="w-5 h-5 text-success" />
                                            </div>
                                            <div class="flex flex-col">
                                                <span class="text-sm font-bold text-white uppercase tracking-tight">{{ resolvedInfo.name || 'TRUSTED SIGNAL FOUND' }}</span>
                                                <span class="text-xs font-mono text-success/60">{{ resolvedInfo.resolved_identity }}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </Transition>

                            <button @click="addAccessRule" class="btn h-14 w-full rounded-2xl text-sm font-bold uppercase tracking-[0.2em] shadow-2xl transition-all border-none group/btn overflow-hidden relative cursor-pointer" 
                                :class="config.access_mode === 'private' ? 'btn-primary bg-primary hover:scale-[1.01]' : 'btn-error bg-red-600 hover:bg-red-500 text-white'"
                                :disabled="!newRule.identity || verifying">
                                <div class="absolute inset-0 bg-white/10 opacity-0 group-hover/btn:opacity-100 transition-opacity"></div>
                                <span v-if="!verifying" class="flex items-center justify-center gap-3 relative z-10">
                                    <Plus v-if="config.access_mode === 'private'" class="w-4 h-4" />
                                    <ShieldAlert v-else class="w-4 h-4" />
                                    {{ config.access_mode === 'private' ? 'Grant Access' : 'Apply Block' }}
                                </span>
                                <span v-else class="flex items-center gap-3 relative z-10">
                                    <Loader2 class="w-5 h-5 animate-spin" /> Analyzing...
                                </span>
                            </button>
                        </div>
                    </div>
                </Transition>

                <div v-if="filteredRules.length > 0" class="grid grid-cols-1 gap-4 max-h-96 overflow-y-auto pr-2 custom-scrollbar p-1">
                    <div v-for="rule in filteredRules" :key="rule.id" class="flex items-center justify-between p-5 bg-black/40 hover:bg-white/[0.04] border border-white/5 rounded-[1.5rem] group transition-all">
                        <div class="flex items-center gap-5">
                            <div class="px-4 py-2 rounded-xl text-xs font-bold uppercase tracking-widest shadow-inner ring-1" :class="rule.action === 'ALLOW' ? 'bg-success/10 text-success ring-success/20' : 'bg-error/10 text-error ring-error/20'">
                                {{ rule.action === 'ALLOW' ? 'TRUSTED' : 'BANNED' }}
                            </div>
                            <div>
                                <div class="text-sm font-bold text-white uppercase tracking-tight">{{ rule.label || 'Unlabeled Identity' }}</div>
                                <div class="text-xs font-mono text-slate-500 mt-0.5 tracking-wider">{{ rule.identity }}</div>
                            </div>
                        </div>
                        <button @click="deleteAccessRule(rule.id)" class="opacity-0 group-hover:opacity-100 btn btn-ghost btn-xs text-slate-500 hover:text-error transition-all p-2 h-10 w-10 cursor-pointer">
                            <X class="w-5 h-5" />
                        </button>
                    </div>
                </div>
                <div v-else class="py-20 text-center bg-black/20 rounded-[2.5rem] border border-dashed border-white/5">
                    <div class="mb-4 inline-flex p-4 bg-white/[0.02] rounded-full">
                        <ShieldAlert class="w-8 h-8 text-slate-800" />
                    </div>
                    <p class="text-xs font-bold text-slate-600 uppercase tracking-[0.2em]">Automatic Intelligence is monitoring signals...</p>
                </div>
            </div>
        </section>
      </div>

      <!-- TAB: Subscribers -->
      <div v-if="activeTab === 'subscribers'" class="space-y-10">
          <header class="flex items-center justify-between">
              <div>
                <h3 class="text-2xl font-black text-white uppercase tracking-tight">Active Subscribers</h3>
                <p class="text-xs text-slate-500 font-medium uppercase tracking-widest mt-1">Clients with exclusive bot assignments or custom prompts</p>
              </div>
              <button @click="loadSubscribers" class="btn-premium btn-premium-ghost px-4 h-10" :disabled="loadingSubscribers">
                  <Activity class="w-4 h-4 mr-2" :class="{ 'animate-pulse': loadingSubscribers }" />
                  Sync
              </button>
          </header>

          <div v-if="loadingSubscribers" class="py-20 flex justify-center">
              <span class="loading loading-ring loading-lg text-primary"></span>
          </div>

          <div v-else-if="subscribers.length === 0" class="py-24 text-center bg-black/10 rounded-[2rem] border border-dashed border-white/5">
              <Users class="w-10 h-10 text-slate-700 mx-auto mb-4 opacity-20" />
              <p class="text-xs font-bold text-slate-600 uppercase tracking-widest leading-loose">No Exclusive Subscriptions<br/>found for this channel manifest.</p>
          </div>

          <div v-else class="grid grid-cols-1 gap-4">
              <!-- Edit Form -->
              <div v-if="editingSub" class="animate-in slide-in-from-right-4 fade-in duration-300">
                  <div class="p-6 bg-black/40 border border-white/5 rounded-[2rem] space-y-6">
                      <div class="flex items-center justify-between">
                         <div class="flex items-center gap-3">
                             <div class="w-10 h-10 rounded-xl bg-primary/20 flex items-center justify-center">
                                 <Edit3 class="w-5 h-5 text-primary" />
                             </div>
                             <div>
                                 <h4 class="text-white font-bold uppercase tracking-wide">Adjust Subscription</h4>
                                 <p class="text-xs text-slate-500 font-mono">CLIENT: {{ editingSub.client.display_name }}</p>
                             </div>
                         </div>
                         <button @click="editingSub = null" class="btn btn-ghost btn-sm rounded-lg hover:bg-white/5">
                             <X class="w-5 h-5" />
                         </button>
                      </div>

                      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                           <div class="form-control w-full">
                                <ResourceSelector
                                    v-model="editForm.custom_bot_id"
                                    :items="bots"
                                    label="Override Bot"
                                    placeholder="Search specific bot..."
                                    iconType="bot"
                                    resourceLabel="Override Active"
                                    :nullable="true"
                                    color="indigo"
                                />
                            </div>
                            <div class="form-control w-full">
                                <label class="label-premium">Subscription Expiry</label>
                                <input v-model="editForm.expires_at" type="date" class="input-premium h-14 w-full font-bold" />
                            </div>
                      </div>

                      <div class="py-4">
                        <SessionTimers 
                            v-model:timeout="editForm.session_timeout"
                            v-model:warning="editForm.inactivity_warning_time"
                            :isOverride="true"
                            :inheritedTimeout="config.session_timeout || 4"
                            :inheritedWarning="config.inactivity_warning_time || 3"
                        />
                      </div>

                      <div class="py-2">
                        <HistoryLimitConfig 
                            v-model="editForm.max_history_limit" 
                            :isOverride="true"
                        />
                      </div>


                      <div class="form-control w-full">
                            <label class="label-premium">System Prompt Override</label>
                            <textarea v-model="editForm.custom_system_prompt" class="textarea textarea-bordered bg-black/40 border-white/5 focus:border-primary/40 text-sm h-32 w-full resize-none transition-all p-4" placeholder="Enter custom behavioral instructions just for this client..."></textarea>
                      </div>

                      <div class="flex justify-end gap-3 pt-4 border-t border-white/5">
                          <button @click="editingSub = null" class="btn btn-ghost">Cancel</button>
                          <button @click="saveSubscription" class="btn btn-primary px-8">Save Changes</button>
                      </div>
                  </div>
              </div>

              <!-- List View -->
              <div v-else v-for="item in subscribers" :key="item.subscription.id" 
                   class="p-6 bg-white/[0.02] border border-white/5 rounded-[2rem] flex flex-col gap-6 hover:bg-white/[0.04] transition-all group relative">
                  
                  <div class="flex flex-col lg:flex-row justify-between lg:items-center gap-6">
                      <div class="flex items-center gap-5">
                          <div class="w-14 h-14 rounded-2xl bg-black/40 flex items-center justify-center ring-1 ring-white/10 group-hover:ring-primary/30 transition-all shadow-xl font-black text-slate-500 uppercase">
                              {{ item.client.display_name?.charAt(0) || 'C' }}
                          </div>
                          <div>
                              <div class="flex items-center gap-2 mb-1">
                                  <h4 class="text-base font-black text-white uppercase tracking-tight">{{ item.client.display_name }}</h4>
                                  <span class="px-2 py-0.5 rounded-lg text-[8px] font-black uppercase tracking-widest border"
                                        :class="item.client.tier === 'vip' ? 'bg-amber-500/10 text-amber-500 border-amber-500/20' : item.client.tier === 'premium' ? 'bg-primary/10 text-primary border-primary/20' : 'bg-slate-500/10 text-slate-500 border-white/10'">
                                      {{ item.client.tier }}
                                  </span>
                              </div>
                              <div class="flex items-center gap-3">
                                  <span class="text-xs font-mono text-slate-500 tracking-tighter">{{ item.client.platform_id }}</span>
                                  <div v-if="item.client.enabled" class="flex items-center gap-1.5">
                                      <div class="w-1 h-1 rounded-full bg-success"></div>
                                      <span class="text-[9px] font-black text-success uppercase tracking-widest">Active</span>
                                  </div>
                              </div>
                          </div>
                      </div>
                      
                      <!-- Action Buttons (Visible on Hover/Focus) -->
                      <div class="flex items-center gap-2 opacity-100 lg:opacity-0 lg:group-hover:opacity-100 transition-opacity">
                            <button @click="startEditSubscription(item)" class="btn btn-sm btn-square btn-ghost hover:bg-white/10 text-slate-400 hover:text-white" title="Edit Subscription">
                                <Edit3 class="w-4 h-4" />
                            </button>
                            <button @click="removeSubscription(item)" class="btn btn-sm btn-square btn-ghost hover:bg-red-500/10 text-slate-400 hover:text-red-500" title="Revoke Subscription">
                                <Trash2 class="w-4 h-4" />
                            </button>
                      </div>
                  </div>

                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div class="storage-box-premium m-0 py-3 bg-black/20">
                           <div class="flex items-center gap-2 mb-1">
                               <Bot class="w-3 h-3 text-primary" />
                               <span class="text-[9px] font-black text-slate-500 uppercase tracking-widest">Custom Bot Engine</span>
                           </div>
                           <span class="text-xs font-mono" :class="item.subscription.custom_bot_id ? 'text-indigo-400' : 'text-slate-600'">
                               {{ getBotName(item.subscription.custom_bot_id) }}
                           </span>
                      </div>
                      <div class="storage-box-premium m-0 py-3 bg-black/20">
                           <div class="flex items-center gap-2 mb-1">
                               <Terminal class="w-3 h-3 text-amber-500" />
                               <span class="text-[9px] font-black text-slate-500 uppercase tracking-widest">System Prompt Override</span>
                           </div>
                           <span class="text-xs italic" :class="item.subscription.custom_system_prompt ? 'text-slate-300' : 'text-slate-700'">
                               {{ item.subscription.custom_system_prompt ? 'Configured (Exclusive)' : 'Standard Flow' }}
                           </span>
                      </div>
                  </div>

                  <div class="flex items-center gap-2 px-4 py-2 bg-black/40 rounded-xl border border-white/5 w-fit">
                      <Calendar class="w-3.5 h-3.5 text-slate-600" />
                      <div class="flex flex-col">
                          <span class="text-[8px] font-black text-slate-600 uppercase tracking-tighter leading-none mb-1">Expiry Date</span>
                          <span class="text-xs font-bold text-slate-400 uppercase tracking-widest">
                              {{ item.subscription.expires_at ? new Date(item.subscription.expires_at).toLocaleDateString() : 'INFINITE' }}
                          </span>
                      </div>
                  </div>
              </div>
          </div>
      </div>

      <!-- TAB: Integrations -->
      <div v-if="activeTab === 'integrations'" class="space-y-12">
        <header>
          <h3 class="text-2xl font-black text-white uppercase tracking-tight">Integrations</h3>
          <p class="text-xs text-slate-500 font-medium uppercase tracking-widest mt-1">Connect Az-Wap with external CRM and Monitoring systems</p>
        </header>

        <!-- Webhook Settings -->
        <section class="space-y-6">
            <h4 class="text-sm font-bold text-slate-500 uppercase tracking-[0.2em] border-l-2 border-primary pl-4">System Webhooks</h4>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6 p-6 bg-white/[0.02] rounded-2xl border border-white/5">
                <div class="form-control">
                    <label class="label-premium font-semibold">Callback Endpoint</label>
                    <input v-model="config.webhook_url" type="text" placeholder="https://your-api.com/events" class="input-premium text-base font-medium placeholder:text-slate-700" />
                </div>
                <div class="form-control">
                    <label class="label-premium font-semibold">Verification Secret</label>
                    <input v-model="config.webhook_secret" type="password" placeholder="••••••••" class="input-premium text-base font-medium" />
                </div>
                <p class="md:col-span-2 text-xs text-slate-600 font-medium uppercase px-2">Outgoing events (messages, status, presence) will be dispatched to this URL.</p>
            </div>
        </section>

        <!-- Chatwoot Integration -->
        <section class="p-8 rounded-[2rem] bg-gradient-to-br from-[#1f305e]/20 to-transparent border border-[#1f305e]/30 space-y-8 shadow-2xl shadow-indigo-500/5">
          <div class="flex items-center justify-between border-b border-white/5 pb-6">
            <div class="flex items-center gap-5">
                <div class="w-14 h-14 rounded-2xl bg-[#1f305e]/40 flex items-center justify-center ring-1 ring-white/10">
                    <HardDrive class="w-7 h-7 text-white" />
                </div>
                <div>
                   <h4 class="text-base font-bold uppercase tracking-widest text-white">Chatwoot Bridge</h4>
                   <p class="text-xs font-medium text-slate-500 uppercase mt-1">Sync conversations with Chatwoot CRM</p>
                </div>
            </div>
            <input type="checkbox" v-model="config.chatwoot.enabled" class="toggle toggle-primary toggle-md" />
          </div>

          <div v-if="config.chatwoot.enabled" class="space-y-8 animate-in fade-in zoom-in-95 duration-300">
              <div class="form-control">
                  <label class="label-premium font-semibold">Managed Credential</label>
                  <select v-model="config.chatwoot.credential_id" class="select-premium w-full text-base font-medium">
                      <option value="">(Enter Credentials Manually)</option>
                      <option v-for="cred in credentials" :key="cred.id" :value="cred.id">
                          {{ cred.name }}
                      </option>
                  </select>
              </div>

              <div v-if="!config.chatwoot.credential_id" class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div class="form-control"><label class="label-premium font-semibold">Access Token</label><input v-model="config.chatwoot.token" type="password" class="input-premium text-base font-medium" /></div>
                <div class="form-control"><label class="label-premium font-semibold">Core URL</label><input v-model="config.chatwoot.url" type="text" placeholder="https://chatwoot.instance.com" class="input-premium text-base font-medium placeholder:text-slate-700" /></div>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-6 border-t border-white/5 pt-6">
                <div class="form-control"><label class="label-premium font-semibold">Account ID</label><input v-model.number="config.chatwoot.account_id" type="number" class="input-premium font-mono text-base font-medium" /></div>
                <div class="form-control"><label class="label-premium font-semibold">Inbox ID</label><input v-model.number="config.chatwoot.inbox_id" type="number" class="input-premium font-mono text-base font-medium" /></div>
                <div class="form-control"><label class="label-premium font-semibold">Agent Bot Token</label><input v-model="config.chatwoot.bot_token" type="text" class="input-premium font-mono text-base font-medium" /></div>
                <div class="form-control"><label class="label-premium font-semibold">Inbox Identifier</label><input v-model="config.chatwoot.inbox_identifier" type="text" class="input-premium font-mono text-base font-medium" /></div>
              </div>

              <div class="p-6 bg-black/60 rounded-2xl border border-white/5 space-y-3">
                  <span class="text-xs font-bold text-primary uppercase tracking-widest px-1">Webhook Target for Chatwoot</span>
                  <div class="flex gap-2">
                      <input readonly :value="config.chatwoot.webhook_url" class="input-premium flex-1 opacity-50 font-mono text-sm font-medium" />
                      <button @click="copyWebhook" class="btn btn-primary h-12 px-6 rounded-xl text-xs font-bold uppercase tracking-widest cursor-pointer shadow-lg shadow-primary/20">Copy</button>
                  </div>
                  <p class="text-xs text-slate-600 font-medium uppercase px-1">Paste this URL in your Chatwoot Inbox settings to receive replies.</p>
              </div>
          </div>
        </section>
      </div>
    </AppTabModal>

  <ConfirmationDialog 
      v-model="confirmModal.show"
      :title="confirmModal.title"
      :message="confirmModal.message"
      :type="confirmModal.type"
      :confirmText="confirmModal.confirmText"
      @confirm="confirmModal.onConfirm"
  />
</template>

<style scoped>
.animate-scan {
    animation: scan 2s linear infinite;
}

@keyframes scan {
    0% { transform: translateY(-100%); opacity: 0; }
    50% { opacity: 1; }
    100% { transform: translateY(1000%); opacity: 0; }
}

.animate-shake {
    animation: shake 0.4s cubic-bezier(0.36, 0.07, 0.19, 0.97) both;
}

@keyframes shake {
    10%, 90% { transform: translate3d(-1px, 0, 0); }
    20%, 80% { transform: translate3d(2px, 0, 0); }
    30%, 50%, 70% { transform: translate3d(-4px, 0, 0); }
    40%, 60% { transform: translate3d(4px, 0, 0); }
}

.animate-bounce-subtle {
    animation: bounce-subtle 0.5s ease-out;
}

@keyframes bounce-subtle {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-4px); }
}
</style>

