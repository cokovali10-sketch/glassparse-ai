<div align="center">

# 🧊 GlassParse AI

**Super-efficient, layout-resilient AI web parser** that converts any website into
clean **Markdown + structured JSON** for RAG pipelines, LLMs and vector databases.

[![Go](https://img.shields.io/badge/Go-%231.22+-00ADD8.svg?&logo=go&logoColor=white)](engine/)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python&logoColor=white)](python/)
[![Kotlin](https://img.shields.io/badge/Kotlin-Compose_MP-7F52FF?logo=kotlin&logoColor=white)](desktop/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![RAG](https://img.shields.io/badge/RAG-Qdrant·Chroma-a855f7?logo=qdrant&logoColor=white)](python/glassparse/vector.py)
[![License](https://img.shields.io/badge/License-MIT-2ea44f)](LICENSE)

</div>

## What it is

A hybrid `fetch → clean → extract → chunk` pipeline:

- a fast, low-memory **Go engine**, immune to ads/scripts/nav noise,
- a **Self-Healing extractor** that survives layout+class changes,
- a **Smart RAG-Chunker** (H1–H4) ready for vector DBs,
- an **anti-bot fetcher** with static → headless switching,
- an async **Python SDK** and a **glassmorphic Compose dashboard**.

```ascii
┌──────────┐   ┌───────────────┐   ┌───────────────┐   ┌──────────────┐
│ Fetcher  │──▶│ DOM Cleaner   │──▶│  Extractor    │──▶│ RAG Chunker  │
│ anti-bot │   │ →Markdown/JSON│   │  self-healing │   │  H1..H4      │
│ http (Go)│   └───────────────┘   └──────┬────────┘   └──────┬───────┘
│ headless │                             │ chunks            │ vectors
└──────────┘                             ▼                   ▼
                                  ┌──────────────┐    ┌──────────────────┐
                                  │ GlassDesktop │    │ Qdrant / Chroma  │
                                  └──────────────┘    └──────────────────┘
                                 Python SDK (asyncio) ────────────┘
```

## Quick start (one line)

```bash
docker compose up -d --build
```

This starts the Go engine on `:8080` and a local **Qdrant** for RAG tests:

```bash
curl -s -X POST "http://localhost:8080/api/parse?url=https://example.com"
curl -s http://localhost:8080/api/healthz   # => {"status":"ok", ...}
```

## Repository layout

```
├─ engine/            Go engine (cleaner · extractor · fetcher · chunker · server)
│  └─ cmd/{glassparse,server}/main.go
├─ python/            Async SDK + RAG vector exporters
│  ├─ glassparse/{client,models,vector}.py
│  └─ examples/rag_pipeline.py
├─ desktop/           Kotlin · Compose Multiplatform glass dashboard
└─ docker-compose.yml engine + Qdrant
```

## Python SDK

Two-line creed:

```python
import asyncio
from glassparse import Client

result = asyncio.run(Client().parse("https://example.com"))
print(result.page.metadata.title)      # parsed metadata
print(result.prices())                 # extracted prices
```

Install:

```bash
pip install -e "python[rag]"            # SDK + Qdrant + Chroma exporters
```

## RAG → vector DB

Export chunks straight into **Qdrant** or **ChromaDB** (hash-embedder built in,
swap in `sentence-transformers` / OpenAI / Ollama):

```python
from glassparse import export_to_qdrant, search_qdrant, hash_embedder

embed = hash_embedder(384)
export_to_qdrant(result.chunks, "my-collection", embedder_func=embed)
hits = search_qdrant("how much does the deluxe plan cost?", "my-collection",
                     embedder_func=embed, limit=5)   # ranked SearchHit list
```

Full end-to-end example (parse → chunk → load):

```bash
docker compose up -d --build
python python/examples/rag_pipeline.py --url https://example.com --store qdrant
```

## HTTP API

Docker / `go run ./cmd/server` serves the engine over HTTP:

| Method | Endpoint            | Purpose                                             |
|--------|---------------------|-----------------------------------------------------|
| POST   | `/api/parse`        | clean + extract + chunk one page (JSON envelope)    |
| POST   | `/api/crawl`        | crawl a site (robots.txt aware) → pages             |
| GET    | `/api/main-text`    | dominant prose of a page (readability)              |
| GET    | `/api/robots`       | robots.txt policy: `allowed`, `crawl_delay`, sitemaps |
| GET    | `/api/fingerprint`  | stable content hash for change-detection / dedupe   |
| GET    | `/api/healthz`      | liveness probe                                      |

```bash
curl -s -X POST "http://localhost:8080/api/parse?url=https://example.com"
curl -s "http://localhost:8080/api/main-text?url=https://example.com"
curl -s -X POST "http://localhost:8080/api/crawl?url=https://example.com&max_pages=20"
```

## Python SDK

Two-line creed:

```python
import asyncio
from glassparse import Client

result = asyncio.run(Client().parse("https://example.com"))
print(result.page.metadata.title)      # parsed metadata
print(result.prices())                 # extracted prices
```

Conveniences on top of the two-liner:

```python
md     = await client.fetch_markdown(url)          # clean Markdown, no envelope
text   = await client.main_text(url)               # article prose (readability)
pages  = await client.crawl_site(root, max_pages=20)       # whole-site crawl
prices = result.prices() / result.emails() / result.words() # quick filters
result.to_json("out.json")  /  result.to_jsonl("chunks.jsonl")  # export
```

Install:

```bash
pip install -e "python[rag]"            # SDK + Qdrant + Chroma exporters
```

## Zero-config CLI

```bash
python -m glassparse https://example.com --markdown
python -m glassparse https://example.com --pretty --out result.json
python -m glassparse crawl https://site.example --max-pages 20 --out pages.json
```

## Desktop dashboard

Kotlin + Compose Multiplatform with a strict premium **Glassmorphism** design:
deep Indigo/Space-Gray/Purple gradient, frosted `GlassCard` panes, thin glowing
1 px borders, neon accents, **Parse / Crawl tabs**, a site crawler panel, live
latency & DOM-reduction bars and a Markdown / RAG-chunk viewer.

```bash
cd desktop && ./gradlew run
```

## Features

- 🧹 **Hybrid DOM Cleaner** — strips scripts/styles/SVG/ads/nav/cookie-banners;
  emits token-efficient Markdown + semantic HTML (word-boundary safe).
- 🔁 **Self-Healing Selector Engine** — scores blocks by text density + tokens +
  DOM-path (not CSS classes), so it keeps extracting after a redesign.
- 📚 **Smart RAG-Chunker** — heading-aware chunks with provenance metadata,
  ready for ChromaDB / Qdrant / **JSONL** (`/api`, `ExportJSONL`).
- 🕸 **Site Crawler** — concurrent, robots.txt-aware, same-host BFS crawl with
  fingerprint dedupe (feed it to chunk → vector).
- 🎯 **Main-Text & Fingerprints** — readability prose extraction, layout-stable
  content hashing for change detection.
- 🛡 **Anti-Bot & Dynamic Content** — UA rotation, proxy support, transparent
  HTTP→headless switch for SPA/React/Vue.
- 🎨 **Glass Dashboard** — tabs, live JSON tree, Markdown editor, crawler panel,
  chunk list, charts.

## Tests

```bash
cd engine && go test ./... && go vet ./...   # cleaner/extractor/fetcher(+robots)/crawler/chunker/server
cd python && pytest -q                        # incl. Qdrant & Chroma in-memory round-trips + retrieval
cd desktop && ./gradlew test
```

CI runs all three suites on every push (see `.github/workflows/ci.yml`).

---

## Developer guide

Everything you need to build, run, debug and extend GlassParse AI locally.

### 1. Prerequisites

| Tool           | Min version | Why                                          |
|----------------|-------------|----------------------------------------------|
| [Go](https://go.dev)     | 1.22        | Engine (fetcher, cleaner, extractor, chunker, crawler, server) |
| [Python](https://python.org) | 3.11   | Async SDK, CLI, RAG exporters/retrievers |
| [JDK](https://adoptium.net)   | 21       | Compose Desktop GUI (Kotlin)                 |
| [Docker](https://docker.com)  | 20+      | One-command stack (engine + Qdrant)          |

Optional: an LLM/embedding service if you swap the hash embedder for
`sentence-transformers`, OpenAI or Ollama in production.

### 2. Clone & quick start

```bash
git clone https://github.com/<you>/glassparse-ai && cd glassparse-ai

# Fastest: whole stack (engine + Qdrant) in one command
docker compose up -d --build          # engine on :8080, Qdrant on :6333
curl -s http://localhost:8080/api/healthz     # => {"status":"ok"}

# Or run the engine directly from source (no Docker)
cd engine && go run ./cmd/server -addr 127.0.0.1:8080
```

### 3. Run the engine

```bash
cd engine
go build -o bin/server ./cmd/server

./bin/server -addr 127.0.0.1:8080            # default listen address
./bin/server -addr 0.0.0.0:8080 -proxy http://my-proxy:3128
```

- Graceful shutdown on `SIGINT`/`SIGTERM` (Ctrl-C, `docker stop`).
- Build a static, non-root, ~18 MB image with `docker build -f engine/Dockerfile engine`.

Endpoints — see the [HTTP API table](#http-api):

```bash
# parse one page (clean + extract + chunk)
curl -s -X POST 'http://localhost:8080/api/parse?url=https://example.com'

# crawl a site (robots.txt-aware)
curl -s -X POST 'http://localhost:8080/api/crawl?url=https://example.com&max_pages=20'

# readability prose + content hash
curl -s 'http://localhost:8080/api/main-text?url=https://example.com'
curl -s 'http://localhost:8080/api/fingerprint?url=https://example.com'
curl -s 'http://localhost:8080/api/robots?url=https://example.com&path=/docs'
```

### 4. Install the Python SDK

```bash
cd python
python -m venv .venv && source .venv/bin/activate   # (Windows: .venv\Scripts\activate)
pip install -e ".[dev]"                             # SDK + test/QA toolchain
pip install -e ".[rag]"                             #   + Qdrant & Chroma
```

Use the CLI immediately:

```bash
python -m glassparse https://www.iana.org/help/example-domains --markdown
python -m glassparse https://example.com --pretty --out result.json
python -m glassparse crawl https://example.com --max-pages 10 --out pages.json
```

The console entry point `glassparse` is also installed by `[dev]`.

### 5. Programmatic SDK

```python
import asyncio
from glassparse import Client, export_to_qdrant, search_qdrant, hash_embedder

async def main():
    async with Client(base_url="http://127.0.0.1:8080/api/parse") as c:
        r = await c.parse("https://example.com")
        print(r.page.metadata.title, r.prices(), len(r.chunks))

        embed = hash_embedder(384)                    # zero-dependency embedder
        export_to_qdrant(r.chunks, "docs", embedder_func=embed)   # -> Qdrant
        hits = search_qdrant("pricing", "docs", embedder_func=embed)
        for h in hits:
            print(round(h.score, 3), h.url, h.text[:60])

asyncio.run(main())
```

### 6. Run & extend the Desktop app

```bash
cd desktop
./gradlew run                                     # dev run
./gradlew packageDistributionForCurrentOS         # native bundle
./gradlew test                                    # Kotlin tests
```

UI source lives in `desktop/src/main/kotlin/io/glassparse/dashboard/`
(`ui/App.kt`, `ui/GlassCard.kt`, `ui/Theme.kt`, `data/EngineClient.kt`).
The app talks to the engine on `127.0.0.1:8080`; change `baseUrl` in
`EngineClient.kt` to point elsewhere.

### 7. Configuration reference

| Option | Where | Default | Meaning |
|--------|-------|---------|---------|
| `-addr` | engine flags | `127.0.0.1:8080` | HTTP listen address |
| `-proxy` | engine flags | —           | upstream proxy (UA rotation still applies) |
| `base_url` | `Client(...)` | `http://127.0.0.1:8080/api/parse` | engine endpoint |
| `engine_bin` | `Client(...)` | auto | run a local Go binary instead of HTTP |
| `timeout` | `Client(...)` | `60.0` s | request timeout |
| `docker-compose.yml` | compose | — | service ports / volumes / healthchecks |

### 8. Project layout

```
engine/                  Go engine
  cmd/server/            HTTP API server (graceful shutdown)
  cmd/glassparse/        CLI entry point
  internal/cleaner/      pruning + Markdown/HTML rendering
  internal/extractor/    self-healing extraction + readability + fingerprints
  internal/fetcher/      anti-bot fetch + robots.txt
  internal/crawler/      concurrent, robots-aware site crawler
  internal/chunker/      RAG chunking + JSONL export
  pkg/model/             shared wire types
python/
  glassparse/            SDK (client, models, vector, __main__ CLI)
  examples/              rag_pipeline.py and friends
  tests/                 pytest suite
desktop/                 Kotlin / Compose Multiplatform GUI
docker-compose.yml       engine + Qdrant
.github/workflows/ci.yml three-stage CI
```

### 9. Testing & reliability

The project ships with deliberately broad tests; every layer is covered:

**Engine (Go) — `go test ./... -race`**
- `cleaner`: pruning, DOM shrink, inline word-boundary safety.
- `extractor`: prose extraction, links, stable/sensitive fingerprints.
- `fetcher`: anti-bot + `robots.txt` (groups, wildcards, `$` anchors,
  crawl-delay, sitemaps).
- `crawler`: discovery, dedupe, `MaxPages` budget, robots honour, canonical
  URL normalisation.
- `chunker`: heading-aware chunks, JSONL round-trip / byte-exact export.
- `server`: full HTTP API driven against an in-memory fake upstream
  (parse / main-text / robots / fingerprint / crawl) — no external network.

```bash
cd engine && go test ./... -count=1 -race
```

**SDK (Python) — `pytest`**
- Models, serialization (`to_json`, `to_jsonl`), content helpers.
- Client against a **mock transport** (URL mapping, crawl/main-text/export).
- RAG: real in-memory **Qdrant** and **Chroma** insert→iterate/search round-trips.

```bash
cd python && pip install -e ".[dev]" && pytest
```

**Pro tips**
- Use `go test -race` to catch data races in the crawler (it is concurrent).
- `pytest --cov=glassparse` for coverage (pytest-cov is in `[dev]`).
- CI runs every suite on each push/PR; fix any red before merging.

### 10. Extending

- **New engine endpoint**: add a `Handler` method in `engine/cmd/server/main.go`,
  register it on the `mux`, then add a matching test in `main_test.go` and an
  SDK helper in `python/glassparse/client.py` (with a mock-transport test).
- **New backend**: mirror `export_to_qdrant`/`search_qdrant` in
  `glassparse/vector.py`; add it to `__all__` and `__init__.py`.
- **Self-healing**: everything is stateless & goroutine-safe; new extractors
  only need to emit `model.Entity` with a `Confidence`.
- Keep the wire models in `engine/pkg/model` and `python/glassparse/models.py`
  in sync; the dashboard DTOs live in `desktop/.../data/Models.kt`.

### 11. Release / build hygiene

```bash
# engine (static, non-root, tiny)
docker build -f engine/Dockerfile -t glassparse-engine:latest .

# wheel
cd python && python -m build

# desktop installer
cd desktop && ./gradlew packageDistributionForCurrentOS
```

Run `go vet ./...` and `git grep -n TODO` before committing; never commit
secrets (put keys in env vars or Docker secrets).

---

## Roadmap ideas
- HTTP+headless browser strategy tuned per-site via a rules engine.
- Streaming crawl → chunk → embed (no full-site memory hold).
- Plugin scene: custom heuristics written by users (Python or WASM).

## License

[MIT](LICENSE)