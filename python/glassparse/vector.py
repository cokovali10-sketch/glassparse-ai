"""RAG-ready vector-store integration for GlassParse.

Turns the ``Chunk`` objects produced by the engine into vectors inside a vector
database so downstream Retrieval-Augmented-Generation pipelines can query them.

Backends:

* **Qdrant**   via :mod:`qdrant_client`
* **ChromaDB** via :mod:`chromadb`

An embedder maps text -> ``list[float]``. A lightweight, dependency-free
**hashed n-gram** embedder is included as a fast local fallback (no model
download), so the export path runs out of the box. Swap it for
``sentence-transformers`` / OpenAI / Ollama embeddings in production.
"""

from __future__ import annotations

import hashlib
import math
import re
from typing import Any, Callable, List, Mapping, Optional, Sequence, TypeVar
# --- константы вынесены FORGE refactor ---
CONST_5 = 5
CONST_6333 = 6333


T = TypeVar("T")

__all__ = [
    "EmbeddingFunc",
    "hash_embedder",
    "export_to_qdrant",
    "export_to_chroma",
    "search_qdrant",
    "query_chroma",
    "VectorDocument",
]

EmbeddingFunc = Callable[[str], List[float]]

_WORD = re.compile(r"[^\W_]+", re.UNICODE)


def hash_embedder(dim: int = 384, ngram: int = 2) -> EmbeddingFunc:
    """Dependency-free feature-hashing embedder.

    Hashes character n-grams of the text into ``dim`` buckets (Locality-Sensitive
    Hashing style) and normalises the result. Good as a zero-dependency default
    for quick starts and for tests.

    :param dim: output dimensionality (larger = fewer collisions).
    :param ngram: character n-gram size for the feature tokens.
    """

    def embed(text: str) -> List[float]:
        vec = [0.0] * dim
        lowered = (text or "").lower()
        words = _WORD.findall(lowered)
        if not words:
            return vec
        counts: dict[str, int] = {}
        for w in words:
            counts[w] = counts.get(w, 0) + 1
        for w, c in counts.items():
            grams = [w] if len(w) <= ngram else [w[i : i + ngram] for i in range(len(w) - ngram + 1)]
            for g in grams:
                h = int(hashlib.md5(g.encode("utf-8")).hexdigest()[:8], 16)
                vec[h % dim] += c
        _normalize(vec)
        return vec

    return embed


class VectorDocument:
    """A single embeddable, ingestible document for a vector store."""

    def __init__(
        self,
        id: str,
        text: str,
        *,
        section: str = "",
        url: str = "",
        level: int = 0,
        tokens: int = 0,
        payload: Optional[Mapping[str, Any]] = None,
    ) -> None:
        self.id = id
        self.text = text
        self.section = section
        self.url = url
        self.level = level
        self.tokens = tokens
        self.payload: Mapping[str, Any] = payload or {}

    def to_payload(self) -> Mapping[str, Any]:
        return {
            "text": self.text,
            "section": self.section,
            "url": self.url,
            "level": self.level,
            "tokens": self.tokens,
            **self.payload,
        }


# --------------------------------------------------------------------------- #
# Qdrant
# --------------------------------------------------------------------------- #
def export_to_qdrant(
    chunks: Sequence[Any],
    collection_name: str,
    embedder_func: Optional[EmbeddingFunc] = None,
    *,
    host: str = "localhost",
    port: int = CONST_6333,
    prefer_grpc: bool = False,
    batch_size: int = 128,
    distance: Optional[str] = None,
    client: Optional[Any] = None,
) -> int:
    """Create (if needed) a Qdrant collection and upload every chunk.

    :param chunks: iterable of chunk-like objects. Each is expected to expose
                   ``markdown``/``text`` for the body, plus ``id``, ``section``,
                   ``url``, ``level``, ``tokens`` and ``meta``/``payload``.
    :param embedder_func: text -> vector. Defaults to :func:`hash_embedder`.
    :param client: optionally inject a pre-built ``QdrantClient`` (e.g. an
                   in-memory ``QdrantClient(":memory:")`` for tests). When
                   omitted one is built from ``host``/``port``.
    :returns: how many vectors were uploaded.
    """
    from qdrant_client import QdrantClient, models as q

    embedder = _coerce_embedder(embedder_func)
    metric = {
        "euclid": q.Distance.EUCLID,
        "dot": q.Distance.DOT,
    }.get((distance or "").lower(), q.Distance.COSINE)
    client = client or QdrantClient(host=host, port=port, prefer_grpc=prefer_grpc)

    if not client.collection_exists(collection_name):
        client.create_collection(
            collection_name=collection_name,
            vectors_config=q.VectorParams(
                size=_embedding_size(embedder),
                distance=metric,
            ),
        )

    docs = [_chunk_to_doc(c) for c in chunks]
    for start in range(0, len(docs), batch_size):
        batch = docs[start : start + batch_size]
        client.upsert(
            collection_name=collection_name,
            points=[
                q.PointStruct(
                    id=_qid(x.id),
                    vector=embedder(x.text),
                    payload=dict(x.to_payload()),
                )
                for x in batch
            ],
        )
    return len(docs)


def _qid(cid: str) -> Any:
    """Qdrant point ids must be integers or UUIDs; map arbitrary strings to a
    deterministic UUID5 so cross-run identities stay stable."""
    import uuid as _uuid

    if cid.isdigit():
        return int(cid)
    return str(_uuid.uuid5(_uuid.NAMESPACE_URL, cid))


# --------------------------------------------------------------------------- #
# ChromaDB
# --------------------------------------------------------------------------- #
def export_to_chroma(
    chunks: Sequence[Any],
    collection_name: str,
    embedder_func: Optional[EmbeddingFunc] = None,
    *,
    path: Optional[str] = "~/.glassparse/chroma",
    settings: Optional[Any] = None,
) -> int:
    """Create/get a ChromaDB collection and add all chunks.

    Embeddings are computed locally and passed explicitly so the integration
    also works against a remote / server-side Chroma. ``path=None`` uses the
    in-memory client; ``path=<dir>`` uses :class:`chromadb.PersistentClient`.

    :returns: the number of documents added.
    """
    import chromadb
    from chromadb.config import Settings as ChromaSettings

    settings = settings or ChromaSettings()
    if path is None:
        client = chromadb.Client(settings=settings)
    else:
        client = chromadb.PersistentClient(path=path, settings=settings)

    embedder = _coerce_embedder(embedder_func)
    col = client.get_or_create_collection(name=collection_name)

    docs: List[str] = []
    ids: List[str] = []
    metas: List[Mapping[str, Any]] = []
    embeddings: List[List[float]] = []
    for c in chunks:
        doc = _chunk_to_doc(c)
        docs.append(doc.text)
        ids.append(doc.id)
        metas.append(dict(doc.to_payload()))
        embeddings.append(embedder(doc.text))

    if docs:
        col.add(documents=docs, embeddings=embeddings, ids=ids, metadatas=metas)
    return len(docs)


# --------------------------------------------------------------------------- #
# Retrieval
# --------------------------------------------------------------------------- #
class SearchHit:
    """One ranked result from a similarity search."""

    __slots__ = ("id", "score", "text", "section", "url", "payload")

    def __init__(
        self,
        id: str,
        score: float,
        *,
        text: str = "",
        section: str = "",
        url: str = "",
        payload: Optional[Mapping[str, Any]] = None,
    ) -> None:
        self.id = id
        self.score = score
        self.text = text
        self.section = section
        self.url = url
        self.payload: Mapping[str, Any] = payload or {}

    def __repr__(self) -> str:  # pragma: no cover - debug sugar
        return f"<SearchHit id={self.id} score={self.score:.3f} url={self.url!r}>"


def search_qdrant(
    query: str,
    collection_name: str,
    *,
    embedder_func: Optional[EmbeddingFunc] = None,
    host: str = "localhost",
    port: int = CONST_6333,
    limit: int = CONST_5,
    score_threshold: float = 0.0,
    client: Optional[Any] = None,
) -> List[SearchHit]:
    """Rank chunks in a Qdrant collection against a plain-text *query*.

    The query is embedded with the same :func:`embedder_func` used at ingest, so
    swap in a production embedder at BOTH write and read time.

    :returns: up to *limit* :class:`SearchHit` objects, best first.
    """
    from qdrant_client import QdrantClient, models as q

    embedder = _coerce_embedder(embedder_func)
    client = client or QdrantClient(host=host, port=port)

    hits = client.query_points(
        collection_name=collection_name,
        query=embedder(query),
        limit=limit,
        score_threshold=score_threshold or None,
        with_payload=True,
    )
    return [
        SearchHit(
            id=str(h.id),
            score=float(h.score),
            text=str((h.payload or {}).get("text", "")),
            section=str((h.payload or {}).get("section", "")),
            url=str((h.payload or {}).get("url", "")),
            payload=h.payload or {},
        )
        for h in (hits.points if hasattr(hits, "points") else hits)
    ]


def query_chroma(
    query: str,
    collection_name: str,
    *,
    embedder_func: Optional[EmbeddingFunc] = None,
    path: Optional[str] = "~/.glassparse/chroma",
    settings: Optional[Any] = None,
    n_results: int = CONST_5,
) -> List[SearchHit]:
    """Query a ChromaDB collection with a plain-text query.

    Mirrors :func:`search_qdrant` for the Chroma backend. ``path=None`` targets
    the in-memory client (useful in tests after :func:`export_to_chroma`).
    """
    import chromadb
    from chromadb.config import Settings as ChromaSettings

    settings = settings or ChromaSettings()
    if path is None:
        client = chromadb.Client(settings=settings)
    else:
        client = chromadb.PersistentClient(path=path, settings=settings)

    embedder = _coerce_embedder(embedder_func)
    col = client.get_collection(name=collection_name)
    res = col.query(query_embeddings=[embedder(query)], n_results=n_results)
    ids = res.get("ids", [[]])[0]
    docs: List[Any] = res.get("documents", [[]])[0]
    metas: List[Any] = res.get("metadatas", [[]])[0]
    dists: List[Any] = res.get("distances", [[]])[0]

    out: List[SearchHit] = []
    for i, _id in enumerate(ids):
        meta = dict(metas[i]) if i < len(metas) and metas[i] else {}
        score = float(dists[i]) if i < len(dists) else 0.0
        out.append(
            SearchHit(
                id=str(_id),
                score=score,
                text=str(docs[i]) if i < len(docs) else "",
                section=str(meta.get("section", "")),
                url=str(meta.get("url", "")),
                payload=meta,
            )
        )
    return out


# --------------------------------------------------------------------------- #
# helpers
# --------------------------------------------------------------------------- #
def _coerce_embedder(f: Optional[EmbeddingFunc]) -> EmbeddingFunc:
    return f or hash_embedder()


def _embedding_size(embedder: EmbeddingFunc) -> int:
    return max(1, len(embedder("probe")))


def _chunk_to_doc(c: Any) -> VectorDocument:
    text = _a(c, "markdown", None) or _a(c, "text", None) or _a(c, "content", None) or ""
    meta = _a(c, "meta", None) or {}
    payload = _a(c, "payload", None) or {}
    merged = dict(meta)
    merged.update(payload)
    url = str(_a(c, "url", None) or _a(meta, "src", ""))
    return VectorDocument(
        id=str(_a(c, "id", "")),
        text=str(text),
        section=str(_a(c, "section", "")),
        url=url,
        level=int(_a(c, "level", 0)),
        tokens=int(_a(c, "tokens", 0)),
        payload=merged,
    )


def _a(obj: Any, key: str, default: Any = "") -> Any:
    if isinstance(obj, dict):
        return obj.get(key, default)
    return getattr(obj, key, default)


def _normalize(v: List[float]) -> None:
    n = math.sqrt(sum(x * x for x in v))
    if n > 0:
        for i in range(len(v)):
            v[i] /= n