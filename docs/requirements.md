# Requirements: Huntr-AI Go Migration

## Functional Requirements

| ID | Requirement | Priority | Source Module |
|----|-------------|----------|---------------|
| FR-01 | Scrape jobs from 20+ sources with site-specific parsers | Must | scrapers.py |
| FR-02 | Rate-limit HTTP requests per domain (2 concurrent, 1.5s cooldown) | Must | fetcher.py |
| FR-03 | Headless Chrome for dynamic sites, HTTP for static | Must | scrapers.py |
| FR-04 | URL rotation with fallback pool per source | Must | scrapers.py |
| FR-05 | Early filtering by salary, location, work type | Must | scrapers.py |
| FR-06 | Normalise job titles, salaries, locations | Must | normaliser.py |
| FR-07 | Score jobs against CV preferences (tech:30, domain:25, location:20, salary:15) | Must | scorer.py |
| FR-08 | Parse DOCX CVs (ZIP+XML extraction) | Must | cv_parser.py |
| FR-09 | Chunk CV text (600 chars, 120 overlap) | Must | cv_chunker.py |
| FR-10 | Generate embeddings via Ollama API | Must | embeddings.py |
| FR-11 | Store/retrieve CV vectors in chromem-go | Must | vector_db.py |
| FR-12 | Extract CV profile via Ollama LLM | Must | cv_profile_extractor.py |
| FR-13 | Serve dashboard with 19 REST endpoints | Must | server.py |
| FR-14 | Email notifications for high-scoring jobs | Must | notifications.py |
| FR-15 | Service health monitoring | Must | service_monitor.py |
| FR-16 | Manual scrape trigger via file | Must | main.py (scraper) |
| FR-17 | Collection auto-rotation (max 2) | Should | vector_db.py |
| FR-18 | Log rotation and error reporting | Should | error_reporter.py |

## Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-01 | Full scrape cycle | < 30 minutes |
| NFR-02 | Web response time | < 2 seconds |
| NFR-03 | CV processing time | < 70 seconds |
| NFR-04 | RAM during scraping | < 512 MB |
| NFR-05 | Scraper Docker image | < 200 MB |
| NFR-06 | Processor Docker image | < 20 MB |
| NFR-07 | Web Docker image | < 20 MB |
| NFR-08 | Config format | Backwards compatible JSON |
| NFR-09 | Data format | Backwards compatible JSON |
| NFR-10 | Deployment | ARM64 Pi 5 via Docker Compose |

## API Endpoints (19 total)

| Route | Method | Purpose |
|-------|--------|---------|
| `/health` | GET | Health check |
| `/` | GET | Dashboard HTML |
| `/api/status` | GET | All service status |
| `/api/cv/upload` | POST | Upload CV file |
| `/api/cv/status` | GET | CV processing status |
| `/api/config` | GET/POST | Load/save config |
| `/api/scraper-filters` | GET/POST | Scraper filter prefs |
| `/api/sources` | GET/POST/PUT/DELETE | Source CRUD |
| `/api/sources/test` | POST | Test source fetch |
| `/api/sources/board/toggle` | POST | Enable/disable source |
| `/api/collections` | GET/DELETE/PUT | ChromaDB collection mgmt |
| `/api/schedule/next-run` | GET | Calculate next scrape time |
| `/api/schedule` | POST | Update schedule |
| `/api/errors` | GET/POST | Error history |
| `/api/logs/scraper` | GET | Tail scraper logs |
| `/api/logs/processor` | GET | Tail processor logs |
| `/api/scraper/trigger` | POST | Manual scrape trigger |
| `/api/scraper/cooldown` | GET | Rate-limit status |
| `/api/data/clear` | POST | Clear run data |
