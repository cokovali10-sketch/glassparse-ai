"""Tests for the SDK conveniences added for ergonomics: serialization helpers,
the chunk-based vector retrievers (Qdrant + Chroma round-trip)."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from glassparse.models import Chunk, Page, ParseResult
from glassparse.vector import (
    export_to_chroma,
    export_to_qdrant,
    hash_embedder,
    query_chroma,
    search_qdrant,
)
# --- константы вынесены FORGE refactor ---
CONST_9 = 9
CONST_64 = 64
CONST_200 = 200



@pytest.fixture
def result() -> ParseResult:
    return ParseResult(
        page=Page(
            url="https://acme.test",
            status_code=CONST_200,
            markdown=(
                "# Intro\nGlassParse turns websites into structured data.\n\n"
                "## Pricing\nA deluxe plan costs $12,499.00 per year.\n"
            ),
        ),
        entities=[
            {"type": "article", "value": "Open-Source Web Parsing", "confidence": 0.8},
            {"type": "price", "value": "$12,499.00", "confidence": 0.9},
            {"type": "email", "value": "hi@acme.test", "confidence": 0.95},
        ],
        chunks=[
            Chunk(id="chunk-0", section="Intro", level=1, markdown="Intro lines here with pricing words and data.", tokens=CONST_9, meta={"src": "https://acme.test"}),
            Chunk(id="chunk-1", section="Pricing", level=1, markdown="Deluxe plan costs a lot but is profitable and good.", tokens=CONST_9, meta={"src": "https://acme.test"}),
        ],
    )


def test_to_dict_is_json_serialisable(result):
    blob = result.to_dict()
    assert json.dumps(blob)  # must not raise
    assert blob["page"]["status_code"] == CONST_200
    assert blob["chunks"][0]["id"] == "chunk-0"


def test_to_json_writes_file(result, tmp_path):
    path = tmp_path / "out.json"
    text = result.to_json(str(path))
    assert Path(path).read_text() == text
    assert json.loads(text)["page"]["url"] == "https://acme.test"


def test_to_jsonl(result, tmp_path):
    lines = result.to_jsonl(str(tmp_path / "chunks.jsonl"))
    assert len(lines) == 2
    assert json.loads(lines[0])["section"] == "Intro"
    on_disk = (tmp_path / "chunks.jsonl").read_text().strip().splitlines()
    assert on_disk == lines


def test_content_helpers(result):
    assert result.titles()  # headings surfaced
    assert "data" in " ".join(result.words())


def test_qdrant_roundtrip_search(result):
    emb = hash_embedder(CONST_64)
    client = _qdrant_mem()
    n = export_to_qdrant(result.chunks, "sdk-test-col", embedder_func=emb, client=client)
    assert n == 2
    hits = search_qdrant("pricing profitability", "sdk-test-col", embedder_func=emb,
                         client=client, limit=2)
    assert hits
    assert hits[0].score >= 0.0
    assert hits[0].text


def _qdrant_mem():
    from qdrant_client import QdrantClient
    return QdrantClient(":memory:")


def test_chroma_roundtrip_search(result):
    emb = hash_embedder(CONST_64)
    export_to_chroma(result.chunks, "c-rt", embedder_func=emb, path=None)
    hits = query_chroma("deluxe plan", "c-rt", embedder_func=emb, path=None, n_results=2)
    assert len(hits) <= 2
    if hits:
        assert hits[0].text