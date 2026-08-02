"""Command-line interface: ``python -m glassparse``.

Thin wrapper around the SDK so a page can be parsed / crawled without writing a
script::

    python -m glassparse https://example.com --json out.json
    python -m glassparse https://example.com --markdown --out page.md
    python -m glassparse crawl https://site.example --max-pages 20 --jsonl
"""

from __future__ import annotations

import argparse
import asyncio
import json
import sys
from pathlib import Path

from . import Client, ParseResult
from .models import __version__


def _parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(prog="glassparse", description="GlassParse AI SDK CLI")
    p.add_argument("--version", action="version", version=f"glassparse {__version__}")
    p.add_argument("--url", default=None, help="target URL to parse")
    p.add_argument("--base-url", default="http://127.0.0.1:8080/api/parse",
                   help="engine HTTP endpoint")
    p.add_argument("--timeout", type=float, default=60.0, help="request timeout (s)")
    p.add_argument("--markdown", action="store_true", help="print clean Markdown")
    p.add_argument("--pretty", action="store_true", help="pretty-print JSON output")
    p.add_argument("--out", default=None, help="write output to a file (json)")

    sub = p.add_subparsers(dest="command")
    crawl = sub.add_parser("crawl", help="crawl a site (needs running server)")
    crawl.add_argument("url")
    crawl.add_argument("--max-pages", type=int, default=10)
    crawl.add_argument("--jsonl", action="store_true", help="JSONL of pages to --out")
    crawl.add_argument("--user-agent", default=None)
    return p.parse_args(argv)


def _emit(args: argparse.Namespace, result: ParseResult) -> None:
    if args.markdown or (args.out and args.out.endswith((".md", ".markdown"))):
        text = result.markdown
        if args.out:
            Path(args.out).write_text(text, encoding="utf-8")
        else:
            print(text)
        return
    payload = result.to_dict()
    dumped = json.dumps(payload, indent=2, ensure_ascii=False) if args.pretty else json.dumps(payload)
    if args.out:
        out = args.out if args.out.endswith(".json") else args.out + ".json"
        Path(out).write_text(dumped, encoding="utf-8")
    print(dumped)


async def _run(argv: list[str]) -> int:
    args = _parse_args(argv)
    async with Client(base_url=args.base_url, timeout=args.timeout) as client:
        if args.command == "crawl":
            pages = await client.crawl_site(
                args.url,
                max_pages=args.max_pages,
                user_agent=args.user_agent,
            )
            payload = {"pages": pages, "count": len(pages)}
            dumped = json.dumps(payload, indent=2)
            if args.out:
                Path(args.out).write_text(dumped, encoding="utf-8")
            else:
                print(dumped)
            return 0
        if not args.url:
            print("glassparse: provide --url (or a parse subcommand)", file=sys.stderr)
            return 2
        result = await client.parse(args.url, as_chunks=True)
        _emit(args, result)
        return 0


def main(argv: list[str] | None = None) -> int:  # noqa: D401
    return asyncio.run(_run(argv if argv is not None else sys.argv[1:]))


if __name__ == "__main__":
    raise SystemExit(main())