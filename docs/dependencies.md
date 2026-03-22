# Go Dependencies

## Module Definition

```
module github.com/campbell/huntr-ai

go 1.22
```

## External Dependencies

| Package | Version | Purpose | Replaces (Python) |
|---------|---------|---------|-------------------|
| github.com/go-chi/chi/v5 | v5.x | HTTP router | flask |
| github.com/go-rod/rod | v0.x | Headless Chrome | selenium |
| github.com/PuerkitBoy/goquery | v1.x | HTML parsing (CSS selectors) | beautifulsoup4 |
| github.com/philippgille/chromem-go | v0.x | In-process vector DB | chromadb |
| github.com/shirou/gopsutil/v4 | v4.x | RAM/CPU monitoring | psutil |
| github.com/stretchr/testify | v1.x | Test assertions | pytest |

## Standard Library Usage (no external dependency needed)

| Go Stdlib Package | Purpose | Replaces (Python) |
|-------------------|---------|-------------------|
| net/http | HTTP client + server | aiohttp, requests |
| html/template | Template rendering | jinja2 |
| encoding/json | Config and data serialisation | json (stdlib) |
| net/smtp | Email notifications | smtplib |
| log/slog | Structured logging | logging |
| archive/zip + encoding/xml | DOCX parsing | python-docx |
| sync, context | Concurrency primitives | asyncio |
| sort | Job ranking | pandas sort |
| crypto/tls | SMTP TLS | ssl |
| os, filepath | File operations | os, pathlib |
| time | Scheduling, timestamps | datetime |
| regexp | Salary/title parsing | re |
| strings | Text processing | str methods |
