# AZ-WAP Frontend (Enterprise UI)

This is the official Vue.js 3 frontend application for **AZ-WAP**, a high-performance AI-driven WhatsApp Web Automation Engine. Built for modern enterprises, it provides a sleek, high-end dashboard for managing multi-tenant WhatsApp nodes, workspaces, and AI bot interactions.

## 🚀 Key Features
- **Modern Dashboard**: Real-time monitoring of WhatsApp bot status and active sessions.
- **Hexagonal Architecture**: Clean and maintainable frontend structure that integrates seamlessly with the Go backend.
- **Enterprise Design**: Premium UI built with Vue 3, Vite, and DaisyUI, featuring dark mode, glassmorphism, and smooth micro-animations.
- **Multi-Tenant Support**: Exclusive "Clients" view for managing global subscriptions and tiers (VIP, Premium, Enterprise).
- **Control Center**: Manage workspaces, bots, AI credentials, and platform health from a single interface.

## 🛠 Technology Stack
- **Framework**: [Vue 3](https://vuejs.org/) (Composition API)
- **Tooling**: [Vite](https://vite.dev/)
- **Styling**: [Tailwind CSS](https://tailwindcss.com/) + [DaisyUI](https://daisyui.com/)
- **Store**: [Pinia](https://pinia.vuejs.org/)
- **Routing**: [Vue Router 4](https://router.vuejs.org/)
- **Icons**: [Lucide Vue Next](https://lucide.dev/)
- **API**: Custom Axios-based composables for secure backend interaction.

## 📖 Recommended Setup
- **IDE**: [VS Code](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar)
- **Formatter**: [Prettier](https://prettier.io/) (configured for the project)
- **Node Version**: Check `.nvmrc` or `package.json` engines (>= 20.x).

## 🚀 Project Execution

### Install Dependencies
```sh
bun install
```

### Hot-Reload for Development
```sh
bun dev
```

### Build for Production
```sh
bun run build
```

### Unit Testing (Vitest)
```sh
bun test:unit
```

## ⚖️ License
This frontend is part of the **AZ-WAP** project and is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. A proprietary license is required for commercial use exceeding $20,000 USD gross annual revenue. See the root `LICENSE` file for full details.

Developed with ❤️ by [AzielCF](https://azielcruzado.com).
