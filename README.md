# Sermon Uploader

A multi-component system for processing and managing sermon uploads, consisting of a host machine processor and a Raspberry Pi queue GUI.

## Project Structure

```
sermon-uploader/
├── ssd-host/           # Host machine sermon processor
│   ├── Dockerfile      # Docker configuration for host
│   ├── sermon_processor.py
│   ├── pyproject.toml
│   └── .python-version
├── pi-processor/       # Raspberry Pi queue GUI
│   ├── Dockerfile      # Docker configuration for Pi
│   └── sermon_queue_gui.py
├── .gitignore         # Git ignore rules for entire project
└── README.md          # This file
```

## Components

### SSD Host Processor (`ssd-host/`)
- Main sermon processing engine
- Runs on the host machine with SSD storage
- Handles bulk sermon processing and storage

### Pi Processor (`pi-processor/`)
- Queue management GUI
- Runs on Raspberry Pi
- Provides user interface for sermon queue management

## Docker Images

### Building Images

#### Host Machine Image
```bash
cd ssd-host
docker build -t sermon-uploader-host:latest .
```

#### Pi Image
```bash
cd pi-processor
docker build -t sermon-uploader-pi:latest .
```

### Publishing to GitHub Container Registry

1. **Login to GitHub Container Registry:**
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

2. **Tag and push host image:**
```bash
docker tag sermon-uploader-host:latest ghcr.io/USERNAME/sermon-uploader-host:latest
docker push ghcr.io/USERNAME/sermon-uploader-host:latest
```

3. **Tag and push Pi image:**
```bash
docker tag sermon-uploader-pi:latest ghcr.io/USERNAME/sermon-uploader-pi:latest
docker push ghcr.io/USERNAME/sermon-uploader-pi:latest
```

### Publishing to Docker Hub

1. **Login to Docker Hub:**
```bash
docker login
```

2. **Tag and push images:**
```bash
docker tag sermon-uploader-host:latest USERNAME/sermon-uploader-host:latest
docker push USERNAME/sermon-uploader-host:latest

docker tag sermon-uploader-pi:latest USERNAME/sermon-uploader-pi:latest
docker push USERNAME/sermon-uploader-pi:latest
```

## Environment Configuration

Each component has its own `.env` file for configuration:

- `ssd-host/.env` - Host machine configuration
- `pi-processor/.env` - Pi configuration

**Note:** `.env` files are ignored by Git for security. Copy `.env.example` files if they exist, or create your own based on the component requirements.

## Development

### Prerequisites
- Python 3.8+
- Docker
- Git

### Setup
1. Clone the repository
2. Create appropriate `.env` files in each component directory
3. Build Docker images as needed
4. Run components using Docker Compose or individual Docker commands

## Deployment

### Using Docker Compose (Recommended)
Create a `docker-compose.yml` file in the root directory to orchestrate both services.

### Manual Deployment
Deploy each component independently using their respective Docker images.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## License

[Add your license information here]
