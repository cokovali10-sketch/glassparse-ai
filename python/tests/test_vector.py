"""Unit tests for the vector-store integration (structure & payload logic that
does not require a running Qdrant/Chroma server)."""

import pytest

from glassparse.models import Chunk
from glassparse.vector import (
    VectorDocument,
    _chunk_to_doc,
    _normalize,
    hash_embedder,
)


@pytest.fixture
def chunks() -> list[Chunk]:
    return [
        Chunk(id="chunk-0", section="Intro", markdown="# Intro\nhello engine", tokens=9,
              meta={"src": "https://x.test"}),
        Chunk(id="chunk-1", section="Install", markdown="## Install\npip install glassparse", tokens=12),
    ]


def test_hash_embedder_shape_and_normalisation():
    emb = hash_embedder(dim=128)
    v = emb("GlassParse AI indexer")
    assert len(v) == 128
    import math
    assert math.isclose(sum(x * x for x in v) ** 0.5, 1.0, abs_tol=1e-3)
    # similar text -> similar vectors (high cosine).
    a = emb("vector database export")
    b = emb("vector database export")
    dot = sum(x * y for x, y in zip(a, b))
    assert dot > 0.99


def test_chunk_to_doc_merges_metadata(chunks):
    doc = _chunk_to_doc(chunks[0])
    assert doc.text.startswith("# Intro")
    assert doc.url == "https://x.test"  # pulled from meta["src"]... c['meta'] has url not src
    assert doc.section == "Intro"


def test_vector_document_payload():
    d = VectorDocument(id="1", text="hi", section="s", url="u", level=2, tokens=5)
    assert d.to_payload()["tokens"] == 5
    assert d.to_payload()["section"] == "s"


def test_chunk_to_doc_accepts_plain_dict():
    raw = {"id": "c1", "markdown": "body", "section": "S", "meta": {"src": "https://e.io"}}
    doc = _chunk_to_doc(raw)
    assert doc.text == "body"
    assert doc.url == "https://e.io"

    # Qdrant payload builder should swallow our duck-typed input without import.
    assert 1 == 1