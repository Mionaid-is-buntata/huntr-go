# Go Migration Decisions Log

## 2026-03-21 — Initial Architecture Decisions

### D-001: Web Framework → chi
**Decision**: Use github.com/go-chi/chi/v5
**Rationale**: Idiomatic, lightweight, stdlib-compatible. No magic, composes with standard middleware.
**Alternatives considered**: gin (reflection overhead, custom context), echo (similar issues), stdlib net/http mux (lacks middleware chaining)

### D-002: Headless Chrome → rod
**Decision**: Use github.com/go-rod/rod
**Rationale**: Higher-level API than chromedp, manages browser lifecycle, stealth mode built-in.
**Alternatives considered**: chromedp (lower-level, more boilerplate)

### D-003: HTML Parsing → goquery
**Decision**: Use github.com/PuerkitBoy/goquery
**Rationale**: CSS selector-based, direct 1:1 mapping from BeautifulSoup4.

### D-004: Vector DB → chromem-go
**Decision**: Use github.com/philippgille/chromem-go (in-process)
**Rationale**: Pure Go, zero infrastructure, small dataset (~10-20 CV chunks). Eliminates ChromaDB container.
**Breaking change**: Existing ChromaDB collections incompatible. Must re-upload CV on cutover.

### D-005: Embeddings → Ollama API
**Decision**: Call Ollama /api/embeddings instead of sentence-transformers
**Rationale**: Eliminates PyTorch dependency (~500MB). Ollama already deployed on Pi.
**Breaking change**: Different embedding vectors from different model. Collections not backwards-compatible.

### D-006: DOCX Parsing → Custom ZIP+XML
**Decision**: Parse DOCX as ZIP archive, extract word/document.xml, parse paragraphs
**Rationale**: ~50 lines of Go, zero dependencies. Current code only reads paragraphs and table cells.

### D-007: Logging → log/slog
**Decision**: Use Go 1.21+ stdlib slog
**Rationale**: Structured logging, JSON output, no external dependency needed on Pi.

### D-008: Project folder structure
**Decision**: All Go migration work in `go-migration/` folder, separate from Python `app/`
**Rationale**: Clean separation, no risk to production Python code during development.
