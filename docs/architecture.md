# Go Architecture

## Service Architecture (unchanged from Python)

```
                    ┌─────────────┐
                    │   huntr-web  │  ← chi router, port 5000
                    │  (Go binary) │
                    └──────┬──────┘
                           │ reads scored jobs JSON
              ┌────────────┴────────────┐
              │                         │
    ┌─────────┴──────────┐    ┌────────┴────────────┐
    │   huntr-scraper    │    │   huntr-processor    │
    │   (Go binary)      │    │   (Go binary)        │
    │                    │    │                      │
    │ rod + goquery      │    │ chromem-go + Ollama  │
    └────────┬───────────┘    └────────┬─────────────┘
             │ writes raw jobs JSON    │ reads raw, writes scored
             └─────────┬──────────────┘
                       │
                  ┌────┴─────┐
                  │  /data/  │  ← NAS mount (shared storage)
                  └──────────┘
```

## Go Project Layout

```
go-migration/src/
├── cmd/                    # Service entrypoints (one binary each)
│   ├── scraper/main.go     # Poll loop, trigger file watch
│   ├── processor/main.go   # Poll loop, CV priority processing
│   └── web/main.go         # HTTP server startup, graceful shutdown
├── internal/               # Private packages (not importable externally)
│   ├── config/             # Config struct, JSON load/save
│   ├── models/             # Shared data structures (Job, CV, Source)
│   ├── scraper/            # Scraping orchestration
│   │   └── parsers/        # One file per source (implements Parser interface)
│   ├── processor/          # Normalisation, scoring, CV pipeline
│   ├── web/                # HTTP handlers, dashboard, monitoring
│   │   └── handlers/       # One file per endpoint group
│   └── common/             # Logging, file utilities
├── templates/              # Go html/template files
└── testdata/               # Test fixtures (HTML, JSON)
```

## Key Design Patterns

### Parser Interface
```go
type Parser interface {
    ParseListings(html string, sourceURL string) ([]models.Job, error)
    Name() string
}
```
Each source implements this interface. New sources = new file, no orchestration changes.

### Concurrency Model
- **Scraper**: goroutines per source, `sync.Mutex` for shared state, `semaphore` for per-domain limiting
- **Processor**: single goroutine poll loop (CV processing is sequential by design - lock file pattern)
- **Web**: chi handles concurrency via Go's net/http server (goroutine per request)

### Configuration
Single `Config` struct with JSON tags matching existing `config.json` keys. Zero migration needed.

### Error Handling
- Structured errors with `fmt.Errorf("scraper: %w", err)` wrapping
- No panics in production code
- Graceful degradation: parser failure skips source, doesn't crash service
