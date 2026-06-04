<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Send, User, Bot, X } from 'lucide-vue-next'

const props = defineProps<{
    channel: any,
    workspaceId: string
}>()

const emit = defineEmits(['close'])

const messages = ref<any[]>([])
const inputText = ref('')
const isTyping = ref(false)
const socket = ref<WebSocket | null>(null)
const messagesContainer = ref<HTMLElement | null>(null)

const getWsUrl = () => {
    let url = localStorage.getItem('api_url')
    if (!url && typeof window !== 'undefined') {
        if (window.location.port === '5173') {
            url = 'http://localhost:3000'
        } else {
            url = window.location.origin
        }
    }
    if (!url) url = 'http://localhost:3000'
    url = url.replace(/\/$/, '').replace(/^http/, 'ws')
    
    // Ensure the URL correctly points to the API group
    if (!url.endsWith('/api')) {
        url += '/api'
    }
    const token = localStorage.getItem('api_token')
    return `${url}/ws/simulator/${props.channel.id}?token=${token}`
}

const connectWebSocket = () => {
    const wsUrl = getWsUrl()
    socket.value = new WebSocket(wsUrl)

    socket.value.onopen = () => {
        messages.value.push({ text: 'System: Test session started.', sender: 'system' })
    }

    socket.value.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data)
            if (data.type === 'message') {
                isTyping.value = false
                messages.value.push({ text: data.text, sender: 'bot' })
                scrollToBottom()
            } else if (data.type === 'presence') {
                if (data.is_typing) {
                    isTyping.value = true
                    scrollToBottom()
                } else {
                    isTyping.value = false
                }
            } else if (data.type === 'read') {
                // Ignore visual read for now
            }
        } catch (e) {
            console.error('Error parsing WS message', e)
        }
    }

    socket.value.onclose = () => {
        messages.value.push({ text: 'System: Disconnected.', sender: 'system' })
        isTyping.value = false
    }
    
    socket.value.onerror = (error) => {
        messages.value.push({ text: 'System: Connection Error.', sender: 'system' })
        isTyping.value = false
    }
}

const sendMessage = () => {
    if (!inputText.value.trim() || !socket.value || socket.value.readyState !== WebSocket.OPEN) return

    const text = inputText.value.trim()
    messages.value.push({ text, sender: 'user' })
    scrollToBottom()

    socket.value.send(JSON.stringify({ type: 'message', text }))
    inputText.value = ''
}

const scrollToBottom = () => {
    nextTick(() => {
        if (messagesContainer.value) {
            messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
        }
    })
}

onMounted(() => {
    connectWebSocket()
})

onUnmounted(() => {
    if (socket.value) {
        socket.value.close()
    }
})
</script>

<template>
    <div class="flex flex-col h-full bg-[#0b0e14] text-slate-300">
        <!-- Header -->
        <div class="px-6 py-4 bg-[#161a23] border-b border-white/5 flex justify-between items-center shrink-0 shadow-lg z-10">
            <div class="flex items-center gap-4">
                <div class="w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center text-primary ring-1 ring-primary/30 relative">
                    <Bot class="w-5 h-5" />
                    <div class="absolute bottom-0 right-0 w-3 h-3 bg-success rounded-full ring-2 ring-[#161a23]"></div>
                </div>
                <div>
                    <h3 class="font-black uppercase tracking-widest text-white leading-none mb-1">{{ channel.name }}</h3>
                    <span class="text-xs font-bold text-slate-500 uppercase tracking-widest">Simulator Test Mode</span>
                </div>
            </div>
        </div>

        <!-- Chat Area -->
        <div class="flex-1 overflow-y-auto p-6 space-y-4 custom-scrollbar" ref="messagesContainer">
            <div v-for="(msg, index) in messages" :key="index" 
                 class="chat" 
                 :class="msg.sender === 'user' ? 'chat-end' : (msg.sender === 'bot' ? 'chat-start' : 'text-center opacity-50 text-xs font-bold uppercase tracking-widest my-4')">
                 
                <!-- System messages -->
                <span v-if="msg.sender === 'system'">{{ msg.text }}</span>
                
                <!-- Normal messages -->
                <template v-else>
                    <div class="chat-image avatar">
                        <div class="w-8 h-8 rounded-full bg-[#161a23] flex items-center justify-center border border-white/10 shadow-lg" :class="msg.sender === 'bot' ? 'text-primary' : 'text-slate-400'">
                            <Bot v-if="msg.sender === 'bot'" class="w-4 h-4" />
                            <User v-else class="w-4 h-4" />
                        </div>
                    </div>
                    <div class="chat-bubble font-medium shadow-xl" :class="msg.sender === 'user' ? 'bg-primary/20 text-white border-primary/30 border' : 'bg-[#161a23] text-slate-300 border border-white/5'">
                        <div class="whitespace-pre-wrap">{{ msg.text }}</div>
                    </div>
                </template>
            </div>

            <!-- Typing indicator -->
            <div v-if="isTyping" class="chat chat-start animate-in fade-in slide-in-from-bottom-2 duration-300">
                <div class="chat-image avatar">
                    <div class="w-8 h-8 rounded-full bg-[#161a23] flex items-center justify-center border border-white/10 text-primary shadow-lg">
                        <Bot class="w-4 h-4" />
                    </div>
                </div>
                <div class="chat-bubble bg-[#161a23] border border-white/5 shadow-xl flex items-center gap-1 px-4 py-4">
                    <div class="w-1.5 h-1.5 rounded-full bg-primary/60 animate-bounce" style="animation-delay: 0ms"></div>
                    <div class="w-1.5 h-1.5 rounded-full bg-primary/60 animate-bounce" style="animation-delay: 150ms"></div>
                    <div class="w-1.5 h-1.5 rounded-full bg-primary/60 animate-bounce" style="animation-delay: 300ms"></div>
                </div>
            </div>
        </div>

        <!-- Input Area -->
        <div class="p-4 bg-[#161a23] border-t border-white/5 shrink-0 shadow-[0_-10px_30px_rgba(0,0,0,0.5)] z-10">
            <form @submit.prevent="sendMessage" class="flex gap-2">
                <input 
                    v-model="inputText" 
                    type="text" 
                    placeholder="Type your message to test..." 
                    class="input-premium flex-1 h-12 bg-[#0b0e14] border-white/5 focus:border-primary/50 text-sm font-medium"
                />
                <button type="submit" class="btn-premium btn-premium-primary btn-premium-square h-12 w-12" :disabled="!inputText.trim()">
                    <Send class="w-4 h-4 ml-[-2px]" />
                </button>
            </form>
        </div>
    </div>
</template>
