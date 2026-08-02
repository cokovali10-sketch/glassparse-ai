"""End-to-end integration check of the Python SDK against the running Go
engine server.

Prerequisite (run from repo root):
  1. cd engine && go build -o bin/glassparse-server.exe ./cmd/server
  2. Start a local page server serving ./engine/sample.html, e.g.:
        python -m http.server 8010 -d engine
  3. Start the engine server:
        ./bin/glassparse-server.exe -addr 127.0.0.1:8080
  4. Run this script:  python python/tests/e2e.py
"""

import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from glassparse import Client

TARGET = "http://127.0.0.1:8010/sample.html"
ENGINE = "http://127.0.0.1:8080/api/parse"


async def main() -> None:
    async with Client(base_url=ENGINE) as client:
        r = await client.parse(TARGET, as_chunks=True)
        assert r.page.metadata.title, "title metadata missing"
        assert r.markdown, "markdown empty"
        assert r.page.stats.nodes_after < r.page.stats.nodes_before, "no DOM shrink"
        assert r.prices(), "no prices extracted"
        assert r.emails(), "no emails extracted"
        assert r.chunks, "no chunks produced"
        print("E2E OK")
        print("  title   :", r.page.metadata.title)
        print("  prices  :", r.prices())
        print("  emails  :", r.emails())
        print("  nodes   : %d -> %d (ratio %.2f)"
              % (r.page.stats.nodes_before, r.page.stats.nodes_after, r.page.stats.compression_ratio))
        print("  chunks  : %d" % len(r.chunks))


if __name__ == "__main__":
    asyncio.run(main())