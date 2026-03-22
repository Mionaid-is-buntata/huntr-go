# PRD: Huntr-AI Python to Go Conversion

## Problem Statement

The Python-based Huntr-AI system suffers from:
- **Large Docker images** (~2.3GB total across 3 services)
- **Slow cold starts** due to Python interpreter and pip package loading
- **Complex dependency management** (pip, venv, PyTorch for sentence-transformers)
- **Limited concurrency model** (asyncio requires explicit async/await)
- **High RAM overhead** on resource-constrained Raspberry Pi 5 (8GB)

## Goals

1. **12x Docker image reduction** (2.3GB → ~180MB)
2. **Native concurrency** via goroutines (replacing Python asyncio)
3. **Eliminate Python runtime** and pip dependency management
4. **Static binaries** with zero runtime dependencies
5. **Full API and data format backwards compatibility**

## Non-Goals

- Changing the system architecture (3-service model stays)
- Adding new features during conversion
- Changing the config.json format
- Migrating away from Ollama
- Adding authentication (separate initiative)

## Success Criteria

- All 19 API endpoints return identical responses
- All 20+ job parsers produce matching output given same HTML input
- CV processing pipeline produces equivalent scored output
- Docker images build and run on ARM64 Pi 5
- Performance budgets maintained:
  - Full scrape cycle: < 30 minutes
  - Web response time: < 2 seconds
  - CV processing time: < 70 seconds
  - RAM during scraping: < 512 MB

## Target Users

- Same single user (job seeker) on home LAN
- No change in user interaction model

## Breaking Changes

- CV embeddings will use Ollama API instead of sentence-transformers
- Existing ChromaDB collections become incompatible
- User must re-upload CV after cutover to regenerate embeddings

## Timeline

16-24 days of focused work across 6 phases.
