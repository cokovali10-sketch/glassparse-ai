"""Live integration tests for the vector exporters against in-memory backends
(Qdrant ``:memory:`` and Chroma's in-memory client). Skipped automatically if
the optional libraries are not installed."""

import pytest

from glassparse import hash_embedder
from glassparse.models import Chunk

qdrant = pytest.importorskip("qdrant_client")


@pytest.fixture
def chunks() -> list[Chunk]:
    return [
        Chunk(id="chunk-0", section="Intro", markdown="# Intro\nGlassParse cleans HTML to markdown.",
              tokens=9, meta={"src": "https://x.test", "section": "Intro"}),
        Chunk(id="chunk-1", section="Install", markdown="## Install\npip install glassparse and run.",
              tokens=12, meta={"src": "https://x.test"}),
    ]


def test_qdrant_inmemory_roundtrip(chunks):
    from qdrant_client import QdrantClient
    from glassparse.vector import export_to_qdrant

    client = QdrantClient(":memory:")
    uploaded = export_to_qdrant(chunks, "pages", embedder_func=hash_embedder(32), client=client)
    assert uploaded == 2

    points = client.scroll(collection_name="pages", limit=10)[0]
    assert len(points) == 2
    id_set = {p.id for p in points}
    import uuid
    expect = {str(uuid.uuid5(uuid.NAMESPACE_URL, "chunk-0")),
              str(uuid.uuid5(uuid.NAMESPACE_URL, "chunk-1"))}
    assert id_set == expect
    # Metadata survived the trip.
    metas = {p.id: p.payload for p in points}
    uuid0 = str(uuid.uuid5(uuid.NAMESPACE_URL, "chunk-0"))
    uuid1 = str(uuid.uuid5(uuid.NAMESPACE_URL, "chunk-1"))
    assert metas[uuid0]["url"] == "https://x.test"
    assert metas[uuid1]["tokens"] == 12
    # Query with the same embedder returns the closest chunk first.
    hits = client.query_points(
        collection_name="pages",
        query=hash_embedder(32)("install pip"),
        limit=1,
    ).points
    assert hits and hits[0].payload["section"] == "Install"


def test_chroma_in_memory_roundtrip(chunks):
    from glassparse.vector import export_to_chroma

    added = export_to_chroma(chunks, "pages", embedder_func=hash_embedder(32), path=None)
    assert added == 2

    import chromadb
    client = chromadb.Client()
    col = client.get_collection("pages")
    got = col.get(ids=["chunk-0"])
    assert got["ids"] == ["chunk-0"]
    assert got["metadatas"][0]["section"] == "Intro"