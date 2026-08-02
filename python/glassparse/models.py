"""Pydantic models mirroring the Go engine's wire format.

Keeping these in sync with ``engine/pkg/model`` guarantees that output captured
from the engine (or produced by the bundled Go binary) deserialises losslessly
in the SDK.
"""

from __future__ import annotations

import time
from datetime import datetime, timezone
from enum import Enum
from pathlib import Path
from typing import Dict, List, Optional

from pydantic import BaseModel, Field

__version__ = "0.1.0"


class RenderMode(str, Enum):
    """How a page was fetched / rendered by the engine."""

    HTTP = "http"
    HEADLESS = "headless"


class FetchOptions(BaseModel):
    """Per-request tunables passed to the engine (anti-bot + headless control)."""

    url: str
    user_agent: Optional[str] = None
    proxy: Optional[str] = None
    rotate_ua: bool = False
    max_redirect: int = 10
    timeout_seconds: float = 30.0
    headless: bool = False

    def to_query(self) -> Dict[str, str]:
        """Serialize to URL query parameters for an ``/parse`` style endpoint."""
        params: Dict[str, str] = {"url": self.url}
        if self.user_agent:
            params["user_agent"] = self.user_agent
        if self.proxy:
            params["proxy"] = self.proxy
        params["rotate_ua"] = "1" if self.rotate_ua else "0"
        params["headless"] = "1" if self.headless else "0"
        return params


class Metadata(BaseModel):
    """Page-level facts extracted from <head>/<meta>."""

    title: str = ""
    description: str = ""
    lang: str = ""
    canonical: str = ""
    site_name: str = ""
    image: str = ""
    author: str = ""
    keywords: Optional[List[str]] = None
    og: Optional[Dict[str, str]] = None

    @property
    def keyword_list(self) -> List[str]:
        return self.keywords or []


class Stats(BaseModel):
    """Cleaning telemetry used for dashboard charts."""

    input_bytes: int = 0
    output_bytes: int = 0
    compression_ratio: float = 0.0
    removed_nodes: int = 0
    headings: int = 0
    links: int = 0
    tables: int = 0
    images: int = 0
    nodes_before: int = 0
    nodes_after: int = 0


class Page(BaseModel):
    """Result of the cleaner stage."""

    url: str = ""
    status_code: int = 0
    mode: RenderMode = RenderMode.HTTP
    duration_ns: int = 0
    fetched_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    metadata: Metadata = Field(default_factory=Metadata)
    markdown: str = ""
    clean_html: str = ""
    stats: Stats = Field(default_factory=Stats)


class Entity(BaseModel):
    """One structured datum extracted from the DOM (price, title, ...)."""

    type: str
    label: str = ""
    value: str = ""
    confidence: float = 0.0
    selector: str = ""
    children: List["Entity"] = Field(default_factory=list)


class Chunk(BaseModel):
    """One RAG-ready chunk emitted by the engine's chunker."""

    id: str = ""
    section: str = ""
    level: int = 0
    markdown: str = ""
    tokens: int = 0
    meta: Dict[str, str] = Field(default_factory=dict)


class ParseResult(BaseModel):
    """The complete deliverable of a parse call."""

    page: Page
    entities: List[Entity] = Field(default_factory=list)
    chunks: List[Chunk] = Field(default_factory=list)

    # -- small convenience helpers -----------------------------------------
    @property
    def markdown(self) -> str:
        return self.page.markdown

    def entities_of_type(self, entity_type: str) -> List[Entity]:
        return [e for e in self.entities if e.type == entity_type]

    def prices(self) -> List[str]:
        return [e.value for e in self.entities if e.type == "price"]

    def emails(self) -> List[str]:
        return [e.value for e in self.entities if e.type == "email"]

    def elapsed_seconds(self) -> float:
        return self.page.duration_ns / 1e9 if self.page.duration_ns else 0.0

    # -- export / serialization --------------------------------------------
    def to_dict(self) -> dict:
        """Return a plain, JSON-serialisable dict of the whole result."""
        return self.model_dump(mode="json")

    def to_json(self, path: Optional[str] = None, *, indent: int = 2) -> str:
        """Serialise to JSON; if *path* is given, also write it to disk."""
        text = self.model_dump_json(indent=indent)
        if path:
            Path(path).write_text(text, encoding="utf-8")
        return text

    def to_jsonl(self, path: Optional[str] = None) -> List[str]:
        """Serialise each chunk as one JSON line (Qdrant/Chroma/LlamaIndex).

        Returns the list of lines; writes them to *path* when given.
        """
        lines: List[str] = [c.model_dump_json() for c in (self.chunks or [])]
        if path:
            Path(path).write_text("\n".join(lines) + "\n", encoding="utf-8")
        return lines

    # -- content helpers ----------------------------------------------------
    def words(self) -> List[str]:
        """Tokenised plain-text words of the cleaned page."""
        return self.markdown.split()

    def titles(self) -> List[str]:
        """Distinct values of ``title``-typed entities (headings)."""
        return list({e.value for e in self.entities if e.type in ("title", "article")})


# Keep the engine binary in a stable namespace for the convenience entrypoints.
_engine_bin = "glassparse-engine"
_last_fetch_ns: int = 0


def stamp_fetch() -> None:
    """Record current time for duration telemetry (nanoseconds)."""
    global _last_fetch_ns
    _last_fetch_ns = int(time.time_ns())