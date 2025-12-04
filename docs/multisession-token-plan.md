# Plan de multi‑sesiones con token (tipo Evolution API)

## Objetivo

Permitir gestionar **cientos de sesiones de WhatsApp** en un único servidor, donde cada sesión se identifica por un **token** (similar a Evolution API) y tiene:

- **Nombre** legible (`name`)
- **Token secreto** (`token`)
- Su propia **base de datos de WhatsApp** y **chatstorage**
- Rutas HTTP reutilizando la API actual pero "escopadas" por token

---

## API propuesta

### 1. Gestión de instancias

- [x] `POST /instances`
  - Crea una nueva instancia
  - Body: `{ "name": "mi-bot-ventas" }`
  - Respuesta: `{ id, name, token, status }`
- [x] `GET /instances`
  - Lista todas las instancias (sin exponer el token completo, opcional)
- [ ] `DELETE /instances/:id`
  - Elimina una instancia específica (logout + cierre de conexiones + borrado físico de sus ficheros SQLite de WhatsApp y chatstorage mediante `os.Remove`; **nunca** hace `DROP`/`DELETE` masivo sobre tablas)

> Persistencia inicial: `storages/instances.json` (simple) para restaurar instancias tras reinicio.

### 2. Autenticación por token en la API existente

- Cabecera propuesta: `X-Instance-Token: <token>`
- Reutilizar rutas actuales, pero ligadas a una instancia:
  - `GET /app/login`
  - `GET /app/login-with-code`
  - `GET /app/logout`
  - `GET /app/reconnect`
  - `GET /app/devices`, etc.
- [x] Resolver `token → instancia → cliente de WhatsApp` en cada petición.

---

## Cambios internos en Go (alto nivel)

### 1. Instance Manager (token → instancia)

- [ ] Crear nueva estructura `Instance` (paquete a definir, p.ej. `infrastructure/instances`):
  - `ID string`
  - `Name string`
  - `Token string`
  - `DBURI string` (p.ej. `file:storages/wa-<ID>.db?_foreign_keys=on`)
  - `ChatDBURI string` (p.ej. `file:storages/chat-<ID>.db`)
  - `WAClient *whatsmeow.Client`
  - `WADB *sqlstore.Container`
  - `ChatStore domainChatStorage.IChatStorageRepository`
- [x] Mapa en memoria protegido por mutex:
  - `token -> *Instance`
- [ ] Funciones principales:
  - `CreateInstance(name string) (*Instance, error)`
  - `GetInstanceByToken(token string) (*Instance, error)`
  - `GetOrInitWAClient(ctx, inst *Instance) (*whatsmeow.Client, error)`

### 2. Inicialización por instancia (sin globals únicos)

Actualmente `InitWaDB` y `InitWaCLI` gestionan un **único cliente global** (`cli`, `db`, `keysDB`).

- [ ] Mantener `InitWaDB(ctx, DBURI string)` pero usarlo **por instancia** (cada instancia tiene su propio `DBURI`).
- [ ] Ajustar `InitWaCLI` para que:
  - No escriba en variables globales (`cli`, `db`, `keysDB`).
  - Devuelva `client`, `primaryDB`, `keysDB` y estos se asignen en la `Instance`.
- [ ] Definir convención de ficheros SQLite por instancia:
  - WhatsApp: `file:storages/wa-<instanceID>.db?_foreign_keys=on`
  - Chatstorage: `file:storages/chat-<instanceID>.db`

### 3. Usecases + capa REST usando token

#### Dominio (`domains/app/app.go`)

- [x] Extender `IAppUsecase` para recibir `token` (o `sessionID`) en todos los métodos relevantes:
  - `Login(ctx, token string) (LoginResponse, error)`
  - `LoginWithCode(ctx, token, phoneNumber string) (string, error)`
  - `Logout(ctx, token string) error`
  - `Reconnect(ctx, token string) error`
  - `FirstDevice(ctx, token string) (DevicesResponse, error)`
  - `FetchDevices(ctx, token string) ([]DevicesResponse, error)`

#### Usecase (`src/usecase/app.go`)

- [x] Reemplazar uso de `whatsapp.GetClient()` y `whatsapp.GetDB()` por:
  - Resolver instancia a partir del `token`.
  - Obtener/crear el cliente de WhatsApp específico de esa instancia.
- [x] Reutilizar la lógica existente de login/logout/reconnect pero aplicada al `client` de la instancia.

#### REST (`src/ui/rest/app.go`)

- [x] Leer el token desde la cabecera `X-Instance-Token` (y/o query `token`).
- [x] Validar que el token exista y mapearlo a una instancia.
- [x] Pasar el token al usecase.
- [x] Definir mensajes de error claros cuando falte o sea inválido el token.

### 4. Limpieza por instancia (logout y remote logout)

Ahora las funciones de limpieza soportan tanto el flujo global legacy como el flujo por instancia.

- [x] Adaptar estas funciones para trabajar **por instancia**:
  - Cerrar solo las conexiones (`WADB`) de la instancia.
  - Borrar solo sus ficheros de WhatsApp (`wa-<instanceID>.db`) mediante `os.Remove`, evitando operaciones `DROP TABLE` o `DELETE` masivas incluso si hay millones de mensajes.
  - [x] Evitar operaciones de borrado masivo por tabla en chatstorage durante los flujos de logout/cleanup (se mantiene un chatstorage global compartido; truncados adicionales deben hacerse de forma controlada fuera del flujo de logout).
- [x] Ajustar `handleRemoteLogout` y eventos relacionados para que actúen solo sobre la instancia correspondiente al `client` que dispara el evento cuando existe un `instanceID` en el contexto, manteniendo el flujo global legacy cuando no lo hay.

### 5. Propagación a otros módulos (send, message, group, etc.)

- [x] Revisar `ui/rest/send.go`, `ui/rest/message.go` y `ui/rest/group.go` para que también lean `X-Instance-Token`.
- [x] Pasar el token a sus respectivos usecases de `send`, `message` y `group`.
- [x] En los usecases de `send`, `message` y `group`, usar el `Instance Manager`/`appUsecase` para obtener el cliente correcto.

### 6. Escalabilidad (cientos de sesiones)

- [ ] Definir política de conexión:
  - Mantener clientes desconectados cuando no se usan.
  - Conectar solo al enviar mensajes o hacer operaciones críticas.
- [ ] Definir límites razonables (número máximo de instancias, tamaño de BDs, etc.).
- [ ] (Opcional) Añadir endpoints de métricas / healthcheck por instancia.

### 7. UI / panel de instancias

- [x] Añadir una nueva vista en la interfaz web para gestionar instancias (panel tipo Evolution API):
  - Listar instancias existentes (id, name, status).
  - Crear nuevas instancias introduciendo al menos `name` (opcionalmente otros metadatos).
  - Mostrar el `token` solo en el momento de creación (o con endpoint dedicado protegido).
- [x] Definir rutas HTTP para la vista de instancias (p.ej. `/instances/ui` o sección dentro del index principal).
- [x] Reutilizar el sistema actual de plantillas HTML (`EmbedViews`) para la nueva vista.
- [x] Conectar la UI con los endpoints REST de instancias (`POST /instances`, `GET /instances`, y opcionalmente `DELETE /instances/:id`).

---

## Orden sugerido de implementación

1. [x] Implementar **Instance Manager** mínimo en memoria + endpoints `POST /instances` y `GET /instances`.
2. [x] Adaptar `app` (usecase + REST) para trabajar con `X-Instance-Token` y un cliente por instancia.
3. [ ] Propagar el uso del token a las rutas de `send`, `message`, `group`, etc.
4. [ ] Migrar la limpieza a nivel de instancia.
5. [ ] Optimizar para cientos de sesiones (política de conexión, métricas, etc.).
