# Huntr-Go

A job search automation system written in Go. Scrapes 20+ job boards, scores listings against your CV using vector embeddings, and serves results through a web dashboard.

> **Mirror**: This repository is mirrored to [GitHub](https://github.com/Mionaid-is-buntata/huntr-go). The canonical source lives on a private Gitea instance.

## Overview

Huntr-Go is a Python-to-Go rewrite of Huntr-AI, built for deployment on a Raspberry Pi 5. The migration targets a dramatically smaller footprint (2.3GB Docker image down to ~200MB) while maintaining full API and data format compatibility with the original.

Three services coordinate through shared file storage:

```
         ┌─────────────┐
         │  huntr-web   │  ← Dashboard + REST API (port 5000)
         └──────┬───────┘
                │ reads scored jobs
     ┌──────────┴──────────┐
     │                     │
┌────┴──────────┐   ┌─────┴────────────┐
│huntr-scraper  │   │huntr-processor   │
│               │   │                  │
│ rod + goquery │   │ chromem-go       │
│ 20+ parsers   │   │ + Ollama         │
└────┬──────────┘   └────┬─────────────┘
     │ writes raw        │ reads raw, writes scored
     └────────┬──────────┘
          ┌───┴────┐
          │ /data/ │  ← NAS-mounted shared volume
          └────────┘
```

## Features

**Scraper**
- 20+ job board parsers (LinkedIn, Indeed, Reed, Glassdoor, OTTA, and more)
- Headless Chrome (via Rod) for JavaScript-rendered pages, static HTTP for simple sites
- URL pool rotation with automatic fallback on failure
- Rate limiting (3s between sources, max 2 concurrent connections per domain)
- Early filtering by salary, location, and work type

**Processor**
- Job normalisation (title expansion, salary parsing, location standardisation, deduplication)
- Weighted scoring against CV profile (tech stack, domain, location, salary)
- DOCX parsing and text chunking for CV uploads
- Vector embeddings via Ollama (nomic-embed-text) stored in chromem-go
- LLM-powered CV profile extraction (skills, domains, experience level)
- Automatic collection rotation (configurable max, oldest dropped)

**Web Dashboard**
- Single-page dashboard with real-time job board status
- 19 REST API endpoints for full system control
- Source management (CRUD, testing, enable/disable)
- CV upload and processing status
- Manual scrape triggers and scheduling (cron)
- Error aggregation and log tailing
- Service health monitoring

## Tech Stack

| Component | Purpose |
|-----------|---------|
| Go 1.25 | Language runtime |
| [chi](https://github.com/go-chi/chi) | HTTP routing |
| [rod](https://github.com/go-rod/rod) | Headless Chrome automation |
| [goquery](https://github.com/PuerkitoBio/goquery) | HTML parsing (CSS selectors) |
| [chromem-go](https://github.com/philippgille/chromem-go) | In-process vector database |
| [gopsutil](https://github.com/shirou/gopsutil) | System resource monitoring |
| [Ollama](https://ollama.com/) | Local LLM for embeddings and profile extraction |

## Project Structure

```
src/
├── cmd/
│   ├── scraper/       # Scraper service entrypoint
│   ├── processor/     # Processor service entrypoint
│   └── web/           # Web server entrypoint
├── internal/
│   ├── config/        # JSON configuration management
│   ├── models/        # Job, CV, and score data structures
│   ├── scraper/       # Scraping orchestration, fetchers, 20+ parsers
│   ├── processor/     # Normalisation, scoring, CV pipeline, vector DB
│   ├── web/           # Chi router, handlers, dashboard, scheduler
│   └── common/        # Shared logging utilities
├── templates/         # HTML dashboard template
├── go.mod
└── go.sum
docker/
├── Dockerfile.scraper
├── Dockerfile.processor
├── Dockerfile.web
└── config/
    └── config.json.template
docker-compose.yml
docs/
├── PRD.md             # Product requirements
├── architecture.md    # System design
├── requirements.md    # Functional & non-functional requirements
└── dependencies.md    # Python-to-Go dependency mapping
```

## Prerequisites

- Docker and Docker Compose
- Ollama running with the `nomic-embed-text` model pulled
- A NAS or local directory for persistent data storage

## Getting Started

1. **Clone and configure**
   ```bash
   git clone https://github.com/Mionaid-is-buntata/huntr-go.git
   cd huntr-go
   cp .env.example .env
   # Edit .env with your email settings (optional) and Ollama host
   ```

2. **Prepare the config**
   ```bash
   cp docker/config/config.json.template /path/to/data/config/config.json
   # Edit config.json with your job sources, preferences, and schedule
   ```

3. **Build and run**
   ```bash
   docker compose build
   docker compose up -d
   ```

4. **Access the dashboard**

   Open `http://localhost:5000` in your browser.

5. **Upload your CV**

   Use the dashboard or POST a `.docx` file to `/api/cv/upload`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `host.docker.internal:11434` | Ollama server address |
| `HUNTR_EMAIL` | _(empty)_ | SMTP sender address for notifications |
| `HUNTR_EMAIL_PASSWORD` | _(empty)_ | SMTP password |
| `HUNTR_EMAIL_RECIPIENT` | _(empty)_ | Notification recipient |
| `LOG_LEVEL` | `info` | Log verbosity: debug, info, warn, error |
| `SCRAPER_POLL_INTERVAL` | `1800` | Seconds between scrape cycles |
| `PROCESSOR_POLL_INTERVAL` | `60` | Seconds between processor polls |
| `DEBUG` | `false` | Enable debug mode |

## Configuration

The system is configured through `/data/config/config.json`, which controls:

- **Job Sources** - URLs, parser type (static/dynamic), enable/disable per board
- **Preferences** - Keywords, locations, minimum salary, work types (used for scoring)
- **Scheduling** - Scrape frequency, time windows, active days
- **CV Settings** - Chunk size, overlap, vector DB parameters, Ollama model
- **Email** - SMTP config and high-score notification threshold

The config can be edited directly or through the web dashboard at `/api/config`.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Dashboard |
| GET | `/health` | Health check |
| GET | `/api/status` | All service statuses |
| POST | `/api/cv/upload` | Upload CV (.docx, 10MB limit) |
| GET | `/api/cv/status` | CV processing status |
| GET/POST | `/api/config` | Read/update configuration |
| GET/POST | `/api/scraper-filters` | Scoring preferences |
| GET/POST/PUT/DELETE | `/api/sources` | Job source CRUD |
| POST | `/api/sources/test` | Test a source URL |
| POST | `/api/sources/board/toggle` | Enable/disable a source |
| GET | `/api/sources/stats` | Per-source job counts |
| GET/PUT/DELETE | `/api/collections` | Vector DB collections |
| GET/POST | `/api/schedule` | Scraper schedule |
| GET | `/api/schedule/next-run` | Next scheduled scrape |
| GET/POST | `/api/errors` | Error history |
| GET | `/api/logs/scraper` | Tail scraper logs |
| GET | `/api/logs/processor` | Tail processor logs |
| POST | `/api/scraper/trigger` | Trigger manual scrape |
| GET | `/api/scraper/cooldown` | Cooldown status |
| POST | `/api/data/clear` | Clear run data |

## Deployment

The target deployment is a Raspberry Pi 5 (ARM64) with services running as Docker containers and data stored on a NAS mount.

```bash
# On the Pi
cd ~/huntr-go
git pull origin main
docker compose build --no-cache
docker compose up -d
```

**Resource constraints:**
- 512MB RAM limit per service
- ARM64 multi-stage Docker builds (alpine-based)
- Scraper image includes Chromium (~180MB), processor and web images are ~20MB each

## Performance Targets

| Metric | Target |
|--------|--------|
| Full scrape cycle | < 30 minutes |
| Web response time | < 2 seconds |
| CV processing | < 70 seconds |
| RAM per service | < 512 MB |
| Total Docker image size | ~200 MB |

## Running Tests

```bash
cd src
go test ./...
```

## Documentation

Detailed design documents are in the [docs/](docs/) directory:

- [PRD.md](docs/PRD.md) - Product requirements document
- [architecture.md](docs/architecture.md) - System architecture and design decisions
- [requirements.md](docs/requirements.md) - Functional and non-functional requirements
- [dependencies.md](docs/dependencies.md) - Python-to-Go dependency mapping
