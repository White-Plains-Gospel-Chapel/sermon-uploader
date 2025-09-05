# 👋 START HERE - First Time Users

> **Reading Time**: 2 minutes to understand everything  
> **Action Time**: 10 minutes to get it running

## 🎯 What This Does (30 seconds)

**The Problem:** 
- 📼 You have large WAV sermon recordings
- 🌐 You need them online for streaming
- 😫 Manual conversion and upload is tedious

**The Solution:**
- 🎵 Drag & drop WAV files to a web page
- ⚡ Automatic conversion to streaming format
- 📢 Discord notifications when ready
- 💾 All files organized and stored

## 🤔 Is This For You? (30 seconds)

### ✅ Perfect If You:
- Record sermons as WAV files
- Want automatic audio processing
- Have a Raspberry Pi (or similar)
- Need web-accessible sermon library

### ❌ Not For You If:
- Already have a streaming solution
- Don't record sermons
- No server/Pi available

## 🛤️ Your Journey (1 minute)

### Step 1: Choose Your Path

| Your Situation | Your Path | Time |
|----------------|-----------|------|
| **"Just make it work!"** | [🚀 Quick Start Recipe](recipes/quick-start.md) | 5 min |
| **"I want to understand first"** | [📊 How It Works](architecture/overview.md) | 15 min |
| **"I'm a developer"** | [💻 Dev Setup](development/setup/local-setup.md) | 30 min |
| **"Setting up for church"** | [⛪ Church Setup Guide](recipes/church-setup.md) | 45 min |

### Step 2: Get It Running (5 minutes)
Follow the [Quick Start Recipe](recipes/quick-start.md) - it's literally copy-paste commands.

### Step 3: Upload Your First Sermon (3 minutes)
Follow the [First Upload Guide](recipes/first-upload.md) - drag, drop, done!

### Step 4: Customize (Optional)
- [Add Discord Notifications](recipes/setup-discord.md)
- [Configure Storage](recipes/change-storage.md)
- [Set Up HTTPS](recipes/add-ssl.md)

## 📦 What You Get

After 10 minutes, you'll have:

```
Your Setup:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Web Upload  │────▶│  Pi Server   │────▶│   Discord    │
│   Interface  │     │  Processing  │     │Notifications │
└──────────────┘     └──────────────┘     └──────────────┘
        ↓                    ↓                     
  [Drag WAV file]      [Converts to AAC]     [Get notified]
```

## ⚡ Quick Wins

**Fastest Success Path:**
1. 📋 Copy-paste from [Quick Start](recipes/quick-start.md) (5 min)
2. 🧪 Run [Test Commands](recipes/test-everything.md) (2 min)  
3. 📤 [Upload First File](recipes/first-upload.md) (3 min)
4. 🎉 Show your pastor! (Priceless)

## 🆘 Common Questions

**Q: Do I need coding skills?**  
A: Nope! Just copy-paste commands.

**Q: Will this work on my Pi?**  
A: Yes, if it has Docker and 2GB+ RAM.

**Q: How much storage do I need?**  
A: ~500MB per hour of audio (AAC format).

**Q: Can multiple people use it?**  
A: Yes! Anyone on your network can upload.

**Q: Is it secure?**  
A: Yes, with proper setup. See [Security Guide](architecture/security/security-model.md).

## 📚 Next Steps

### After Getting It Running:
1. **Learn More**: [Visual System Overview](architecture/overview.md)
2. **Customize**: [Configuration Guide](development/setup/)
3. **Go Production**: [Deployment Guide](architecture/deployment/)
4. **Get Help**: [Troubleshooting](operations/troubleshooting/)

### Quick Reference:
- ⚡ [All Commands](quick-reference/commands.md)
- 🍰 [All Recipes](recipes/)
- 🗺️ [Navigation Map](NAVIGATION.md)
- 📋 [Version Info](releases/CHANGELOG.md)

## 💡 Pro Tips for Beginners

1. **Don't overthink it** - The quick start really is that quick
2. **Test with one file first** - Don't batch upload 50 sermons immediately
3. **Keep the commands handy** - Bookmark the [commands page](quick-reference/commands.md)
4. **Join the Discord** - Get help when stuck

## 🎯 Success Metrics

You'll know it's working when:
- ✅ Web interface loads at `http://your-pi:8000`
- ✅ You can drag a WAV file to the upload area
- ✅ Progress bar shows upload/conversion status
- ✅ Discord (if configured) sends notifications
- ✅ AAC file appears in MinIO storage

---

**Ready?** Let's go! → [🚀 Start Quick Recipe](recipes/quick-start.md)

**Need help?** The whole setup takes 10 minutes. If you're stuck after 15 minutes, something's wrong - check [troubleshooting](operations/troubleshooting/) or ask in Discord!