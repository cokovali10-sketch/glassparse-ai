<div align="center">

# GlassParse AI

**Layout-resilient web parser that converts any website into clean Markdown + structured JSON for RAG pipelines, LLMs, and vector databases**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python&logoColor=white)](https://python.org)
[![Kotlin](https://img.shields.io/badge/Kotlin-Compose_MP-7F52FF?logo=kotlin&logoColor=white)](https://kotlinlang.org)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://docker.com)
[![CI](https://img.shields.io/badge/CI-passing-brightgreen?logo=githubactions&logoColor=white)](.github/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-MIT-2ea44f)](LICENSE)

</div>

---

A hybrid `fetch → clean → extract → chunk` pipeline that turns any web page into LLM-ready content — immune to ads, scripts, and layout noise.

## What It Does

```
┌──────────┐   ┌───────────────┐   ┌───────────────┐   ┌──────────────┐
│ Fetcher  │──▶│ DOM Cleaner   │──▶│  Extractor    │──▶│ RAG Chunker  │
│ anti-bot │   │ → Markdown    │   │  self-healing │   │  H1–H4 aware │
│ http/    │   │   + HTML      │   │  text-density │   │  provenance  │
│ headless │   └───────────────┘   └──────┬────────┘   └──────┬───────┘
└──────────┘                             │ chunks            │ vectors
                                         ▼                   ▼
                                   ┌──────────────┐    ┌──────────────────┐
                                   │  JSON / CLI  │    │ Qdrant / Chroma  │
                                   └──────────────┘    └──────────────────┘
```

- **Go engine** — fast, low-memory, immune to ads/scripts/nav noise
- **Self-Healing Extractor** — survives layout + class changes by scoring blocks on text density + tokens + DOM path (not CSS classes)
- **Smart RAG Chunker** — heading-aware chunks with provenance metadata, ready for vector DBs
- **Anti-Bot Fetcher** — UA rotation, proxy support, transparent HTTP → headless switching for SPAs
- **Python SDK** — async client with CLI and vector DB exporters
- **Desktop Dashboard** — glassmorphic Compose Multiplatform GUI

---

## Quick Start

```bash
# One command — engine + Qdrant for RAG tests
docker compose up -d --build

# Verify
curl -s http://localhost:8080/api/healthz
# => {"status":"ok","version":"0.1.0"}

# Parse a page
curl -s -X POST "http://localhost:8080/api/parse?url=https://example.com"
```

---

## Features

| Feature | Description |
|---|---|
| **Hybrid DOM Cleaner** | Strips scripts/styles/SVG/ads/nav/cookie-banners; emits token-efficient Markdown + semantic HTML |
| **Self-Healing Selector** | Scores blocks by text density + tokens + DOM path — keeps extracting after a redesign |
| **Smart RAG Chunker** | Heading-aware (H1–H4) chunks with provenance, ready for ChromaDB / Qdrant / JSONL |
| **Site Crawler** | Concurrent, robots.txt-aware, same-host BFS crawl with fingerprint dedupe |
| **Main-Text Extraction** | Readability prose extraction — "give me the article" |
| **Content Fingerprinting** | Layout-stable content hashing for change detection and dedup |
| **Anti-Bot Fetching** | UA rotation, proxy support, transparent HTTP → headless switch for JS-rendered pages |
| **Zero-Config CLI** | `python -m glassparse <url> --markdown` — works immediately |
| **Python SDK** | Async client with vector DB export (Qdrant, Chroma) built in |
| **Desktop Dashboard** | Glassmorphic Kotlin/Compose UI with live JSON tree, Markdown viewer, crawler panel |

---

## HTTP API

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/api/parse` | Clean + extract + chunk one page (JSON envelope) |
| `POST` | `/api/crawl` | Crawl a site (robots.txt aware) → pages |
| `GET` | `/api/main-text` | Dominant prose of a page (readability) |
| `GET` | `/api/robots` | robots.txt policy: allowed, crawl_delay, sitemaps |
| `GET` | `/api/fingerprint` | Stable content hash for change detection / dedup |
| `GET` | `/api/healthz` | Liveness probe |
| `GET` | `/api/version` | Engine version |

### Examples

```bash
# Parse one page (clean + extract + chunk)
curl -s -X POST "http://localhost:8080/api/parse?url=https://example.com"

# Crawl a site (robots.txt-aware)
curl -s -X POST "http://localhost:8080/api/crawl?url=https://example.com&max_pages=20"

# Readability prose + content hash
curl -s "http://localhost:8080/api/main-text?url=https://example.com"
curl -s "http://localhost:8080/api/fingerprint?url=https://example.com"

# Check robots.txt policy
curl -s "http://localhost:8080/api/robots?url=https://example.com&path=/docs"
```

---

## Python SDK

### Install

```bash
pip install -e "python[rag]"    # SDK + Qdrant + Chroma exporters
```

### Usage

```python
import asyncio
from glassparse import Client

async def main():
    async with Client() as c:
        result = await c.parse("https://example.com")
        print(result.page.metadata.title)   # page title
        print(result.prices())               # extracted prices
        print(len(result.chunks))            # RAG chunk count

asyncio.run(main())
```

### Convenience Methods

```python
md     = await client.fetch_markdown(url)            # clean Markdown
text   = await client.main_text(url)                 # article prose
pages  = await client.crawl_site(root, max_pages=20) # whole-site crawl
result.to_json("out.json")                            # export
result.to_jsonl("chunks.jsonl")                       # JSONL export
```

### RAG → Vector DB

```python
from glassparse import export_to_qdrant, search_qdrant, hash_embedder

embed = hash_embedder(384)
export_to_qdrant(result.chunks, "my-collection", embedder_func=embed)
hits = search_qdrant("pricing", "my-collection", embedder_func=embed, limit=5)
for h in hits:
    print(round(h.score, 3), h.url, h.text[:60])
```

---

## Zero-Config CLI

```bash
python -m glassparse https://example.com --markdown
python -m glassparse https://example.com --pretty --out result.json
python -m glassparse crawl https://site.example --max-pages 20 --out pages.json
```

---

## Desktop Dashboard

Kotlin + Compose Multiplatform with a glassmorphism design: deep Indigo/Space-Gray/Purple gradient, frosted `GlassCard` panes, neon accents, Parse/Crawl tabs, site crawler panel, live latency & DOM-reduction bars, and a Markdown / RAG-chunk viewer.

```bash
cd desktop && ./gradlew run
```

---

## Project Structure

```
glassparse-ai/
├── engine/                          Go engine
│   ├── cmd/server/                  HTTP API server (graceful shutdown)
│   ├── cmd/glassparse/              CLI entry point
│   ├── internal/
│   │   ├── cleaner/                 DOM pruning + Markdown/HTML rendering
│   │   ├── extractor/               Self-healing extraction + readability + fingerprints
│   │   ├── fetcher/                 Anti-bot fetch + robots.txt
│   │   ├── crawler/                 Concurrent, robots-aware site crawler
│   │   └── chunker/                 RAG chunking + JSONL export
│   └── pkg/model/                   Shared wire types
├── python/                          Async SDK + CLI + RAG exporters
│   ├── glassparse/                  Client, models, vector, CLI
│   ├── examples/                    rag_pipeline.py
│   └── tests/                       pytest suite
├── desktop/                         Kotlin / Compose Multiplatform GUI
├── docker-compose.yml               Engine + Qdrant
└── .github/workflows/ci.yml         Three-stage CI (Go, Python, Kotlin)
```

---

## Architecture

### Go Engine Pipeline

```
Request → Fetcher (HTTP/headless) → Cleaner (DOM prune → Markdown)
       → Extractor (self-healing entities) → Chunker (H1–H4 aware chunks)
       → JSON Response
```

Each stage is **stateless and goroutine-safe** — instances are created per request. The engine uses only `net/http` stdlib for the server, with `goquery` for DOM parsing.

### Key Design Decisions

- **Self-healing extraction**: Scores content blocks by text density, token count, and DOM path depth — not CSS class names. Survives full site redesigns.
- **Heading-aware chunking**: Splits on H1–H4 boundaries, preserving parent heading context in each chunk's metadata for retrieval.
- **Graceful shutdown**: SIGINT/SIGTERM drains in-flight requests with a 10s timeout.

---

## Development

### Prerequisites

| Tool | Min Version | Purpose |
|---|---|---|
| [Go](https://go.dev) | 1.22 | Engine |
| [Python](https://python.org) | 3.11 | SDK + CLI |
| [JDK](https://adoptium.net) | 21 | Desktop GUI |
| [Docker](https://docker.com) | 20+ | One-command stack |

### Build & Run

```bash
# Engine (from source)
cd engine && go run ./cmd/server -addr 127.0.0.1:8080

# Python SDK
cd python && pip install -e ".[dev]"
python -m glassparse https://example.com --markdown

# Desktop
cd desktop && ./gradlew run
```

### Docker

```bash
docker compose up -d --build           # engine on :8080, Qdrant on :6333
docker build -f engine/Dockerfile -t glassparse-engine:latest .  # standalone image
```

---

## Testing

### Go (Engine)

```bash
cd engine && go test ./... -count=1 -race
```

Covers: cleaner (pruning, DOM shrink, word-boundary safety), extractor (prose, links, fingerprints), fetcher (anti-bot + robots.txt), crawler (discovery, dedupe, MaxPages, robots), chunker (heading-aware chunks, JSONL round-trip), server (full HTTP API against in-memory fakes — no external network).

### Python (SDK)

```bash
cd python && pytest -q
```

Covers: models, serialization, client (mock transport), RAG round-trips (in-memory Qdrant + Chroma).

### Desktop (Kotlin)

```bash
cd desktop && ./gradlew test
```

---

## CI/CD

Three-stage pipeline on every push/PR (`.github/workflows/ci.yml`):

| Stage | Runner | Steps |
|---|---|---|
| Engine (Go) | ubuntu-latest | `go build` → `go vet` → `go test` |
| SDK (Python) | ubuntu-latest | `pip install` → `pytest` |
| Desktop (Kotlin) | ubuntu-latest | `gradle compileKotlin` |

---

## Configuration

| Option | Where | Default | Description |
|---|---|---|---|
| `-addr` | Engine flag | `127.0.0.1:8080` | HTTP listen address |
| `-proxy` | Engine flag | — | Upstream proxy (UA rotation still applies) |
| `base_url` | `Client(...)` | `http://127.0.0.1:8080/api/parse` | Engine endpoint |
| `engine_bin` | `Client(...)` | auto | Run local Go binary instead of HTTP |
| `timeout` | `Client(...)` | `60.0` s | Request timeout |

---

## Extending

- **New endpoint**: add a `Handler` method in `engine/cmd/server/main.go`, register on the mux, add tests in `main_test.go`, add SDK helper in `python/glassparse/client.py`.
- **New vector backend**: mirror `export_to_qdrant` in `python/glassparse/vector.py`, add to `__all__` and `__init__.py`.
- **Custom extractors**: stateless & goroutine-safe — emit `model.Entity` with a `Confidence` score.

---

## Roadmap

- HTTP + headless browser strategy tuned per-site via a rules engine
- Streaming crawl → chunk → embed (no full-site memory hold)
- Plugin system: custom heuristics written by users (Python or WASM)

---

## License

[MIT](LICENSE)
