<!-- markdownlint-disable MD041 -->
<!-- markdownlint-disable-next-line MD033 -->
<div align="center">
  <!-- markdownlint-disable-next-line MD033 -->
  <img src="src/views/assets/azwap.svg" alt="AzWap Logo" width="200" height="200">

## AzWap (Gowa Fork) - Multi-instance WhatsApp gateway with native Chatwoot & AI agents support

</div>


___

![release version](https://img.shields.io/github/v/release/AzielCF/az-wap)
![Build Image](https://github.com/AzielCF/az-wap/actions/workflows/build-docker-image.yaml/badge.svg)
![Binary Release](https://github.com/AzielCF/az-wap/actions/workflows/release.yml/badge.svg)

## Support for `ARM` & `AMD` Architecture along with `MCP` Support

---

### 🚀 High-Scale & Enterprise Architecture (v2)

AzWap v2 has been re-engineered for production environments requiring high availability and massive session management. It is prepared to handle **hundreds of concurrent WhatsApp sessions** on a single instance thanks to its specialized concurrency engine.

#### 🏗️ Dual-Worker Pool System
Unlike standard gateways that spawn a new process for every message (causing CPU/RAM explosions), AzWap uses a **Fixed-Size Worker Pool** strategy:

1.  **Message Worker Pool**: Manages the core message loop (AI processing, webhooks, and Chatwoot sync). It uses a sharding algorithm to ensure **Sequential Consistency**: messages from the same chat are always processed in order, while messages from different chats are processed in parallel.
2.  **Bot Webhook Pool**: A dedicated secondary pool just for the Bot Webhook API. This isolates your LLM/AI logic from the WhatsApp core, preventing a slow AI response from affecting message delivery.

#### 🤖 AI Agents with MCP Client Support
AzWap now allows your AI Bots to have "hands" via the **Model Context Protocol (MCP)**.
- **Dynamic Tooling**: You can link one or multiple MCP Servers to a single Bot.
- **Enhanced Capabilities**: Your Bots are no longer limited to chat; they can now perform Google searches, query databases, or interact with any external service that provides an MCP interface.
- **Remote & SSE Support**: Connect to MCP servers running locally or remotely via SSE (Server-Sent Events) with full encryption for custom headers and auth.

---

Download:

- [Release](https://github.com/AzielCF/az-wap/releases/latest)
- [GitHub Container Registry](https://github.com/AzielCF/az-wap/pkgs/container/az-wap)

## Wiki / Documentation

- [GitHub Wiki](https://github.com/AzielCF/az-wap/wiki)
- Key pages:
  - [Multi-Instance Configuration](https://github.com/AzielCF/az-wap/wiki/Multi-Instance-Configuration)
  - [Chatwoot Integration](https://github.com/AzielCF/az-wap/wiki/Chatwoot-Integration)
  - [Bot System](https://github.com/AzielCF/az-wap/wiki/Bot-System)

## Support n8n package (n8n.io)

- [n8n package](https://www.npmjs.com/package/@aldinokemal2104/n8n-nodes-gowa)
- Go to Settings -> Community Nodes -> Input `@aldinokemal2104/n8n-nodes-gowa` -> Install

## Breaking Changes

- `v6`
  - For REST mode, you need to run `<binary> rest` instead of `<binary>`
    - for example: `./whatsapp rest` instead of ~~./whatsapp~~
  - For MCP mode, you need to run `<binary> mcp`
    - for example: `./whatsapp mcp`
- `v7`
  - Starting version 7.x we are using goreleaser to build the binary, so you can download the binary
      from [release](https://github.com/AzielCF/az-wap/releases/latest)

## Feature

- Send WhatsApp message via http API, [docs/openapi.yml](./docs/openapi.yaml) for more details
- **Multi-instance Support** - Run multiple WhatsApp sessions/accounts at the same time
- **MCP (Model Context Protocol) Server Support** - Integrate with AI agents and tools using standardized protocol
- **AI Agents Support** - Ready to be used by AI agents through the built-in MCP server and native Bot/LLM integrations
- **Native Chatwoot Integration** - Sync contacts and conversations with Chatwoot for customer support workflows
- Mention someone
  - `@phoneNumber`
  - example: `Hello @628974812XXXX, @628974812XXXX`
- Post Whatsapp Status
- **Send Stickers** - Automatically converts images to WebP sticker format
  - Supports JPG, JPEG, PNG, WebP, and GIF formats
  - Automatic resizing to 512x512 pixels
  - Preserves transparency for PNG images
- Compress image before send
- Compress video before send
- Change OS name become your app (it's the device name when connect via mobile)
  - `--os=Chrome` or `--os=MyApplication`
- Basic Auth (able to add multi credentials)
  - `--basic-auth=kemal:secret,toni:password,userName:secretPassword`, or you can simplify
  - `-b=kemal:secret,toni:password,userName:secretPassword`
- Subpath deployment support
  - `--base-path="/gowa"` (allows deployment under a specific path like `/gowa/sub/path`)
- Customizable port and debug mode
  - `--port 8000`
  - `--debug true`
- Auto reply message
  - `--autoreply="Don't reply this message"`
- Auto mark read incoming messages
  - `--auto-mark-read=true` (automatically marks incoming messages as read)
- Auto download media from incoming messages
  - `--auto-download-media=false` (disable automatic media downloads, default: `true`)
- **Webhook Payload Documentation**
- **Reactive Health Monitoring**: Real-time status updates for all entities.
- **Worker Pool Dashboard**: Real-time monitoring of message processing.
- **Chatwoot Webhook (Inbound)**
  Configure Chatwoot to send webhooks to AzWap:

  - `POST http://localhost:3000/instances/{instance_id}/chatwoot/webhook`

  **Security (recommended):**

  This endpoint is intentionally public (Chatwoot usually doesn't send custom auth headers by default). To avoid random
  callers forging events, you can protect it per-instance using the instance `webhook_secret`.

  If `webhook_secret` is set for the instance, AzWap will require one of:

  - `POST /instances/{instance_id}/chatwoot/webhook?token=<webhook_secret>`
  - Header `X-Webhook-Token: <webhook_secret>`

  If `webhook_secret` is empty, the endpoint remains public (backward compatible).

  Supported events:

  - `message_created` (agent outbound messages forwarded to WhatsApp)
  - `conversation_typing_on` / `conversation_typing_off` (typing indicators forwarded to WhatsApp)

  Loop prevention:

  - Bot messages synced by AzWap to Chatwoot are marked with `content_attributes.from_bot=true` and are ignored by the webhook handler.
- Webhook Secret
  Our webhook will be sent to you with an HMAC header and a sha256 default key `secret`.

  You may modify this by using the option below:
  - `--webhook-secret="secret"`
- **Webhook Payload Documentation**
  For detailed webhook payload schemas, security implementation, and integration examples,
  see [Webhook Payload Documentation](./docs/webhook-payload.md)
- **Webhook TLS Configuration**

  If you encounter TLS certificate verification errors when using webhooks (e.g., with Cloudflare tunnels or self-signed certificates):
  ```
  tls: failed to verify certificate: x509: certificate signed by unknown authority
  ```

  You can disable TLS certificate verification using:
  - `--webhook-insecure-skip-verify=true`
  - Or environment variable: `WHATSAPP_WEBHOOK_INSECURE_SKIP_VERIFY=true`

  **Security Warning**: This option disables TLS certificate verification and should only be used in:
  - Development/testing environments
  - Cloudflare tunnels (which provide their own security layer)
  - Internal networks with self-signed certificates

  **For production environments**, it's strongly recommended to use proper SSL certificates (e.g., Let's Encrypt) instead of disabling verification.

## Configuration

You can configure the application using either command-line flags (shown above) or environment variables. Configuration
can be set in three ways (in order of priority):

1. Command-line flags (highest priority)
2. Environment variables
3. `.env` file (lowest priority)

### Environment Variables

You can configure the application using environment variables. Configuration can be set in three ways (in order of
priority):

1. Command-line flags (highest priority)
2. Environment variables
3. `.env` file (lowest priority)

1. Copy `.env.example` to `.env` in your project root (`cp src/.env.example src/.env`)
2. Modify the values in `.env` according to your needs
3. Or set the same variables as system environment variables

#### Available Environment Variables

| Variable                      | Description                                 | Default                                      | Example                                     |
|-------------------------------|---------------------------------------------|----------------------------------------------|---------------------------------------------|
| `APP_PORT`                    | Application port                            | `3000`                                       | `APP_PORT=8080`                             |
| `APP_DEBUG`                   | Enable debug logging                        | `false`                                      | `APP_DEBUG=true`                            |
| `APP_OS`                      | OS name (device name in WhatsApp)           | `Chrome`                                     | `APP_OS=MyApp`                              |
| `APP_BASIC_AUTH`              | Basic authentication credentials            | -                                            | `APP_BASIC_AUTH=user1:pass1,user2:pass2`    |
| `APP_BASE_PATH`               | Base path for subpath deployment            | -                                            | `APP_BASE_PATH=/gowa`                       |
| `APP_TRUSTED_PROXIES`         | Trusted proxy IP ranges for reverse proxy   | -                                            | `APP_TRUSTED_PROXIES=0.0.0.0/0`             |
| `DB_URI`                      | Database connection URI                     | `file:storages/whatsapp.db?_foreign_keys=on` | `DB_URI=postgres://user:pass@host/db`       |
| `WHATSAPP_AUTO_REPLY`         | Auto-reply message (leave empty to disable) | -                                            | `WHATSAPP_AUTO_REPLY="Thanks, we'll reply soon"`  |
| `WHATSAPP_AUTO_MARK_READ`     | Auto-mark incoming messages as read         | `false`                                      | `WHATSAPP_AUTO_MARK_READ=true`              |
| `WHATSAPP_AUTO_DOWNLOAD_MEDIA`| Auto-download media from incoming messages  | `true`                                       | `WHATSAPP_AUTO_DOWNLOAD_MEDIA=false`        |
| `WHATSAPP_WEBHOOK`            | Webhook URL(s) for events (comma-separated) | -                                            | `WHATSAPP_WEBHOOK=https://webhook.site/xxx` |
| `WHATSAPP_WEBHOOK_SECRET`     | Webhook secret for validation               | `secret`                                     | `WHATSAPP_WEBHOOK_SECRET=super-secret-key`  |
| `WHATSAPP_WEBHOOK_INSECURE_SKIP_VERIFY` | Skip TLS verification for webhooks (insecure) | `false` | `WHATSAPP_WEBHOOK_INSECURE_SKIP_VERIFY=true` |
| `WHATSAPP_ACCOUNT_VALIDATION` | Enable account validation                   | `true`                                       | `WHATSAPP_ACCOUNT_VALIDATION=false`         |
| `MESSAGE_WORKER_POOL_SIZE`        | Number of concurrent message workers            | `20`                                         | `MESSAGE_WORKER_POOL_SIZE=30`                   |
| `MESSAGE_WORKER_QUEUE_SIZE`       | Queue size per message worker                   | `1000`                                       | `MESSAGE_WORKER_QUEUE_SIZE=1500`                |
| `BOT_WEBHOOK_POOL_SIZE`       | Bot webhook worker pool size                | `6`                                          | `BOT_WEBHOOK_POOL_SIZE=12`                  |
| `BOT_WEBHOOK_QUEUE_SIZE`      | Bot webhook queue size per worker           | `250`                                        | `BOT_WEBHOOK_QUEUE_SIZE=500`                |
| `BOT_MONITOR_BUFFER`          | Bot monitor ring buffer size (events)       | `200`                                        | `BOT_MONITOR_BUFFER=500`                    |
| `BOT_MONITOR_TTL`             | Bot monitor TTL for recent events           | `0` (disabled)                               | `BOT_MONITOR_TTL=10m`                       |

---

### ⚙️ Scalability Configuration (Enterprise Sizing)

To deploy in a real-world environment with 100+ sessions, tune these variables in your `.env` or via CLI:

| Environment Variable | Scaling Recommendation | Sessions Covered | Description |
| :--- | :--- | :--- | :--- |
| `MESSAGE_WORKER_POOL_SIZE` | `50` to `100` | 100 - 300 | Concurrent workers for message processing. |
| `MESSAGE_WORKER_QUEUE_SIZE` | `2000` to `5000` | Heavy Load | Prevents data loss during spikes. |
| `BOT_WEBHOOK_POOL_SIZE` | `20` to `50` | 500+ Bots | Concurrency for AI webhooks. |
| `WHATSAPP_ACCOUNT_VALIDATION` | `false` | Large Deployments | Skips initial check for faster startup and safer mass-sending. |

#### ⚠️ Pro-Tip: Disabling Account Validation at Scale
For 100+ sessions or high-frequency automated sending, we strongly recommend setting `WHATSAPP_ACCOUNT_VALIDATION=false`.

- **Reduced Latency**: Skips the pre-send network check, making message delivery significantly faster.
- **Improved Stability**: Prevents hitting WhatsApp's rate-limits for "check number" queries, which is a common trigger for temporary session blocks in mass-sending environments.

---

### 🏥 Reactive Health Monitoring
AzWap v2 replaces heavy periodic scanning with an **Event-Driven Health System**:
- **Zero Overhead**: Health status only updates when a real interaction occurs (e.g., a failed tool call) or manual validation.
- **Dependency Propagation**: If an MCP Server fails, all dependent Bots are automatically flagged with the error status.
- **Real-Time UI**: The dashboard reflects these changes instantly through a light 10-second polling mechanism.

---

Note: Command-line flags will override any values set in environment variables or `.env` file.

### Message Worker Pool Configuration

The Message Worker Pool manages WhatsApp inbound message processing (bot AI over WhatsApp, auto-reply, webhook forwarding, Chatwoot forwarding) with controlled concurrency and guaranteed sequential processing per conversation.

**CLI Flags:**
- `--message-workers <number>` - Number of concurrent workers (default: 20)
- `--message-queue-size <number>` - Queue size per worker (default: 1000)

**Environment Variables:**
- `MESSAGE_WORKER_POOL_SIZE` - Number of workers (backward compatible with `BOT_WORKER_POOL_SIZE`)
- `MESSAGE_WORKER_QUEUE_SIZE` - Queue size per worker (backward compatible with `BOT_WORKER_QUEUE_SIZE`)

**Examples:**

```bash
# For 50-200 instances
./whatsapp rest --message-workers=30 --message-queue-size=1500

# For 200-400 instances
./whatsapp rest --message-workers=50 --message-queue-size=2000

# For 400-500 instances
./whatsapp rest --message-workers=80 --message-queue-size=2500
```

**Key Features:**
- Controlled concurrency (no memory/CPU explosions)
- Sequential processing per conversation (messages in order)
- Parallel processing across different chats
- Processes full message flow (bot AI + auto-reply + webhooks + Chatwoot)
- Real-time monitoring via `/api/worker-pool/stats` endpoint
- Backpressure with buffered queues
- Graceful shutdown (completes in-flight jobs)
- Horizontal scaling ready (future: Redis Streams)

**Monitoring Dashboard:**
Access the real-time worker pool monitor in the UI to view:
- Active workers and queue depths
- Message throughput and error rates
- Active chats and their assigned workers
- Per-worker statistics

**Monitoring endpoints:**
- `/api/worker-pool/stats`

### Bot Webhook Worker Pool (separate pool)

The bot webhook endpoint (`POST /bots/:id/webhook`) uses a dedicated worker pool to avoid blocking WhatsApp processing.

**Key properties:**
- Sharding: per `botID|memoryID` (sequential processing for the same memory)
- Backpressure: returns HTTP `429` when the queue is full
- Monitoring endpoint: `/api/bot-webhook-pool/stats`

See [docs/scaling-architecture-plan.md](./docs/scaling-architecture-plan.md) for detailed architecture and scaling guide.

- For more command `./whatsapp --help`

## Requirements

### System Requirements

- **Go 1.24.0 or higher** (for building from source)
- **FFmpeg** (for media processing)

### Platform Support

- Linux (x86_64, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x86_64) - WSL recommended

### Dependencies (without docker)

- Mac OS:
  - `brew install ffmpeg`
  - `export CGO_CFLAGS_ALLOW="-Xpreprocessor"`
- Linux:
  - `sudo apt update`
  - `sudo apt install ffmpeg`
- Windows (not recomended, prefer using [WSL](https://docs.microsoft.com/en-us/windows/wsl/install)):
  - install ffmpeg, [download here](https://www.ffmpeg.org/download.html#build-windows)
  - add to ffmpeg to [environment variable](https://www.google.com/search?q=windows+add+to+environment+path)

## How to use

### Basic

1. Clone this repo: `git clone https://github.com/AzielCF/az-wap`
2. Open the folder that was cloned via cmd/terminal.
3. run `cd src`
4. run `go run . rest` (for REST API mode)
5. Open `http://localhost:3000`

### Docker (you don't need to install in required)

1. Clone this repo: `git clone https://github.com/AzielCF/az-wap`
2. Open the folder that was cloned via cmd/terminal.
3. run `docker-compose up -d --build`
4. open `http://localhost:3000`

### Build your own binary

1. Clone this repo `git clone https://github.com/AzielCF/az-wap`
2. Open the folder that was cloned via cmd/terminal.
3. run `cd src`
4. run
    1. Linux & MacOS: `go build -o whatsapp`
    2. Windows (CMD / PowerShell): `go build -o whatsapp.exe`
5. run
    1. Linux & MacOS: `./whatsapp rest` (for REST API mode)
        1. run `./whatsapp --help` for more detail flags
    2. Windows: `.\whatsapp.exe rest` (for REST API mode)
        1. run `.\whatsapp.exe --help` for more detail flags
6. open `http://localhost:3000` in browser

### MCP Server (Model Context Protocol)

This application can also run as an MCP server, allowing AI agents and tools to interact with WhatsApp through a
standardized protocol.

1. Clone this repo `git clone https://github.com/AzielCF/az-wap`
2. Open the folder that was cloned via cmd/terminal.
3. run `cd src`
4. run `go run . mcp` or build the binary and run `./whatsapp mcp`
5. The MCP server will start on `http://localhost:8080` by default

#### MCP Server Options

- `--host localhost` - Set the host for MCP server (default: localhost)
- `--port 8080` - Set the port for MCP server (default: 8080)

#### Available MCP Tools

The WhatsApp MCP server provides comprehensive tools for AI agents to interact with WhatsApp through a standardized protocol. Below is the complete list of available tools:

##### **📱 Connection Management**

- `whatsapp_connection_status` - Check whether the WhatsApp client is connected and logged in
- `whatsapp_login_qr` - Initiate QR code based login flow with image output
- `whatsapp_login_with_code` - Generate pairing code for multi-device login using phone number
- `whatsapp_logout` - Sign out the current WhatsApp session
- `whatsapp_reconnect` - Attempt to reconnect to WhatsApp using stored session

##### **💬 Messaging & Communication**

- `whatsapp_send_text` - Send text messages with reply and forwarding support
- `whatsapp_send_contact` - Send contact cards with name and phone number
- `whatsapp_send_link` - Send links with custom captions
- `whatsapp_send_location` - Send location coordinates (latitude/longitude)
- `whatsapp_send_image` - Send images with captions, compression, and view-once options
- `whatsapp_send_sticker` - Send stickers with automatic WebP conversion (supports JPG/PNG/GIF)

##### **📋 Chat & Contact Management**

- `whatsapp_list_contacts` - Retrieve all contacts in your WhatsApp account
- `whatsapp_list_chats` - Get recent chats with pagination and search filters
- `whatsapp_get_chat_messages` - Fetch messages from specific chats with time/media filtering
- `whatsapp_download_message_media` - Download images/videos from messages

##### **👥 Group Management**

- `whatsapp_group_create` - Create new groups with optional initial participants
- `whatsapp_group_join_via_link` - Join groups using invite links
- `whatsapp_group_leave` - Leave groups by group ID
- `whatsapp_group_participants` - List all participants in a group
- `whatsapp_group_manage_participants` - Add, remove, promote, or demote group members
- `whatsapp_group_invite_link` - Get or reset group invite links
- `whatsapp_group_info` - Get detailed group information
- `whatsapp_group_set_name` - Update group display name
- `whatsapp_group_set_topic` - Update group description/topic
- `whatsapp_group_set_locked` - Toggle admin-only group info editing
- `whatsapp_group_set_announce` - Toggle announcement-only mode
- `whatsapp_group_join_requests` - List pending join requests
- `whatsapp_group_manage_join_requests` - Approve or reject join requests

#### MCP Endpoints

- SSE endpoint: `http://localhost:8080/sse`
- Message endpoint: `http://localhost:8080/message`

### MCP Configuration

Make sure you have the MCP server running: `./whatsapp mcp`

For AI tools that support MCP with SSE (like Cursor), add this configuration:

```json
{
  "mcpServers": {
    "whatsapp": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

### Production Mode REST (docker)

Using Docker Hub:

```bash
docker run --detach --publish=3000:3000 --name=whatsapp --restart=always --volume=$(docker volume create --name=whatsapp):/app/storages azielcf/az-wap rest --autoreply="Dont't reply this message please"
```

Using GitHub Container Registry:

```bash
docker run --detach --publish=3000:3000 --name=whatsapp --restart=always --volume=$(docker volume create --name=whatsapp):/app/storages ghcr.io/azielcf/az-wap rest --autoreply="Dont't reply this message please"
```

### Production Mode REST (docker compose)

create `docker-compose.yml` file with the following configuration:

Using Docker Hub:

```yml
services:
  whatsapp:
    image: azielcf/az-wap
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    command:
      - rest
      - --basic-auth=admin:admin
      - --port=3000
      - --debug=true
      - --os=Chrome
      - --account-validation=false

volumes:
  whatsapp:
```

Using GitHub Container Registry:

```yml
services:
  whatsapp:
    image: ghcr.io/azielcf/az-wap
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    command:
      - rest
      - --basic-auth=admin:admin
      - --port=3000
      - --debug=true
      - --os=Chrome
      - --account-validation=false

volumes:
  whatsapp:
```

or with env file (Docker Hub):

```yml
services:
  whatsapp:
    image: azielcf/az-wap
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    environment:
      - APP_BASIC_AUTH=admin:admin
      - APP_PORT=3000
      - APP_DEBUG=true
      - APP_OS=Chrome
      - APP_ACCOUNT_VALIDATION=false

volumes:
  whatsapp:
```

or with env file (GitHub Container Registry):

```yml
services:
  whatsapp:
    image: ghcr.io/azielcf/az-wap
    container_name: whatsapp
    restart: always
    ports:
      - "3000:3000"
    volumes:
      - whatsapp:/app/storages
    environment:
      - APP_BASIC_AUTH=admin:admin
      - APP_PORT=3000
      - APP_DEBUG=true
      - APP_OS=Chrome
      - APP_ACCOUNT_VALIDATION=false

volumes:
  whatsapp:
```

### Production Mode (binary)

- download binary from [release](https://github.com/AzielCF/az-wap/releases)

You can fork or edit this source code !

## Current API

### MCP (Model Context Protocol) API

- MCP server provides standardized tools for AI agents to interact with WhatsApp
- Supports Server-Sent Events (SSE) transport
- Available tools: `whatsapp_send_text`, `whatsapp_send_contact`, `whatsapp_send_link`, `whatsapp_send_location`
- Compatible with MCP-enabled AI tools and agents

### HTTP REST API

- [API Specification Document](https://bump.sh/AzielCF/doc/az-wap).
- Check [docs/openapi.yml](./docs/openapi.yaml) for detailed API specifications.
- Use [SwaggerEditor](https://editor.swagger.io) to visualize the API.
- Generate HTTP clients using [openapi-generator](https://openapi-generator.tech/#try).

| Feature | Menu                                   | Method | URL                                 |
|---------|----------------------------------------|--------|-------------------------------------|
| ✅       | Login with Scan QR                     | GET    | /app/login                          |
| ✅       | Login With Pair Code                   | GET    | /app/login-with-code                |
| ✅       | Logout                                 | GET    | /app/logout                         |  
| ✅       | Reconnect                              | GET    | /app/reconnect                      |
| ✅       | Devices                                | GET    | /app/devices                        |
| ✅       | User Info                              | GET    | /user/info                          |
| ✅       | User Avatar                            | GET    | /user/avatar                        |
| ✅       | User Change Avatar                     | POST   | /user/avatar                        |
| ✅       | User Change PushName                   | POST   | /user/pushname                      |
| ✅       | User My Groups                         | GET    | /user/my/groups                     |
| ✅       | User My Newsletter                     | GET    | /user/my/newsletters                |
| ✅       | User My Privacy Setting                | GET    | /user/my/privacy                    |
| ✅       | User My Contacts                       | GET    | /user/my/contacts                   |
| ✅       | User Check                             | GET    | /user/check                         |
| ✅       | User Business Profile                  | GET    | /user/business-profile              |
| ✅       | Send Message                           | POST   | /send/message                       |
| ✅       | Send Image                             | POST   | /send/image                         |
| ✅       | Send Audio                             | POST   | /send/audio                         |
| ✅       | Send File                              | POST   | /send/file                          |
| ✅       | Send Video                             | POST   | /send/video                         |
| ✅       | Send Sticker                           | POST   | /send/sticker                       |
| ✅       | Send Contact                           | POST   | /send/contact                       |
| ✅       | Send Link                              | POST   | /send/link                          |
| ✅       | Send Location                          | POST   | /send/location                      |
| ✅       | Send Poll / Vote                       | POST   | /send/poll                          |
| ✅       | Send Presence                          | POST   | /send/presence                      |
| ✅       | Send Chat Presence (Typing Indicator)  | POST   | /send/chat-presence                 |
| ✅       | Revoke Message                         | POST   | /message/:message_id/revoke         |
| ✅       | React Message                          | POST   | /message/:message_id/reaction       |
| ✅       | Delete Message                         | POST   | /message/:message_id/delete         |
| ✅       | Edit Message                           | POST   | /message/:message_id/update         |
| ✅       | Read Message (DM)                      | POST   | /message/:message_id/read           |
| ✅       | Star Message                           | POST   | /message/:message_id/star           |
| ✅       | Unstar Message                         | POST   | /message/:message_id/unstar         |
| ✅       | Join Group With Link                   | POST   | /group/join-with-link               |
| ✅       | Group Info From Link                   | GET    | /group/info-from-link               |
| ✅       | Group Info                             | GET    | /group/info                         |
| ✅       | Leave Group                            | POST   | /group/leave                        |
| ✅       | Create Group                           | POST   | /group                              |
| ✅       | List Participants in Group             | GET    | /group/participants                 |
| ✅       | Add Participants in Group              | POST   | /group/participants                 |
| ✅       | Remove Participant in Group            | POST   | /group/participants/remove          |
| ✅       | Promote Participant in Group           | POST   | /group/participants/promote         |
| ✅       | Demote Participant in Group            | POST   | /group/participants/demote          |
| ✅       | Export Group Participants (CSV)        | GET    | /group/participants/export          |
| ✅       | List Requested Participants in Group   | GET    | /group/participant-requests         |
| ✅       | Approve Requested Participant in Group | POST   | /group/participant-requests/approve |
| ✅       | Reject Requested Participant in Group  | POST   | /group/participant-requests/reject  |
| ✅       | Set Group Photo                        | POST   | /group/photo                        |
| ✅       | Set Group Name                         | POST   | /group/name                         |
| ✅       | Set Group Locked                       | POST   | /group/locked                       |
| ✅       | Set Group Announce                     | POST   | /group/announce                     |
| ✅       | Set Group Topic                        | POST   | /group/topic                        |
| ✅       | Get Group Invite Link                  | GET    | /group/invite-link                  |
| ✅       | Unfollow Newsletter                    | POST   | /newsletter/unfollow                |
| ✅       | Get Chat List                          | GET    | /chats                              |
| ✅       | Get Chat Messages                      | GET    | /chat/:chat_jid/messages            |
| ✅       | Label Chat                             | POST   | /chat/:chat_jid/label               |
| ✅       | Pin Chat                               | POST   | /chat/:chat_jid/pin                 |
| ✅       | Get Health Status                      | GET    | /api/health/status                  |
| ✅       | Trigger Individual Check               | POST   | /api/health/:type/:id/check         |
| ✅       | Worker Pool Metrics                    | GET    | /api/worker-pool/stats              |
| ✅       | Bot Webhook Metrics                    | GET    | /api/bot-webhook-pool/stats         |

```txt
✅ = Available
❌ = Not Available Yet
```

## User Interface

### MCP UI

- Setup MCP (tested in cursor)
  ![Setup MCP](https://i.ibb.co/vCg4zNWt/mcpsetup.png)
- Test MCP
  ![Test MCP](https://i.ibb.co/B2LX38DW/mcptest.png)
- Successfully setup MCP
  ![Success MCP](https://i.ibb.co/1fCx0Myc/mcpsuccess.png)

### HTTP REST API UI

| Description          | Image                                                         |
|----------------------|---------------------------------------------------------------|
| Homepage             | ![Homepage](./gallery/homepage.jpeg)                           |
| Login                | ![Login](./gallery/login.png)                                 |
| Login With Code      | ![Login With Code](./gallery/login-with-code.png)             |
| Send Message         | ![Send Message](./gallery/send-message.png)                   |
| Send Image           | ![Send Image](./gallery/send-image.png)                       |
| Send File            | ![Send File](./gallery/send-file.png)                         |
| Send Video           | ![Send Video](./gallery/send-video.png)                       |
| Send Sticker         | ![Send Sticker](./gallery/send-sticker.png)                   |
| Send Contact         | ![Send Contact](./gallery/send-contact.png)                   |
| Send Location        | ![Send Location](./gallery/send-location.png)                 |
| Send Audio           | ![Send Audio](./gallery/send-audio.png)                       |
| Send Poll            | ![Send Poll](./gallery/send-poll.png)                         |
| Send Presence        | ![Send Presence](./gallery/send-presence.png)                 |
| Send Link            | ![Send Link](./gallery/send-link.png)                         |
| My Group             | ![My Group](./gallery/group-list.png)                         |
| Group Info From Link | ![Group Info From Link](./gallery/group-info-from-link.png)   |
| Create Group         | ![Create Group](./gallery/group-create.png)                   |
| Join Group with Link | ![Join Group with Link](./gallery/group-join-link.png)        |
| Manage Participant   | ![Manage Participant](./gallery/group-manage-participant.png) |
| My Newsletter        | ![My Newsletter](./gallery/newsletter-list.png)               |
| My Contacts          | ![My Contacts](./gallery/contact-list.png)                    |
| Business Profile     | ![Business Profile](./gallery/business-profile.png)           |

### Mac OS NOTE

- Please do this if you have an error (invalid flag in pkg-config --cflags: -Xpreprocessor)
  `export CGO_CFLAGS_ALLOW="-Xpreprocessor"`

## Important

- This project is unofficial and not affiliated with WhatsApp.
- Please use official WhatsApp API to avoid any issues.
- We only able to run MCP or REST API, this is limitation from whatsmeow library. independent MCP will be available in
  the future.

## License Notes

- This fork is licensed under **LGPLv3** (see `LICENSE`).
- Older AzWap versions (and the original upstream) were under **MIT**.
- The predecessor project (`gowa`) remains under its own license in its original repository.
- External libraries used by AzWap keep their original licenses.
- This change is intended to apply to **modifications to this fork** (AzWap). If you only integrate with AzWap as a standalone service (e.g., via HTTP/webhooks), your project is not re-licensed.

See `THIRD_PARTY_NOTICES.md` for preserved upstream notices.