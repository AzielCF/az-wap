<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useApi } from '@/composables/useApi'
import AppTabModal from '@/components/AppTabModal.vue'
import ClientIdentityTab from './tabs/ClientIdentityTab.vue'
import ClientContactTab from './tabs/ClientContactTab.vue'
import ClientSettingsTab from './tabs/ClientSettingsTab.vue'
import ClientAccessTab from './tabs/ClientAccessTab.vue'
import ClientWorkspacesManager from './ClientWorkspacesManager.vue'
import { 
  ShieldCheck, 
  Contact, 
  Layout, 
  Bot, 
  LayoutGrid, 
  Users 
} from 'lucide-vue-next'

const props = defineProps<{
  show: boolean
  clientToEdit: any | null
}>()

const emit = defineEmits(['update:show', 'saved', 'closed'])

const api = useApi()

const isVisible = computed({
  get: () => props.show,
  set: (val) => {
    emit('update:show', val)
    if (!val) {
      emit('closed')
    }
  }
})

const activeClientTab = ref('identity')
const isLoadingClient = ref(false)
const originalClientState = ref('')
const clientChannels = ref<any[]>([])
const bots = ref<any[]>([])

const newClient = ref({
  platform_id: '',
  platform_type: 'whatsapp',
  display_name: '',
  email: '',
  phone: '',
  tier: 'standard',
  tags: [] as string[],
  notes: '',
  language: 'en',
  timezone: '',
  country: '',
  allowed_bots: [] as string[],
  owned_channels: [] as string[],
  is_tester: false
})

const lidVerified = ref(false)
const validatingLid = ref(false)
const lastValidatedPhone = ref('')
const hasWorkspaceAlert = ref(false)

const hasChanges = computed(() => {
  if (!props.clientToEdit) {
    return newClient.value.platform_id.length > 0
  }
  return JSON.stringify(newClient.value) !== originalClientState.value
})

watch(() => props.show, async (newVal) => {
  if (newVal) {
    initializeModalScope()
  } else {
    activeClientTab.value = 'identity'
  }
})

async function initializeModalScope() {
  isLoadingClient.value = true
  
  if (props.clientToEdit) {
    newClient.value = {
      platform_id: props.clientToEdit.platform_id,
      platform_type: props.clientToEdit.platform_type,
      display_name: props.clientToEdit.display_name,
      email: props.clientToEdit.email || '',
      phone: props.clientToEdit.phone || '',
      tier: props.clientToEdit.tier,
      tags: props.clientToEdit.tags || [],
      notes: props.clientToEdit.notes || '',
      language: props.clientToEdit.language || 'en',
      timezone: props.clientToEdit.timezone || '',
      country: props.clientToEdit.country || '',
      allowed_bots: props.clientToEdit.allowed_bots || [],
      owned_channels: props.clientToEdit.owned_channels || [],
      is_tester: props.clientToEdit.is_tester || false
    }

    if (props.clientToEdit.platform_type === 'whatsapp' && props.clientToEdit.platform_id.includes('@lid')) {
      lidVerified.value = true
      lastValidatedPhone.value = props.clientToEdit.phone || ''
    } else {
      lidVerified.value = false
      lastValidatedPhone.value = ''
    }

    api.get(`/clients/${props.clientToEdit.id}/channels`).then((res: any) => {
      clientChannels.value = res || []
    }).catch((err: any) => {
      console.error('Failed to load client channels:', err)
      clientChannels.value = []
    })
  } else {
    newClient.value = {
      platform_id: '',
      platform_type: 'whatsapp',
      display_name: '',
      email: '',
      phone: '',
      tier: 'standard',
      tags: [],
      notes: '',
      language: 'en',
      timezone: '',
      country: '',
      allowed_bots: [],
      owned_channels: [],
      is_tester: false
    }
    lidVerified.value = false
    lastValidatedPhone.value = ''
    clientChannels.value = []
  }

  originalClientState.value = JSON.stringify(newClient.value)
  isLoadingClient.value = false

  // check for workspace revocations
  checkWorkspaceRevocations()

  // Load all bots so user can search and add any bot
  try {
    const bts = await api.get('/bots') as any
    bots.value = bts?.results || []
  } catch (err) {
    console.error('Failed to load bots', err)
    bots.value = []
  }
}

async function checkWorkspaceRevocations() {
  if (!props.clientToEdit) return
  try {
    const workspaces = await api.get(`/clients/${props.clientToEdit.id}/workspaces`) || []
    for (const ws of workspaces) {
      const guests = await api.get(`/clients/${props.clientToEdit.id}/workspaces/${ws.id}/guests`) || []
      const hasRevoked = guests.some((g: any) => {
        if (!g.bot_id) return false
        if (!newClient.value.allowed_bots || newClient.value.allowed_bots.length === 0) return false
        return !newClient.value.allowed_bots.includes(g.bot_id)
      })
      if (hasRevoked) {
        hasWorkspaceAlert.value = true
        return
      }
    }
    hasWorkspaceAlert.value = false
  } catch (e) {
    console.error(e)
  }
}

async function extractLid() {
  const rawInput = newClient.value.platform_id || newClient.value.phone
  if (!rawInput) {
    alert('Please enter a phone number first')
    return
  }

  if (rawInput.includes('@')) {
    alert('Please enter the actual phone number (without @lid or @s.whatsapp.net). The system will resolve the LID for you.')
    return
  }

  const phoneNumber = rawInput.replace(/[^\d]/g, '')
  if (!phoneNumber || phoneNumber.length < 8) {
    alert('Please enter a valid phone number')
    return
  }

  const resWs = await api.get('/workspaces') as any
  const workspaces = resWs || []

  let targetWorkspace = null
  let targetChannel = null

  for (const ws of workspaces) {
    const ch = ws.channels?.find((c: any) => (c.type === 'whatsapp' || c.platform_type === 'whatsapp') && (c.enabled || c.status === 'connected'))
    if (ch) {
      targetWorkspace = ws
      targetChannel = ch
      break
    }
  }

  if (!targetChannel) {
    alert('No active WhatsApp channel found (Enabled or Connected) to perform validation.')
    return
  }

  validatingLid.value = true
  try {
    const res = await api.get(`/workspaces/${targetWorkspace.id}/channels/${targetChannel.id}/resolve-identity?identity=${phoneNumber}`) as any
    if (res.resolved_identity) {
      newClient.value.platform_id = res.resolved_identity
      lidVerified.value = res.status === 'verified'
      newClient.value.phone = phoneNumber
      lastValidatedPhone.value = phoneNumber
      if (!newClient.value.display_name && res.name) {
        newClient.value.display_name = res.name
      }
    }
  } catch (err) {
    alert('Could not resolve LID. Make sure the channel is connected and the number is registered on WhatsApp.')
  } finally {
    validatingLid.value = false
  }
}

async function saveClient() {
  try {
    if (props.clientToEdit) {
      await api.put(`/clients/${props.clientToEdit.id}`, newClient.value)
    } else {
      await api.post('/clients', newClient.value)
    }
    isVisible.value = false
    emit('saved')
  } catch (err: any) {
    if (err.status === 409) {
      alert('CONFLICTO: Este ID ya está siendo usado por otro cliente. Busca el cliente duplicado y elimínalo primero.')
    } else {
      alert('Error saving client: ' + (err.message || 'Unknown error'))
    }
  }
}
</script>

<template>
  <AppTabModal 
      v-model="isVisible"
      :title="clientToEdit ? 'Edit Client' : 'New Client'"
      v-model:activeTab="activeClientTab"
      :saveDisabled="!hasChanges"
      :tabs="[
          { id: 'identity', label: 'Identity', icon: ShieldCheck as any },
          { id: 'personal', label: 'Contact', icon: Contact as any },
          { id: 'settings', label: 'Settings', icon: Layout as any },
          { id: 'operational', label: 'Access', icon: Bot as any },
          { id: 'workspaces', label: 'Workspaces', icon: LayoutGrid as any, alert: hasWorkspaceAlert }
      ]"
      :identity="clientToEdit ? {
          name: (newClient.display_name || 'Anonymous Object'),
          id: newClient.platform_id,
          icon: Users as any,
          iconType: 'component'
      } : undefined"
      saveText="Save"
      @save="saveClient"
      @cancel="isVisible = false"
  >
      <!-- Tab: Identity -->
      <ClientIdentityTab 
          v-if="activeClientTab === 'identity'" 
          v-model="newClient" 
          :editingClient="clientToEdit"
          :lidVerified="lidVerified"
          :validatingLid="validatingLid"
          :lastValidatedPhone="lastValidatedPhone"
          @extractLid="extractLid" 
      />

      <!-- Tab: Personal -->
      <ClientContactTab 
          v-if="activeClientTab === 'personal'" 
          v-model="newClient" 
      />

      <!-- Tab: Settings -->
      <ClientSettingsTab 
          v-if="activeClientTab === 'settings'" 
          v-model="newClient" 
      />

      <!-- Tab: Operational -->
      <ClientAccessTab 
          v-if="activeClientTab === 'operational'" 
          v-model="newClient"
          :editingClient="clientToEdit"
          :clientChannels="clientChannels"
          :bots="bots"
      />

      <!-- Tab: Workspaces -->
      <div v-if="activeClientTab === 'workspaces'" class="h-full flex flex-col overflow-hidden">
          <ClientWorkspacesManager 
             v-if="clientToEdit"
             :clientId="clientToEdit.id" 
             :allowedBots="newClient.allowed_bots"
             :bots="bots"
             :clientChannels="clientChannels"
          />
      </div>
  </AppTabModal>
</template>
