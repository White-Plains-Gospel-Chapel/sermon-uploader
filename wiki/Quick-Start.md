# 🚀 Recipe: 5-Minute Quick Start

> **Goal**: Get sermon uploader running on your Pi  
> **Time**: ⏱️ 5 minutes  
> **Difficulty**: 🟢 Easy

## 📦 What You Need

- [ ] Raspberry Pi with Docker installed
- [ ] Computer on same network
- [ ] Discord webhook URL (optional)
- [ ] 5 minutes of time

## 🎯 What You'll Get

After 5 minutes:
- ✅ Web interface for uploading sermons
- ✅ Automatic WAV → AAC conversion
- ✅ Discord notifications (if configured)
- ✅ MinIO storage ready

## 📝 Steps

### Step 1: Clone the Repository (30 seconds)

```bash
git clone https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git
cd sermon-uploader
```

### Step 2: Configure Environment (2 minutes)

```bash
# Copy the template
cp backend/.env.example backend/.env

# Edit with nano (or your favorite editor)
nano backend/.env
```

**Change these values:**
```env
MINIO_ACCESS_KEY=gaius           # Or your preferred key
MINIO_SECRET_KEY=John 3:16       # Or your preferred secret
DISCORD_WEBHOOK_URL=https://...  # Your Discord webhook
```

**Save:** Press `Ctrl+X`, then `Y`, then `Enter`

### Step 3: Start Everything (2 minutes)

```bash
docker-compose up -d
```

**You'll see:**
```
Creating network sermon-uploader_default
Creating sermon-processor ... done
✅ Success!
```

### Step 4: Open the Web Interface (30 seconds)

Find your Pi's IP:
```bash
hostname -I | awk '{print $1}'
```

Open in browser:
```
http://[your-pi-ip]:8000
```

## ✅ Success Checklist

- [ ] Web interface loads
- [ ] Drag-drop area visible
- [ ] "System Ready" status shown
- [ ] Can select WAV files

## 🚨 Troubleshooting

### Web interface won't load?
```bash
# Check if container is running
docker ps | grep sermon

# If not, check logs
docker logs sermon-processor
```

### Can't connect to Pi?
```bash
# From your computer
ping [your-pi-ip]

# Check firewall
sudo ufw status
```

### Container won't start?
```bash
# Reset and try again
docker-compose down
docker system prune -a
docker-compose up -d
```

## 🎉 Next Steps

**Now that it's running:**
1. 📤 [[Upload your first sermon|Upload-Guide]]
2. 🧪 [[Test the system|Testing-Guide]]
3. 🔧 [[Configure for production|Deployment-Guide]]

## 💡 Pro Tips

### Make it start on boot
```bash
# Add to crontab
crontab -e

# Add this line
@reboot cd /path/to/sermon-uploader && docker-compose up -d
```

### Quick health check
```bash
curl http://localhost:8000/api/health
```

### Monitor logs live
```bash
docker logs -f sermon-processor
```

## 💬 Need Help?

- 🙋 [Ask in Discussions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/discussions/categories/q-a)
- 🐛 [Report a Bug](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/issues)
- 📚 [[Back to Home|Home]]

---

**Still stuck?** The community typically responds within 24 hours!