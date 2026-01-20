<!-- markdownlint-disable MD041 -->
<!-- markdownlint-disable-next-line MD033 -->
<div align="center">
  <!-- markdownlint-disable-next-line MD033 -->
  <img src="src/frontend/src/assets/azwap.svg" alt="Az-Wap Logo" width="200" height="200">

# AZ-WAP Enterprise v2.0 BETA
### High-Performance AI-Driven WhatsApp Orchestration Engine

[![Release](https://img.shields.io/github/v/release/AzielCF/az-wap?style=for-the-badge&color=7C3AED)](https://github.com/AzielCF/az-wap/releases)
[![Build Status](https://github.com/AzielCF/az-wap/actions/workflows/build-docker-image.yaml/badge.svg?style=for-the-badge)](https://github.com/AzielCF/az-wap/actions)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg?style=for-the-badge)](LICENSE)

> [!WARNING]
> **PRE-RELEASE / BETA STATUS**: This version (v2.0 Beta) is currently in active development. As a pre-release, unexpected behaviors, bugs, or breaking changes may occur until the first stable version is officially published. Use in production environments with caution.



**AZ-WAP** is a next-generation, enterprise-grade WhatsApp Web API gateway engineered for massive scale, multi-tenant orchestration, and sovereign AI integration. Built on a strict **Hexagonal Architecture**, it enables businesses to handle hundreds of concurrent WhatsApp sessions with absolute stability and AI intelligence.

</div>

---

## 🌟 Vision & Roadmap

> [!NOTE]
> **AZ-WAP** is under constant development. Our mission is to build the most advanced AI agent system for personal assistants and professional use cases across multiple environments.

### The Objective:
Our goal is to transcend simple messaging. We are crafting a highly sophisticated AI ecosystem that learns, remembers, and acts as a true personal assistant. While WhatsApp is our primary channel today, we are not limited by it. We are progressively moving towards a **multi-channel engine**, implementing more networks and interaction environments to become a universal AI orchestration layer.

---

## 📑 Table of Contents
- [🌟 Vision & Roadmap](#-vision--roadmap)
- [✨ Key Features](#-key-features)
- [🏗️ System Architecture](#-system-architecture)
- [💎 Global Client & Tier System](#-global-client--tier-system)
- [🤖 Advanced AI Bot Engine](#-advanced-ai-bot-engine)
- [🚀 Quick Start Guide](#-quick-start-guide)
- [⚙️ Configuration Manual](#-configuration-manual)
- [📂 Project Structure](#-project-structure)
- [📡 API & Webhooks](#-api--webhooks)
- [⚖️ Licensing & Governance](#-licensing--governance)
- [📩 Contact & Support](#-contact--support)

---

## ✨ Key Features

- **High-Scale Concurrency**: Run hundreds of independent WhatsApp nodes using a centralized worker-pool architecture.
- **Autonomous AI Agents**: Integrated with Gemini, OpenAI, and Anthropic. Supports advanced **Tool-Calling** and **Multimodal Context**.
- **Human-Like Simulation**: Advanced presence logic including randomized typing indicators, voice recording simulation, and smart hibernation to prevent account bans.
- **Enterprise Dashboard**: A premium Vue 3 + DaisyUI interface for real-time orchestration of all nodes and clients.
- **Multi-Tenant SaaS Ready**: Built-in system for managing clients, subscriptions, and granular capability toggles.
- **Native Chatwoot Sync**: Bi-directional communication for enterprise customer support workflows.
- **Persistent Memory**: Shared, searchable AI memory across all chat sessions.
- **Model Context Protocol (MCP)**: Use AZ-WAP as an MCP server to provide WhatsApp capabilities to other AI agents.

---

## 🏗️ System Architecture

AZ-WAP is built using **Clean Hexagonal Architecture**, ensuring the business logic remains pure and decoupled from external drivers.

### Core Layers:
1.  **Domain**: Contains the pure business logic, entities (Bot, Workspace, Client), and port interfaces.
2.  **Application (Use Cases)**: Orchestrates the flow of data between the Domain and Infrastructure.
3.  **Infrastructure (Adapters)**:
    *   **WhatsApp Adapter**: High-availability driver built on `whatsmeow`.
    *   **AI Providers**: Dynamic adapters for Gemini, VertexAI, etc.
    *   **Persistence**: Repository implementations for SQLite and PostgreSQL.
    *   **REST/MCP Adapters**: Interface layers for external communication.

---

## 💎 Global Client & Tier System

AZ-WAP includes a robust multi-tenant infrastructure designed to scale. The system supports a flexible subscription model where capabilities can be dynamically toggled and restricted based on the client's tier.

### Dynamic Resource Management
- **Modular Capabilities**: Tiers can be configured to enable or disable specific features like AI Vision, Audio processing, or Proactive Memory.
- **Resource Quotas**: Manage workspace limits and message throughput per client.
- **Scalable Tiers**: The platform is prepared for Standard, Premium, VIP, and Enterprise classifications, with configurations that can be adjusted through the administration layer.


---

## 🤖 Advanced AI Bot Engine

The engine transforms simple chat sessions into **Sovereign Agents**.

### Proactive Mental State
The AI doesn't just respond; it thinks. It analyzes the conversation flow to determine if it should:
- Record a voice note (Audio Generation).
- Use a tool (Query database, fetch web data).
- Save a memory (Update user profile).
- Ack a complex task ("I'm working on that...").

### Multimodal Intelligence
Supports native analysis of:
- **Images & Videos**: Real-time object and context recognition.
- **Documents**: PDF/Text extraction and semantic search.
- **Audio**: Whisper-powered transcription and HSL-controlled voice output.

---

## 🚀 Quick Start Guide

### Prerequisites
- **Go 1.24+** (Modern Go features required)
- **Node.js 20+** & **Bun** (Frontend package manager)
- **FFmpeg** (Required for media conversion)
- **SQLite / PostgreSQL**

### Backend Setup
```bash
cd src
go mod tidy
# Run in REST API mode
go run . rest
```

### Frontend Setup
```bash
cd src/frontend
bun install
bun dev
```

### Environment Synchronization
Copy `.env.example` to `.env` inside the `src` folder and configure your keys.

---

## ⚙️ Configuration Manual

| Variable | Type | Description |
| :--- | :--- | :--- |
| `APP_PORT` | `int` | The main listener port (Default: 3000). |
| `DB_URI` | `string` | Main DB (Example: `postgres://user:pass@host:5432/db`). |
| `APP_BASIC_AUTH` | `string` | Dashboard credentials (`user:pass`). |
| `WHATSAPP_AUTO_REPLY` | `string` | Global welcome message for new sessions. |
| `WHATSAPP_WEBHOOK` | `url` | CSV list of URLs for event notifications. |
| `GEMINI_API_KEY` | `string` | Main AI provider key. |

---

## 📂 Project Structure

```text
az-wap/
├── docs/               # Technical specs & OpenAPI documentation
├── src/                # Core implementation
│   ├── botengine/      # AI Intelligence Layer (Hexagonal)
│   ├── workspace/      # Session Management (Hexagonal)
│   ├── clients/        # Multi-tenant Management
│   ├── infrastructure/ # External Adapters (WA, DB, API)
│   ├── domains/        # Shared Business Entities
│   ├── frontend/       # Enterprise Vue 3 Dashboard
│   └── main.go         # Application Entry Point
├── docker/             # Containerization assets
└── LICENSE             # Dual-License Agreement
```

---

## 📡 API & Webhooks

### REST API Highlights
- **Auth**: Bearer Token or Basic Auth.
- **Media**: Native support for File ID and URL-based sending.
- **Webhooks**: Signed payloads with retry logic and sequential processing.

### Event System
AZ-WAP emits events for:
- `message_received`: Incoming text/media.
- `session_status`: QR ready, Connected, Disconnected.
- `ai_thinking`: Bot start/end processing.
- `client_limit_reached`: Resource exhaustion warnings.

---

## ⚖️ Licensing & Governance

**AZ-WAP** is a **Dual-Licensed** product.

- **Open Source (GNU AGPL v3.0)**: Free for community, personal, and educational use. Any derived work must remain open source.
- **Commercial License**: **MANDATORY** for any legal entity generating **>$20,000 USD** in annual gross revenue. 

### Contribution Agreement
By submitting code, you agree to assign all rights to the original authors as per the **Rights Assignment Clause** in the `LICENSE` file. This allows the project to maintain commercial viability while remaining free for the community.

**Jurisdiction**: Laws of **Peru**.

---

## 📩 Contact & Support

- **Lead Developer**: [Aziel Cruzado](https://azielcruzado.com)
- **Business Email**: [contacto@azielcruzado.com](mailto:contacto@azielcruzado.com)
- **Issues**: [GitHub Issue Tracker](https://github.com/AzielCF/az-wap/issues)

___
<div align="center">
  <p>Copyright (c) 2025-2026 <b>Aziel Cruzado</b>. All rights reserved.</p>
</div>