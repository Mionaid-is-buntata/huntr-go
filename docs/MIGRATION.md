# Migration Guide: Python to Go Cutover

## Breaking Changes

### 1. CV Embeddings (High Impact)
- **Before**: sentence-transformers `all-MiniLM-L6-v2` (384-dim vectors)
- **After**: Ollama `/api/embeddings` endpoint (model-dependent dimensions)
- **Impact**: Existing ChromaDB collections are incompatible
- **Action**: Re-upload CV after cutover to generate new embeddings
- **Data loss**: None. Raw CV files preserved in `/data/cv/cv-processed/`

### 2. Vector Database (Medium Impact)
- **Before**: ChromaDB (Python library, SQLite-backed)
- **After**: chromem-go (in-process Go library, file-backed)
- **Impact**: Different storage format. Old `/chromadb/` data not readable
- **Action**: New chromem-go data stored in `/data/chromadb-go/`

## Directory Layout

```
app/                    # Legacy Python version (reference only)
go-migration/           # Go version (deployable bundle)
├── src/                # Go source code
├── docker/             # Dockerfiles (multi-stage Go builds)
├── docker-compose.yml  # Production compose
├── .env.example        # Environment template
└── docs/               # Migration documentation
```

## Production Deployment (Pi)

### Before Cutover
The production Pi (`finlay.local`) auto-deploys from `app/` every 10 minutes via systemd timer.

### Cutover Procedure

1. **SSH to Pi**: `ssh finlay.local`
2. **Stop Python services**: `cd ~/Huntr-AI/app && docker compose down`
3. **Create .env**: `cp ~/Huntr-AI/go-migration/.env.example ~/Huntr-AI/go-migration/.env`
   Then edit to add email credentials (or copy from `app/.env`)
4. **Deploy Go services**: `cd ~/Huntr-AI/go-migration && docker compose up -d --build`
5. **Verify health**: `curl http://localhost:5000/health`
6. **Re-upload CV**: Upload DOCX via dashboard to trigger re-processing
7. **Verify dashboard**: Check scored jobs display correctly
8. **Monitor logs**: `docker compose logs -f`
9. **Update auto-deploy timer**: Change working directory from
   `/home/campbell/Huntr-AI/app` to `/home/campbell/Huntr-AI/go-migration`

### Container Name Conflicts
Both `app/` and `go-migration/` use the same container names (`huntr-scraper`,
`huntr-processor`, `huntr-web`). Always stop one set before starting the other.

## Rollback Procedure

If issues are found after cutover:

1. **Stop Go services**: `cd ~/Huntr-AI/go-migration && docker compose down`
2. **Restart Python**: `cd ~/Huntr-AI/app && docker compose up -d --build`
3. **Revert auto-deploy timer** to use `app/` directory
4. **Verify**: Python services resume with existing data (JSON files unchanged)

Rollback is safe because:
- Job JSON files (raw, normalised, scored) are format-compatible
- Config.json is unchanged
- NAS mount paths are unchanged
- Only embeddings/vector DB need regeneration

## Data Compatibility Matrix

| Data Type | Format | Compatible? |
|-----------|--------|-------------|
| config.json | JSON | Yes |
| jobs_raw_*.json | JSON | Yes |
| jobs_normalised_*.json | JSON | Yes |
| jobs_scored_*.json | JSON | Yes |
| cv_profile.json | JSON | Yes |
| notified_jobs.json | JSON | Yes |
| scraper_errors.json | JSON | Yes |
| ChromaDB collections | SQLite | No (replaced by chromem-go) |
| Scraper logs | Plain text | Yes (same format) |
