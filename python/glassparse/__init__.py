"""GlassParse AI Python SDK.

A typed, asyncio-first client that lets an AI engineer drive the GlassParse AI
Go engine in two lines of code::

    import asyncio
    from glassparse import GlassParse

    result = asyncio.run(GlassParse.parse("https://example.com", json=True))

The module re-exports the public API and version so callers can ``from
glassparse import Client, ParseResult``.
"""

from .client import Client
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
)
from .vector import (
    VectorDocument,
    hash_embedder,
    export_to_chroma,
    export_to_qdrant,
    search_qdrant,
    query_chroma,
)

__all__ = [
    "Client",
    "Chunk",
    "Entity",
    "FetchOptions",
    "Metadata",
    "Page",
    "ParseResult",
    "RenderMode",
    "Stats",
    "VectorDocument",
    "hash_embedder",
    "export_to_qdrant",
    "export_to_chroma",
    "search_qdrant",
    "query_chroma",
    "__version__",
]