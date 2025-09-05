# 🎵 Welcome to the Sermon Uploader Wiki

> **TL;DR**: Convert sermon WAV files to streaming format automatically. Drop files, get notified, done!

## 🚀 Quick Navigation

### For Church Staff
- [**5-Minute Setup**](Quick-Start) - Get it running NOW
- [**Upload Your First Sermon**](Upload-Guide) - Step-by-step guide
- [**Troubleshooting**](Troubleshooting) - When things go wrong

### For Tech Teams
- [**Mac GUI Setup**](Mac-GUI-Setup) - Desktop uploader
- [**Pi Setup**](Raspberry-Pi-Setup) - Server configuration
- [**Docker Deployment**](Docker-Deployment) - Container setup

### For Developers
- [**Architecture Overview**](Architecture) - How it works
- [**API Reference**](API-Reference) - Endpoints & usage
- [**Contributing Guide**](Contributing) - Help improve the project

## 💬 Community

- [**Ask Questions**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/q-a) - Get help from community
- [**Share Ideas**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/ideas) - Suggest features
- [**Report Bugs**](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues) - Help us fix problems

## 🎯 What This System Does

```mermaid
graph LR
    A[📼 WAV File] -->|Drag & Drop| B[🌐 Web Interface]
    B -->|Upload| C[☁️ MinIO Storage]
    C -->|Process| D[🔄 FFmpeg Convert]
    D -->|Output| E[🎵 AAC Stream]
    E -->|Notify| F[💬 Discord Alert]
```

## ⏱️ Time Investment

| Goal | Time | Guide |
|------|------|--------|
| **Just get it running** | 5 min | [Quick Start](Quick-Start) |
| **Understand the system** | 15 min | [Architecture](Architecture) |
| **Full production setup** | 45 min | [Deployment Guide](Deployment-Guide) |
| **Contribute code** | 1 hour | [Developer Setup](Developer-Setup) |

## 📊 System Requirements

### Minimum
- **Pi**: Raspberry Pi 4 with 2GB RAM
- **Storage**: 50GB free space
- **Network**: 100Mbps connection
- **Docker**: Version 20+

### Recommended
- **Pi**: Raspberry Pi 5 with 8GB RAM
- **Storage**: 500GB+ SSD
- **Network**: Gigabit connection
- **Cooling**: Active cooling for Pi

## 🆘 Getting Help

1. **Check the [FAQ](FAQ)** - Common questions answered
2. **Search [Discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions)** - Community solutions
3. **Ask in [Q&A](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/q-a)** - Get help
4. **Report [Issues](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues)** - Bug reports

## 📈 Project Stats

- **Version**: 0.2.0
- **License**: Internal Use
- **Platform**: Raspberry Pi / Docker
- **Language**: Go + React

---

**Ready to start?** → [🚀 Quick Start Guide](Quick-Start)