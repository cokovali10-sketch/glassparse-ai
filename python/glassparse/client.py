"""Async client for the GlassParse AI engine.

``Client`` supports two transports:

* **HTTP** — talks to a running GlassParse server (``POST /api/parse`` gives a
  token-efficient JSON body). This is the recommended production path.
* **Local** — spawns the compiled ``glassparse-engine`` binary and parses its
  ``-json`` output, which is ideal for a zero-deploy / dev workflow.
"""

from __future__ import annotations

import asyncio
import json
import shutil
from pathlib import Path
from typing import Any, Awaitable, Callable, List, Optional, Union

import httpx

from .models import (
    Chunk,
    Entity,
    FetchOptions,
    Metadata,
    Page,
    ParseResult,
    RenderMode,
    Stats,
    __version__,
    stamp_fetch,
)


class Client:
    """Async facade over the engine. Safe to reuse across parses."""

    def __init__(
        self,
        *,
        base_url: str = "http://127.0.0.1:8080/api/parse",
        engine_bin: Optional[Union[str, Path]] = None,
        timeout: float = 60.0,
        headers: Optional[dict] = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.engine_bin = shutil.which("glassparse-engine") if engine_bin is None else str(engine_bin)
        self._http = httpx.AsyncClient(
            timeout=httpx.Timeout(timeout),
            headers={"User-Agent": f"glassparse-py/{__version__}", **(headers or {})},
        )

    async def aclose(self) -> None:
        await self._http.aclose()

    async def __aenter__(self) -> "Client":
        return self

    async def __aexit__(self, *exc: object) -> None:
        await self.aclose()

    # ------------------------------- public ---------------------------------
    async def parse(self, url: str, *, as_chunks: bool = True) -> ParseResult:
        """Extract a web page into a structured :class:`ParseResult`."""
        opts = FetchOptions(url=url)
        return await self.parse_opts(opts, as_chunks=as_chunks)

    async def parse_opts(self, opts: FetchOptions, *, as_chunks: bool = True) -> ParseResult:
        """Extract using explicit :class:`FetchOptions` (proxy, headless, ...)."""
        stamp_fetch()
        if self.engine_bin:
            return await self._parse_local(opts, as_chunks=as_chunks)
        data: dict = await self._parse_http(opts)
        data["chunks"] = data.get("chunks") or []
        result = ParseResult.model_validate(data)
        if as_chunks and not result.chunks:
            result.chunks = [Chunk(**c) for c in self._chunk_markdown(result.markdown)]
        return result

    async def parse_many(self, urls: list[str], *, concurrency: int = 8) -> list[ParseResult]:
        """Parse several URLs concurrently with a bounded worker pool."""
        sem = asyncio.Semaphore(concurrency)

        async def one(u: str) -> ParseResult:
            async with sem:
                return await self.parse(u)

        return await asyncio.gather(*(one(u) for u in urls))

    async def fetch_markdown(self, url: str, **opts: Any) -> str:
        """Parse *url* and return only its clean Markdown — a one-liner for
        pipelines that don't need the entity / chunk envelopes."""
        return (await self.parse_opts(FetchOptions(url=url, **opts))).markdown

    async def crawl_site(
        self,
        root: str,
        *,
        max_pages: int = 10,
        concurrency: int = 4,
        user_agent: Optional[str] = None,
        on_page: Optional[Callable[[dict], Awaitable[None]]] = None,
    ) -> List[dict]:
        """Crawl a site (via ``/api/crawl``), honouring robots.txt + same-host.

        Returns the parsed pages as raw dicts (URL, title, markdown,
        fingerprint, status, mode, entities). Pass ``on_page`` to stream each
        page as it lands. Requires the engine server to be running.
        """
        params = {"url": root, "max_pages": str(max_pages), "concurrency": str(concurrency)}
        if user_agent:
            params["user_agent"] = user_agent
        resp = await self._http.post(self._endpoint("crawl"), params=params)
        resp.raise_for_status()
        pages: List[dict] = resp.json()
        if on_page:
            for p in pages:
                await on_page(p)
        return pages

    async def main_text(self, url: str, *, user_agent: Optional[str] = None) -> str:
        """Return the dominant prose of *url* (readability-style)."""
        params: dict = {"url": url}
        if user_agent:
            params["user_agent"] = user_agent
        resp = await self._http.get(self._endpoint("main_text"), params=params)
        resp.raise_for_status()
        return resp.json().get("main_text", "")

    async def export_json(self, url: str, path: Union[str, Path], *, as_chunks: bool = True) -> ParseResult:
        """Parse *url* and write the full result (JSON) to *path*; returns it."""
        result = await self.parse(url, as_chunks=as_chunks)
        result.to_json(str(path))
        return result

    async def export_jsonl(self, url: str, path: Union[str, Path], *, as_chunks: bool = True) -> ParseResult:
        """Parse *url* and write its chunks as JSONL (one per line)."""
        if not as_chunks:
            raise ValueError("export_jsonl requires as_chunks=True")
        result = await self.parse(url, as_chunks=True)
        result.to_jsonl(str(path))
        return result

    # ------------------------------ transports -------------------------------
    def _endpoint(self, name: str) -> str:
        """Map a logical endpoint name to a URL under the server origin."""
        base = "/".join(self.base_url.split("/")[:-2])  # drop /api/parse
        return f"{base}/api/{name.replace('_', '-')}"

    # ------------------------------ transports -------------------------------
    async def _parse_http(self, opts: FetchOptions) -> dict:
        params = opts.to_query()
        resp = await self._http.post(self.base_url, params=params)
        resp.raise_for_status()
        return resp.json()

    async def _parse_local(self, opts: FetchOptions, *, as_chunks: bool = True) -> ParseResult:
        cmd = [
            self.engine_bin,
            "-url", opts.url,
            "-json",
        ]
        if opts.timeout_seconds:
            cmd += ["-timeout", str(opts.timeout_seconds)]
        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        out, err = await proc.communicate()
        if proc.returncode != 0:
            raise RuntimeError(f"engine failed (exit={proc.returncode}): {err.decode(errors='replace')}")
        payload = json.loads(out.decode("utf-8"))
        data = {
            "page": payload.get("page", {}),
            "entities": payload.get("entities", []),
            "chunks": payload.get("chunks", []),
        }
        result = ParseResult.model_validate(data)
        if as_chunks and not result.chunks:
            result.chunks = [Chunk(**c) for c in self._chunk_markdown(result.markdown)]
        return result

    # ---------------------------- client-side chunking ------------------------
    def _chunk_markdown(self, markdown: str, max_tokens: int = 1000) -> list[dict]:
        """Lightweight fallback chunker (used only when the engine had none)."""
        lines = markdown.splitlines()
        chunks: list[dict] = []
        buf: list[str] = []
        toks = 0
        for line in lines:
            toks += len(line) // 4 + 1
            if toks > max_tokens and buf:
                chunks.append(_mk_chunk(len(chunks), buf))
                buf, toks = [], 0
            buf.append(line)
        if buf:
            chunks.append(_mk_chunk(len(chunks), buf))
        return chunks


def _mk_chunk(i: int, lines: list[str]) -> dict:
    text = "\n".join(lines)
    return {"id": f"chunk-{i}", "section": "", "level": 0, "markdown": text,
            "tokens": len(text) // 4 + 1, "meta": {}}


async def example_usage() -> None:
    """Demonstrates the two-line API promised by the product pitch."""
    async with Client() as client:
        result = await client.parse("https://example.com")
        print(result.page.metadata.title)
        print("prices:", result.prices())


if __name__ == "__main__":
    asyncio.run(example_usage())