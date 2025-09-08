# WPGC Church Platform Architecture

## Overview
Complete church management platform with public website, admin dashboard, and API backend.

## Domain Structure

### 1. **wpgc.church** - Public Website
- Public-facing church website
- Sermon library for congregation
- Event calendar
- Announcements
- Contact information

### 2. **admin.wpgc.church** - Admin Dashboard
- Church staff portal
- Sermon upload and management
- Member management
- Event management
- Media library
- Analytics and reports

### 3. **api.wpgc.church** - Backend API
- Central API server (Go)
- Handles all data operations
- File uploads and processing
- MinIO integration for storage
- Discord notifications

## API Route Structure

### Public Routes (`/api/public/*`)
Used by wpgc.church
```
GET  /api/public/sermons         - List public sermons
GET  /api/public/sermons/:id     - Get sermon details
GET  /api/public/sermons/latest  - Latest sermons
GET  /api/public/events          - Public events
GET  /api/public/announcements   - Church announcements
```

### Admin Routes (`/api/admin/*`)
Used by admin.wpgc.church
```
# Sermon Management
GET    /api/admin/sermons        - List all sermons (admin view)
GET    /api/admin/sermons/:id    - Detailed sermon info
PUT    /api/admin/sermons/:id    - Update sermon metadata
DELETE /api/admin/sermons/:id    - Delete sermon

# Member Management
GET    /api/admin/members        - List members
POST   /api/admin/members        - Create member
PUT    /api/admin/members/:id    - Update member
DELETE /api/admin/members/:id    - Delete member

# Event Management
GET    /api/admin/events         - List events
POST   /api/admin/events         - Create event
PUT    /api/admin/events/:id     - Update event
DELETE /api/admin/events/:id     - Delete event

# Media Management
GET    /api/admin/media          - List media files
POST   /api/admin/media/upload   - Upload media
DELETE /api/admin/media/:id      - Delete media

# Dashboard Stats
GET    /api/admin/stats          - Dashboard statistics
GET    /api/admin/stats/uploads  - Upload statistics
GET    /api/admin/stats/usage    - Usage statistics
```

### Upload Routes (`/api/uploads/*`)
Direct upload endpoints
```
# Duplicate Detection
GET  /api/uploads/check-hash/:hash    - Check file hash
GET  /api/uploads/hash-stats          - Hash statistics
POST /api/uploads/check-files         - Check multiple files

# Upload Operations
POST   /api/uploads/sermon            - Upload single sermon
POST   /api/uploads/sermons/batch     - Batch upload sermons
POST   /api/uploads/media             - Upload media file

# Upload Management
GET    /api/uploads/status/:id        - Get upload status
DELETE /api/uploads/cancel/:id        - Cancel upload
```

## Technology Stack

### Backend (Go)
- **Framework**: Fiber v2
- **Storage**: MinIO (S3-compatible)
- **Database**: PostgreSQL (future)
- **Caching**: In-memory hash cache
- **Notifications**: Discord webhooks
- **Optimizations**: 
  - Circuit breakers
  - Rate limiting
  - Memory pooling
  - Concurrent uploads

### Admin Dashboard (Next.js)
- **Framework**: Next.js 14 with App Router
- **UI**: Tailwind CSS
- **State**: React hooks
- **File Upload**: react-dropzone
- **API Client**: Fetch API

### Public Website (Future)
- **Framework**: Next.js or static site
- **CMS Integration**: Headless CMS
- **SEO Optimized**
- **CDN**: Cloudflare

## Deployment

### Current Setup
```
api.wpgc.church (Port 80/443) 
    → Raspberry Pi (192.168.1.127:8000)
    → Go backend service
    → MinIO storage
```

### Future Setup
```
wpgc.church         → Vercel/Netlify (Public site)
admin.wpgc.church   → Vercel/Netlify (Admin dashboard)
api.wpgc.church     → Raspberry Pi (API backend)
```

## Security

### CORS Configuration
- Strict origin validation
- Domain whitelist
- Credentials support

### Authentication (Future)
- JWT tokens
- Role-based access (Admin, Staff, Member)
- OAuth integration

### File Security
- Hash-based duplicate detection
- File type validation
- Size limits
- Rate limiting

## Performance Optimizations

### Upload Optimization
- 64MB chunk size
- 10 concurrent threads
- 16MB read/write buffers
- Memory streaming

### Caching
- In-memory hash cache
- MinIO metadata caching
- CDN for static assets

### Raspberry Pi Optimization
- GOMAXPROCS tuning
- Memory limits
- GC optimization
- Connection pooling

## Monitoring

### Health Checks
- `/api/health` - Basic health
- `/api/status` - Service status
- `/api/maintenance/metrics` - Detailed metrics

### Notifications
- Discord webhooks for uploads
- Error notifications
- System alerts

## Future Enhancements

1. **Database Integration**
   - PostgreSQL for metadata
   - User management
   - Audit logs

2. **Authentication System**
   - User login
   - Role management
   - API keys

3. **Advanced Features**
   - AI transcription
   - Auto-generated summaries
   - Search functionality
   - Analytics dashboard

4. **Mobile Apps**
   - iOS/Android apps
   - Push notifications
   - Offline support

## DNS Configuration

Configure these DNS records:

```
wpgc.church         A    YOUR_PUBLIC_IP
admin.wpgc.church   A    YOUR_PUBLIC_IP  
api.wpgc.church     A    YOUR_PUBLIC_IP
```

All domains point to the same IP, with routing handled by:
- Port forwarding on router
- Nginx reverse proxy (optional)
- Application-level routing