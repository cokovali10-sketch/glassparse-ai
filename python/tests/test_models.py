"""Model round-trip tests: mirrors the Go engine's JSON so the SDK's types stay
contract-safe with the engine binary."""

from glassparse.models import Entity, Page, ParseResult, RenderMode


def test_parse_result_roundtrip():
    payload = {
        "page": {
            "url": "https://example.com",
            "status_code": 200,
            "mode": "http",
            "duration_ns": 123,
            "metadata": {"title": "Example", "description": ""},
            "markdown": "# Hi",
            "clean_html": "",
            "stats": {"input_bytes": 10, "output_bytes": 2, "compression_ratio": 0.2},
        },
        "entities": [
            {"type": "price", "value": "$99", "confidence": 0.9},
            {"type": "email", "value": "a@b.co", "confidence": 0.95},
        ],
        "chunks": [],
    }
    result = ParseResult.model_validate(payload)
    assert result.markdown == "# Hi"
    assert result.prices() == ["$99"]
    assert result.emails() == ["a@b.co"]
    assert result.page.mode is RenderMode.HTTP
    assert result.page.stats.compression_ratio == 0.2


def test_entity_children():
    row = Entity(type="row", value="A | B")
    table = Entity(type="table", value="sum", children=[row])
    assert table.children[0].value == "A | B"


def test_fetch_options_query():
    from glassparse.models import FetchOptions

    opts = FetchOptions(url="https://x.y", proxy="http://p:3128", headless=True)
    q = opts.to_query()
    assert q["headless"] == "1"
    assert q["proxy"] == "http://p:3128"
    assert q["url"] == "https://x.y"