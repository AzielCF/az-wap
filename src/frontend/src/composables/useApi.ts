import { ref } from 'vue'

const getApiBaseUrl = () => localStorage.getItem('api_url') || 'http://localhost:3000'
const getAuthToken = () => localStorage.getItem('api_token') || btoa('user1:pass1')

export function useApi() {
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function request(path: string, options: RequestInit = {}) {
    loading.value = true
    error.value = null
    
    const baseUrl = getApiBaseUrl()
    const url = `${baseUrl}${path.startsWith('/') ? path : `/${path}`}`
    
    const headers = {
      'Authorization': `Basic ${getAuthToken()}`,
      'Content-Type': 'application/json',
      ...options.headers,
    }

    try {
      const response = await fetch(url, { ...options, headers })
      
      let data: any = null
      const contentType = response.headers.get('content-type')
      if (contentType && contentType.includes('application/json')) {
        const text = await response.text()
        if (text) {
          data = JSON.parse(text)
        }
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
    post: (path: string, body: any) => request(path, { method: 'POST', body: JSON.stringify(body) }),
    put: (path: string, body: any) => request(path, { method: 'PUT', body: JSON.stringify(body) }),
    delete: (path: string) => request(path, { method: 'DELETE' }),
  }
}
