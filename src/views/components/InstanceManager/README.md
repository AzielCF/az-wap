# InstanceManager - Componente Modular

## 📁 Estructura

```
InstanceManager/
├── index.js                    # Orquestador principal (6.7 KB)
├── utils.js                    # Funciones auxiliares (1.9 KB)
├── GlobalIASettings.js         # Configuración global de IA (4.6 KB)
├── CredentialManager.js        # Gestión de credenciales (11.7 KB)
├── BotManager.js               # Gestión de Bots AI (19.3 KB)
├── NewInstanceForm.js          # Formulario nueva instancia (3.5 KB)
├── InstanceList.js             # Lista de instancias (5.2 KB)
├── WebhookEditor.js            # Editor de webhooks (3.8 KB)
├── ChatwootEditor.js           # Modal Chatwoot (7.7 KB)
└── GeminiEditor.js             # Panel Gemini/IA (10.1 KB)
```

**Total:** 10 archivos modulares vs 1 archivo monolítico de 74.7 KB

## 🎯 Responsabilidades

### index.js (Orquestador)
- Coordina todos los subcomponentes
- Maneja el estado global del manager
- Controla qué editor está activo
- Emite eventos al componente padre

### utils.js
Funciones auxiliares compartidas:
- `showSuccessInfo()` / `showErrorInfo()`
- `chatwootWebhookUrl()` / `botWebhookUrl()`
- `copyToClipboard()`
- `filterCredentialsByKind()`
- `handleApiError()`

### GlobalIASettings.js
- Configuración global de IA (prompt y timezone)
- Aplica a todos los asistentes en todas las instancias
- Métodos: `loadGlobalGeminiPrompt()`, `saveGlobalGeminiPrompt()`

### CredentialManager.js
- CRUD de credenciales (Gemini y Chatwoot)
- Modal de edición de credenciales
- Métodos: `loadCredentials()`, `saveCredential()`, `deleteCredential()`
- Emite: `credentials-loaded`

### BotManager.js
- CRUD de Bots AI reutilizables
- Modal de edición de bots
- Métodos: `loadBots()`, `saveBot()`, `deleteBot()`, `clearBotMemory()`
- Emite: `bots-loaded`

### NewInstanceForm.js
- Formulario para crear nueva instancia
- Muestra el token generado
- Método: `createInstance()`
- Emite: `refresh-instances`, `set-active-token`

### InstanceList.js
- Tabla de instancias existentes
- Botones de acción (Use, Delete, Webhooks, Chatwoot, IA)
- Métodos: `deleteInstance()`, `useInstance()`
- Emite: `open-webhook-editor`, `open-chatwoot-editor`, `open-gemini-editor`

### WebhookEditor.js
- Panel de edición de webhooks
- Configuración de URLs, secret y TLS
- Métodos: `save()`, `cancel()`
- Emite: `refresh-instances`, `cancel`

### ChatwootEditor.js
- Modal de configuración de Chatwoot
- Integración con credenciales
- Métodos: `save()`, `cancel()`
- Emite: `cancel`

### GeminiEditor.js
- Panel de configuración de Gemini/IA
- Integración con Bots AI reutilizables
- Métodos: `save()`, `cancel()`, `clearMemory()`
- Emite: `refresh-instances`, `cancel`

## 🔄 Flujo de Datos

```
index.js (Orquestador)
    ├─→ GlobalIASettings (independiente)
    ├─→ CredentialManager ──→ credentials ──→ BotManager
    │                                      └─→ ChatwootEditor
    ├─→ NewInstanceForm ──→ refresh-instances
    ├─→ BotManager ──→ bots ──→ GeminiEditor
    ├─→ InstanceList ──→ eventos de edición
    ├─→ WebhookEditor (editingInstanceId)
    ├─→ ChatwootEditor (chatwootEditingInstanceId)
    └─→ GeminiEditor (geminiEditingInstanceId)
```

## 📊 Beneficios de la Refactorización

### Antes
- ❌ 1 archivo de 1305 líneas (74.7 KB)
- ❌ Difícil de mantener y navegar
- ❌ Mezcla de responsabilidades
- ❌ Difícil de testear

### Después
- ✅ 10 archivos modulares (~130 líneas promedio)
- ✅ Separación clara de responsabilidades
- ✅ Más fácil de mantener y testear
- ✅ Reutilización de código
- ✅ Mejor organización del proyecto

## 🚀 Uso

### Importar el componente principal

```javascript
import InstanceManager from './components/InstanceManager/index.js';

// En tu componente Vue
export default {
    components: {
        InstanceManager,
    },
    // ...
};
```

### Props del componente principal

```javascript
<InstanceManager 
    :instances="instances"
    :selectedToken="selectedToken"
    @set-active-token="handleSetActiveToken"
    @refresh-instances="handleRefreshInstances"
/>
```

## 🔧 Mantenimiento

Cada componente es ahora independiente y puede ser:
- Modificado sin afectar a otros
- Testeado de forma aislada
- Reutilizado en otros contextos
- Extendido con nuevas funcionalidades

## 📝 Notas

- Todos los componentes usan las funciones auxiliares de `utils.js`
- Los editores se comunican con el orquestador mediante eventos
- Solo un editor puede estar activo a la vez
- Las credenciales y bots se cargan una vez y se comparten entre componentes
