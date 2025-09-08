# WPGC Church Admin Platform

Complete sermon management system with automated CI/CD deployment pipeline.

## Live Sites

- **Admin Dashboard**: https://admin.wpgc.church
- **API Endpoint**: https://api.wpgc.church

## Features

- 🎵 Sermon upload and management
- 📊 Real-time dashboard analytics  
- 🚀 Automated CI/CD deployment
- 🔔 Discord notifications
- 🔒 SSL/TLS security
- 📱 Responsive design

## Tech Stack

- **Frontend**: Next.js 14, React 18, Tailwind CSS
- **Backend**: Go 1.21, Fiber framework
- **Storage**: MinIO object storage
- **Deployment**: GitHub Actions, Self-hosted runner
- **Infrastructure**: Raspberry Pi, Nginx, Cloudflare

## Local Development

### Backend
```bash
cd backend
go run main.go
```

### Frontend
```bash
cd frontend-react
npm install
npm run dev
```

## Deployment

Automated deployment via GitHub Actions on push to master branch.

## Documentation

- [Architecture Overview](ARCHITECTURE.md)
- [CI/CD Setup Guide](CI-CD-SETUP.md)
- [Deployment Guide](DEPLOYMENT.md)

## Status

[![Deploy to Production](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/actions/workflows/deploy.yml/badge.svg)](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/actions/workflows/deploy.yml)

---

Built for White Plains Gospel Chapel • 2025# Auto-deployment test
