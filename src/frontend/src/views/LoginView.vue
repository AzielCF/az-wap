<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'

const router = useRouter()
const { login } = useApi()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  if (!username.value || !password.value) {
    error.value = 'Please enter both username and password'
    return
  }
  
  loading.value = true
  error.value = ''
  
  try {
    const success = await login(username.value, password.value)
    if (success) {
      router.push('/')
    } else {
      error.value = 'Invalid credentials'
    }
  } catch (err: any) {
    error.value = err.message || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-[#0b0e14] flex flex-col items-center justify-center p-4 relative overflow-hidden">
    <!-- Background Decor -->
    <div class="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
        <div class="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/5 rounded-full blur-[100px]"></div>
        <div class="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-blue-500/5 rounded-full blur-[100px]"></div>
    </div>

    <div class="w-full max-w-md bg-[#161a23] border border-white/5 rounded-[2rem] p-10 shadow-2xl relative z-10 animate-in fade-in zoom-in duration-500">
      <div class="text-center mb-10">
        <div class="w-20 h-20 rounded-[2rem] bg-primary/10 flex items-center justify-center text-primary mx-auto mb-6 border border-primary/20 shadow-lg shadow-primary/20">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
        </div>
        <h1 class="text-3xl font-black text-white uppercase tracking-tighter mb-2">Access Control</h1>
        <p class="text-xs text-slate-500 font-bold uppercase tracking-widest">Az-Wap Enterprise Engine</p>
      </div>

      <form @submit.prevent="handleLogin" class="space-y-6">
        <div class="space-y-2">
            <label class="text-xs font-black text-slate-500 uppercase tracking-widest ml-1">Identity</label>
            <input v-model="username" type="text" class="input-premium w-full h-14 bg-black/20 focus:bg-black/40" placeholder="Username" autofocus />
        </div>
        
        <div class="space-y-2">
            <label class="text-xs font-black text-slate-500 uppercase tracking-widest ml-1">Security Key</label>
            <input v-model="password" type="password" class="input-premium w-full h-14 bg-black/20 focus:bg-black/40" placeholder="Password" />
        </div>

        <div v-if="error" class="p-4 rounded-xl bg-error/10 border border-error/20 text-error text-xs font-bold text-center animate-in fade-in slide-in-from-top-2">
            {{ error }}
        </div>

        <button type="submit" class="btn-premium btn-premium-primary w-full h-14 text-sm mt-4" :disabled="loading">
            <span v-if="loading" class="loading loading-spinner"></span>
            <span v-else>Authenticate Session</span>
        </button>
      </form>

      <div class="mt-8 text-center">
        <p class="text-xs text-slate-600 font-mono">v2.0.0 Secure Environment</p>
      </div>
    </div>
  </div>
</template>
