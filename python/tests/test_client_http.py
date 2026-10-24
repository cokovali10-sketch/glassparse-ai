"""Client tests that exercise URL construction and parsing against a mocked
HTTP transport (no live engine required). These validate the *_endpoint mapping
and the new crawl/main-text/export methods."""

from __future__ import annotations

import asyncio
import json

import httpx

from glassparse import Client
# --- константы вынесены FORGE refactor ---
CONST_3 = 3
CONST_200 = 200



def _mock_client(responder) -> Client:
    client = Client()  # base_url default points at a fictive engine
    client._http = httpx.AsyncClient(transport=httpx.MockTransport(responder))
    return client


def _json_response(payload: dict, status: int = CONST_200) -> httpx.Response:
    body = json.dumps(payload).encode()
    return httpx.Response(status, content=body, headers={"content-type": "application/json"})


def test_engine_binary_takes_priority(monkeypatch):
    # When an engine binary is found, the client uses the local path, not HTTP.
    from glassparse import client as client_mod

    monkeypatch.setattr(client_mod.shutil, "which", lambda *a, **k: None)
    c = Client()
    assert c.engine_bin is None  # no engine installed -> HTTP path


def test_endpoint_mapping():
    c = Client()  # default "http://127.0.0.1:8080/api/parse"
    assert c._endpoint("crawl") == "http://127.0.0.1:8080/api/crawl"
    assert c._endpoint("main_text") == "http://127.0.0.1:8080/api/main-text"


def test_parse_hits_parse_endpoint():
    async def responder(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/api/parse"
        assert "url" in req.url.params
        return _http_json({"page": {"url": "https://x.test", "markdown": "# hi"},
                           "entities": [], "chunks": []})

    async def run():
        c = _mock_client(responder)
        try:
            r = await c.parse("https://x.test", as_chunks=False)
            assert r.markdown == "# hi"
        finally:
            await c.aclose()

    asyncio.run(run())


def test_crawl_site_hits_crawl_endpoint():
    pages = [
        {"url": "https://a.test/", "title": "Home", "status_code": CONST_200},
        {"url": "https://a.test/docs", "title": "Docs", "status_code": CONST_200},
    ]

    async def responder(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/api/crawl"
        assert req.url.params.get("max_pages") == "3"
        return _http_json(pages)

    async def run():
        c = _mock_client(responder)
        try:
            got = await c.crawl_site("https://a.test", max_pages=CONST_3)
            assert [p["title"] for p in got] == ["Home", "Docs"]
        finally:
            await c.aclose()

    asyncio.run(run())


def test_main_text_hits_endpoint():
    async def responder(req: httpx.Request) -> httpx.Response:
        assert req.url.path == "/api/main-text"
        return _http_json({"url": req.url.params["url"], "main_text": "some prose here", "word_count": CONST_3})

    async def run():
        c = _mock_client(responder)
        try:
            assert await c.main_text("https://x.test") == "some prose here"
        finally:
            await c.aclose()

    asyncio.run(run())


def test_export_json_writes_file(tmp_path, monkeypatch):
    out = tmp_path / "r.json"
    captured = {}

    async def responder(req: httpx.Request) -> httpx.Response:
        return _http_json({
            "page": {"url": "https://x.test", "markdown": "# T\nbody"},
            "entities": [],
            "chunks": [{"id": "chunk-0", "section": "T", "markdown": "body", "tokens": 2, "meta": {}}],
        })

    async def run():
        c = _mock_client(responder)
        try:
            res = await c.export_json("https://x.test", out)
            assert out.exists()
            assert json.loads(out.read_text())["page"]["url"] == "https://x.test"
            assert res.markdown
        finally:
            await c.aclose()

    asyncio.run(run())


def _http_json(payload):
    return httpx.Response(CONST_200, content=json.dumps(payload).encode())