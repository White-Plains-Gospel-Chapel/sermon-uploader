# 🧭 Visual Navigation Guide

> **Can't find what you need?** This interactive map will get you there in seconds!

## 🎯 Choose Your Path

```mermaid
flowchart TD
    Start[🤔 What do you need?] --> Q1{Your Goal?}
    
    Q1 -->|"🚀 Get it running"| Running[Running System]
    Q1 -->|"🔧 Fix something"| Broken[Something's Broken]
    Q1 -->|"📚 Learn about it"| Learn[Understanding]
    Q1 -->|"💻 Modify code"| Dev[Development]
    
    Running --> R1[⏱️ Time Available?]
    R1 -->|"5 min"| R1A[📦 Quick Start Recipe]
    R1 -->|"30 min"| R1B[🔧 Full Setup Guide]
    R1 -->|"1 hour"| R1C[🚀 Production Deploy]
    
    Broken --> B1[What's broken?]
    B1 -->|"Won't start"| B1A[🐳 Docker Troubleshooting]
    B1 -->|"Can't upload"| B1B[📤 Upload Issues]
    B1 -->|"No notifications"| B1C[💬 Discord Fix]
    B1 -->|"Other"| B1D[🔍 Debug Commands]
    
    Learn --> L1[Level of Detail?]
    L1 -->|"Quick overview"| L1A[📊 Visual Diagrams]
    L1 -->|"Technical details"| L1B[🏗️ Architecture Docs]
    L1 -->|"API reference"| L1C[📡 API Documentation]
    
    Dev --> D1[What to build?]
    D1 -->|"New feature"| D1A[🎨 Frontend Guide]
    D1 -->|"Fix bug"| D1B[🐛 Debug Guide]
    D1 -->|"Improve performance"| D1C[⚡ Optimization Guide]
```

## 🔍 Quick Decision Helper

### "I don't know where to start"
→ Go to **[🍰 Quick Start Recipe](recipes/quick-start.md)**

### "It was working but now it's not"
→ Go to **[⚡ Emergency Commands](quick-reference/commands.md#emergency-commands)**

### "I need to understand before I touch anything"
→ Go to **[📊 Visual System Overview](architecture/overview.md)**

### "I want to add a new feature"
→ Go to **[💻 Development Setup](development/setup/local-setup.md)**

### "I need to deploy this for real users"
→ Go to **[🚀 Production Deployment](architecture/deployment/cloudflare-tunnel.md)**

## 📍 Most Visited Pages

Based on what people usually need:

1. **[⚡ Commands Cheat Sheet](quick-reference/commands.md)** - #1 most used
2. **[🍰 5-Minute Quick Start](recipes/quick-start.md)** - For new users
3. **[🔧 Fix Common Problems](operations/troubleshooting/critical-insights.md)** - When stuck
4. **[📊 System Status Check](quick-reference/commands.md#monitoring-commands)** - Is it working?
5. **[🚀 Docker Commands](quick-reference/commands.md#essential-commands)** - Start/stop/restart

## 🏷️ By Component

### Frontend (Web Interface)
- 📝 [Code Location](../frontend/)
- 🎨 [UI Components](development/guides/frontend-components.md)
- 🔧 [Configuration](development/setup/frontend-config.md)

### Backend (Processing)
- 📝 [Code Location](../backend/)
- ⚙️ [Services](development/guides/backend-services.md)
- 🔌 [API Endpoints](quick-reference/api.md)

### MinIO (Storage)
- 💾 [Setup Guide](architecture/deployment/secrets-setup.md)
- 🔐 [Credentials](quick-reference/credentials.md)
- 📦 [Bucket Structure](architecture/decisions/001-minio-storage.md)

### Docker (Deployment)
- 🐳 [Compose Files](../docker-compose.yml)
- 🏗️ [Build Process](operations/ci-cd/)
- 📦 [Container Management](quick-reference/commands.md#maintenance-commands)

## 🎯 By Use Case

### 👤 For Church Staff
- Start here: **[User Guide](recipes/)**
- Daily use: **[Upload Sermons](recipes/first-upload.md)**
- Problems: **[Get Help](operations/troubleshooting/)**

### 👨‍💻 For Developers
- Start here: **[Dev Setup](development/setup/)**
- Architecture: **[Technical Docs](architecture/)**
- Contributing: **[Development Guide](development/guides/)**

### 🔧 For System Admins
- Start here: **[Deployment](architecture/deployment/)**
- Monitoring: **[System Health](operations/monitoring/)**
- Maintenance: **[Admin Commands](quick-reference/commands.md#maintenance-commands)**

## 🆘 Still Lost?

Can't find what you need? Try:

1. **Search** - Use Ctrl+F on this page
2. **[Commands Reference](quick-reference/commands.md)** - Has everything
3. **[FAQ](operations/troubleshooting/faq.md)** - Common questions
4. **Ask** - Check the Discord channel

---

💡 **Pro Tip**: Bookmark this page! It's your map to everything.