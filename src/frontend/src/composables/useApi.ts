import { ref } from 'vue'

const getApiBaseUrl = () => {
    let url = localStorage.getItem('api_url')
    if (!url && typeof window !== 'undefined') {
        // If we are in Vite Dev Server (5173), default to backend on 3000
        if (window.location.port === '5173') {
            url = 'http://localhost:3000'
        } else {
            url = window.location.origin
        }
    }
    if (!url) url = 'http://localhost:3000'

    // Remove trailing slash if present
    url = url.replace(/\/$/, '')
    // Ensure it ends with /api for backend compatibility
    if (!url.endsWith('/api')) {
        url += '/api'
    }
    return url
}
const getAuthToken = () => localStorage.getItem('api_token')

export function useApi() {
  const loading = ref(false)
  const error = ref<string | null>(null)

  const login = async (user: string, pass: string): Promise<boolean> => {
      const token = btoa(`${user}:${pass}`)
      // Verify credentials by calling a protected endpoint (e.g. settings or health)
      try {
          // Temporarily store to test, or pass in header manually for this request
          const headers = { 'Authorization': `Basic ${token}` }
          // URL construction: getApiBaseUrl() ends with '/api', so we just append '/health/status'
          const res = await fetch(`${getApiBaseUrl()}/health/status`, { headers })
          if (res.ok) {
              localStorage.setItem('api_token', token)
              return true
          }
      } catch (e) {
          console.error(e)
      }
      return false
  }

  const logout = () => {
      localStorage.removeItem('api_token')
      if (typeof window !== 'undefined') {
          window.location.href = '/login'
      }
  }

  async function request(path: string, options: RequestInit = {}) {
    loading.value = true
    error.value = null
    
    const baseUrl = getApiBaseUrl()
    let cleanPath = path.startsWith('/') ? path : `/${path}`
    if (cleanPath.startsWith('/api/')) {
      cleanPath = cleanPath.substring(4)
    } else if (cleanPath === '/api') {
      cleanPath = '/'
    }
    const url = `${baseUrl}${cleanPath}`
    
    const isFormData = options.body instanceof FormData
    const headers: any = {
      'Authorization': `Basic ${getAuthToken()}`,
      ...options.headers,
    }

    if (!isFormData) {
      headers['Content-Type'] = 'application/json'
    }

    try {
      const response = await fetch(url, { ...options, headers })
      
      let data: any = null
      const contentType = response.headers.get('content-type')
      if (contentType && contentType.includes('application/json')) {
        const text = await response.text()
        data = text ? JSON.parse(text) : {}
      } else {
        data = {}
      }
      
      if (!response.ok) {
        // Create an error object that mimics axios structure so components can read err.response.data
        const err: any = new Error(data?.message || data?.error || `API Error ${response.status}`)
        err.response = {
            data: data,
            status: response.status,
            statusText: response.statusText
        }
        throw err
      }
      
      return data
    } catch (err: any) {
      error.value = err.message
      throw err
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    error,
    get: (path: string) => request(path, { method: 'GET' }),
    post: (path: string, body: any) => request(path, { 
      method: 'POST', 
      body: body instanceof FormData ? body : JSON.stringify(body) 
    }),
    put: (path: string, body: any) => request(path, { 
      method: 'PUT', 
      body: body instanceof FormData ? body : JSON.stringify(body) 
    }),
    delete: (path: string) => request(path, { method: 'DELETE' }),
    login,
    logout
  }
}
