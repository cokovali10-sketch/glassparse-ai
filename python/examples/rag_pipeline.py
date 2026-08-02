"""RAG pipeline demo: Parse a site via the GlassParse Go engine, chunk it, and
load those chunks into Qdrant (or Chroma) as searchable vectors.

Prerequisites
-------------
1. Python SDK installed:        pip install -e "python[dev]"
2. Optional vector libs:        pip install qdrant-client chromadb
3. The engine server running:   docker compose up -d      (or run the Go binary)
4. For a quick test WITHOUT Qdrant running, set CLOUD=0 and EXPORT=plains.

Run:
    python examples/rag_pipeline.py --url https://example.com \\
        --collection examples --store qdrant
"""

from __future__ import annotations

import argparse
import asyncio

from glassparse import Client, hash_embedder


async def main() -> None:
    parser = argparse.ArgumentParser(description="GlassParse -> RAG pipeline")
    parser.add_argument("--url", default="https://example.com", help="site to parse")
    parser.add_argument("--engine", default="http://127.0.0.1:8080/api/parse",
                        help="GlassParse engine endpoint")
    parser.add_argument("--collection", default="glassparse-web", help="vector collection")
    parser.add_argument("--store", choices=["qdrant", "chroma"], default="qdrant")
    parser.add_argument("--qdrant-host", default="localhost")
    parser.add_argument("--qdrant-port", type=int, default=6333)
    args = parser.parse_args()

    # 1) Parse through the engine.
    async with Client(base_url=args.engine) as client:
        result = await client.parse(args.url, as_chunks=True)

    print(f"page title    : {result.page.metadata.title}")
    print(f"entities      : {len(result.entities)} "
          f"(prices={len(result.prices())}, emails={len(result.emails())})")
    print(f"DOM reduction : {result.page.stats.nodes_before} -> "
          f"{result.page.stats.nodes_after} (ratio {result.page.stats.compression_ratio:.2f})")

    # 2) Embed + export. Default embedder = local hashed n-gram (no downloads);
    #    pass `embedder_func=your_fun` to use sentence-transformers / OpenAI.
    embedder = hash_embedder(dim=384)
    if args.store == "qdrant":
        from glassparse import export_to_qdrant
        count = export_to_qdrant(
            result.chunks,
            args.collection,
            embedder_func=embedder,
            host=args.qdrant_host,
            port=args.qdrant_port,
        )
    else:
        from glassparse import export_to_chroma
        count = export_to_chroma(result.chunks, args.collection, embedder_func=embedder)

    print(f"uploaded {count} chunks to {args.store!r} (collection {args.collection!r})")


if __name__ == "__main__":
    asyncio.run(main())