# Code Review: Huntr-AI Go Migration

**Date:** 2026-03-22
**Reviewer:** Claude
**Branch:** main
**Context:** Single-user local deployment on Raspberry Pi 5. No auth needed. Email notifications disabled.

## Overall Assessment

Well-structured Go migration with clean separation of concerns (3-service architecture), good use of interfaces for testability, and sensible dependency choices. Below are the issues found, organised by severity.

---

## 1. Bugs

### BUG: CV upload reads wrong byte count after seek — `server.go:270-278`

```go
if seeker, ok := file.(io.Seeker); ok {
    seeker.Seek(0, io.SeekStart)
}

data := make([]byte, header.Size)
if _, err := file.Read(data); err != nil {
```

Two problems:
1. If the file doesn't implement `Seeker`, `Read` starts from byte 4 (after magic byte check), missing the first 4 bytes of the file.
2. Even after seeking, `Read` is not guaranteed to read `header.Size` bytes in a single call — use `io.ReadFull` or `io.ReadAll`.

### BUG: Processor main loop has no graceful shutdown — `cmd/processor/main.go:227-233`

```go
for {
    processCV()
    processJobs()
    time.Sleep(time.Duration(interval) * time.Second)
}
```

Unlike the scraper which handles SIGINT/SIGTERM, the processor has no signal handling. `docker stop` will wait for `stop_grace_period` (5s) then SIGKILL, potentially mid-write of scored JSON files.

### BUG: `rotateLog` deletes log if under size limit — `cmd/scraper/main.go:179-181`

```go
} else {
    os.Remove(logFile)
}
```

If the log is under 100KB, it gets deleted every scrape cycle. This means you lose recent log entries even when the file is small.

### BUG: Template caching with `sync.Once` prevents recovery — `server.go:212-225`

If the template fails to parse on the first attempt (e.g. missing file at startup), `tmplOnce.Do` never runs again, so the dashboard is permanently broken until restart. Consider lazy init without `sync.Once`, or fail fast at startup.

### BUG: `handleBoardToggle` toggles ALL sources — `server.go:560-565`

```go
// For now, toggle all sources (board grouping is a UI concern)
for i := range cfg.JobSources {
    cfg.JobSources[i].Enabled = data.Enabled
    affected++
}
```

The endpoint accepts a `board` parameter but ignores it and toggles every source.

### BUG: Vector DB path outside data volume — `vector_db.go:14`

```go
vectorDBPath = "/chromadb"
```

This is outside `/data/` unlike every other persistent path. Vector DB data is stored in the container root, not on the NAS volume. It will be lost on container recreation.

---

## 2. Design Issues

### Ollama needs separate model and embedding model selection — `ollama.go`

Currently `SelectModel()` picks a single model used for both text generation (profile extraction) and embeddings. These should be independently configurable — the best embedding model is often different from the best generation model (e.g. `nomic-embed-text` for embeddings, `mistral:7b` for generation). The config should allow:

```json
{
  "cv_processing": {
    "llm_model": "mistral:7b",
    "embedding_model": "nomic-embed-text"
  }
}
```

With fallback to auto-selection if not specified.

### Log retention should be 24 hours — `cmd/scraper/main.go`

Current behaviour: archives logs when over 100KB and keeps 3 archives (no time limit). Logs should instead be retained for 24 hours maximum. Replace the size-based rotation with time-based cleanup:
- Delete archive files older than 24 hours
- Remove the "delete log if under size limit" branch (line 179-181) which is currently a bug

### Log file read loads entire file into memory — `server.go:777`

```go
data, err := os.ReadFile(logPath)
```

With 24hr retention this is less likely to be problematic, but reading only the tail of the file would be more efficient — especially since the API already only returns the last N lines.

---

## 3. Robustness

### Unbounded response body read — `fetcher.go:128`

```go
body, readErr := io.ReadAll(resp.Body)
```

A misbehaving job board could return an enormous response, exhausting the 512MB container memory. Use `io.LimitReader(resp.Body, maxBodySize)` (e.g. 10MB).

### Error return values silently ignored

Multiple locations where `config.Save` return values are discarded — the user gets a success response while their change may have been lost:

- `server.go:442` — `config.Save`
- `server.go:481` — `config.Save`
- `server.go:508` — `config.Save`
- `server.go:567` — `config.Save`
- `server.go:274` — `os.MkdirAll`
- `cmd/scraper/main.go:128` — `os.MkdirAll`

### Scraper doesn't run on startup

The scraper waits the full poll interval (30 minutes) before the first scrape. After a container restart you'd wait up to 30 minutes for fresh data.

---

## 4. Performance

### Sequential embedding generation — `ollama.go:116-141`

Embeddings are generated one chunk at a time with sequential HTTP calls. For a CV with many chunks, this adds unnecessary latency. Consider batching or parallel requests (with a semaphore).

### Regex compilation in `ExtractSalaryNumber` — `parsers/helpers.go:49`

```go
re := regexp.MustCompile(`£?\s*(\d+(?:\.\d+)?)\s*k?`)
```

This compiles the regex on every call. Move it to a package-level `var` like the other patterns in this file.

---

## 5. Code Quality

### Shadow variable in `handleScraperFiltersPost` — `server.go:383`

```go
if s, ok := item.(string); ok {
```

The variable `s` shadows the receiver `s *Server`. Use a different name like `str`.

### `.gitignore` contains a stray log line — `.gitignore:2`

```
492 INFO Collecting jobs.txt
```

This is an accidental paste from log output, not a gitignore pattern.

### Model type misplacement

`CVProfile` and `CVChunkWithEmbedding` are defined in `ollama.go`. These are model types that should live in the `models` package alongside `Job` and the existing `CVProfile`/`CVChunk` types in `models/cv.go`. Having duplicate definitions in different packages will cause confusion.

### `docker-compose.yml` uses deprecated `mem_limit`

Lines 20, 48, 74. Use `deploy.resources.limits.memory` for compose v3+ compatibility.

---

## 6. Testing

### Good coverage areas

- **Web server handlers:** health, config CRUD, sources CRUD, cooldown, schedule validation, data clear, CV status, dashboard helpers
- **Processor:** normalisation, salary parsing, location/work type standardisation, deduplication, scoring, chunking, lock files, CV parsing fallback
- **Scraper:** mock fetcher integration, filter application, context cancellation, enabled sources

### Missing test coverage

- **No tests for `error_aggregator.go`** — error summary and aggregation untested
- **No tests for `urlbuilder.go`** — URL construction for each source untested
- **No tests for `urlpool.go`** — URL rotation, failure tracking untested
- **No tests for `fetcher.go`** — HTTP retry logic, cooldown, backoff untested (would benefit from an HTTP test server)
- **No tests for `cv_chunker.go` edge cases** — overlap behavior, word boundary breaking
- **No tests for most individual parsers** — only reed and generic have test files; the other 15+ parsers are untested
- **No integration/end-to-end tests** — no tests that verify the full pipeline (scrape -> normalise -> score -> dashboard)

### Test quality issues

- `scraper_test.go:188` — `var _ = models.Job{}` unused import guard is a code smell; remove the import if tests don't need it
- Tests don't validate error responses (e.g., what happens on malformed JSON body?)

---

## Summary of Priority Fixes

| Priority | Issue | Location |
|----------|-------|----------|
| **P0** | CV upload `Read` after partial seek | `server.go:270-278` |
| **P0** | Vector DB path outside data volume | `vector_db.go:14` |
| **P0** | Processor has no graceful shutdown | `cmd/processor/main.go` |
| **P1** | Separate LLM model and embedding model config | `ollama.go` |
| **P1** | `rotateLog` deletes small logs + switch to 24hr retention | `cmd/scraper/main.go:179` |
| **P1** | Config save errors silently swallowed | `server.go` (multiple) |
| **P1** | Template `sync.Once` prevents error recovery | `server.go:212` |
| **P1** | Unbounded `io.ReadAll` on HTTP responses | `fetcher.go:128` |
| **P2** | Board toggle ignores board parameter | `server.go:560` |
| **P2** | Stray line in `.gitignore` | `.gitignore:2` |
| **P2** | Regex compiled per-call in `ExtractSalaryNumber` | `parsers/helpers.go:49` |
| **P2** | Scraper doesn't run on startup | `cmd/scraper/main.go` |
| **P3** | Add URL pool and fetcher tests | test files |
| **P3** | Move model types to `models` package | `ollama.go` |
