#!/usr/bin/env python3
"""Workspace knowledge catalog sync — the only supported remote order.

1. GET /api/v1/knowledge-catalog/knowledge-bases
2. For each kb_id: page GET /api/v1/knowledge-catalog/knowledge until has_more=false
3. Download GET /api/v1/knowledge/{id}/download only when
   can_download=true AND knowledge_type=file AND has_file=true

Catalog JSON must never contain file_path or vector_store_id.

  set WEKNORA_HOST=https://127.0.0.1:8080
  set WEKNORA_API_KEY=sk-...
  python scripts/2026-09-01-knowledge-catalog-sync.py --insecure --out ./tmp/catalog-files
"""

from __future__ import annotations

import argparse
import json
import os
import ssl
import sys
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import HTTPSHandler, Request, build_opener, install_opener, urlopen

FORBIDDEN_JSON_KEYS = ("file_path", "vector_store_id")
DEFAULT_HOST = "https://127.0.0.1:8080"
DEFAULT_LIMIT = 100


class CatalogError(RuntimeError):
    pass


def enable_insecure_tls() -> None:
    ctx = ssl._create_unverified_context()
    install_opener(build_opener(HTTPSHandler(context=ctx)))


def assert_no_forbidden_keys(payload: Any, where: str) -> None:
    if isinstance(payload, dict):
        for key, value in payload.items():
            if key in FORBIDDEN_JSON_KEYS:
                raise CatalogError(f"{where}: forbidden field {key!r} in catalog JSON")
            assert_no_forbidden_keys(value, where)
        return
    if isinstance(payload, list):
        for item in payload:
            assert_no_forbidden_keys(item, where)


def api_get_json(base: str, path: str, headers: dict[str, str], query: dict[str, Any] | None = None) -> Any:
    url = base.rstrip("/") + path
    if query:
        filtered = {k: v for k, v in query.items() if v is not None and v != ""}
        if filtered:
            url += "?" + urlencode(filtered)
    req = Request(url, headers=headers, method="GET")
    try:
        with urlopen(req, timeout=60) as resp:
            raw = resp.read()
            ctype = resp.headers.get("Content-Type", "")
    except HTTPError as err:
        body = err.read().decode("utf-8", errors="replace")
        raise CatalogError(f"GET {url} -> HTTP {err.code}: {body}") from err
    except URLError as err:
        raise CatalogError(f"GET {url} failed: {err}") from err
    if "json" not in ctype and not raw.startswith(b"{") and not raw.startswith(b"["):
        raise CatalogError(f"GET {url} expected JSON, got {ctype!r}")
    payload = json.loads(raw.decode("utf-8"))
    assert_no_forbidden_keys(payload, url)
    return payload


def api_download(base: str, knowledge_id: str, headers: dict[str, str], dest_dir: Path) -> Path:
    url = base.rstrip("/") + f"/api/v1/knowledge/{knowledge_id}/download"
    req = Request(url, headers=headers, method="GET")
    try:
        with urlopen(req, timeout=120) as resp:
            filename = filename_from_disposition(resp.headers.get("Content-Disposition", ""), knowledge_id)
            dest = unique_path(dest_dir / safe_filename(filename))
            dest.write_bytes(resp.read())
            return dest
    except HTTPError as err:
        body = err.read().decode("utf-8", errors="replace")
        raise CatalogError(f"GET {url} -> HTTP {err.code}: {body}") from err


def filename_from_disposition(header: str, fallback: str) -> str:
    if not header:
        return fallback
    for part in header.split(";"):
        part = part.strip()
        if part.lower().startswith("filename*="):
            value = part.split("=", 1)[1].strip().strip('"')
            if "''" in value:
                value = value.split("''", 1)[1]
            return value
        if part.lower().startswith("filename="):
            return part.split("=", 1)[1].strip().strip('"')
    return fallback


def safe_filename(name: str) -> str:
    cleaned = "".join(ch if ch not in '<>:"/\\|?*' else "_" for ch in name).strip()
    return cleaned or "download.bin"


def unique_path(path: Path) -> Path:
    if not path.exists():
        return path
    stem, suffix, parent = path.stem, path.suffix, path.parent
    n = 2
    while True:
        candidate = parent / f"{stem}-{n}{suffix}"
        if not candidate.exists():
            return candidate
        n += 1


def unwrap_data(payload: Any) -> Any:
    if isinstance(payload, dict) and "data" in payload:
        if payload.get("success") is False:
            raise CatalogError(f"API error: {payload.get('error')}")
        return payload["data"]
    return payload


def should_download(kb: dict[str, Any], item: dict[str, Any]) -> tuple[bool, str]:
    if not kb.get("can_download"):
        return False, "kb can_download=false"
    if item.get("knowledge_type") != "file":
        return False, f"knowledge_type={item.get('knowledge_type')!r}"
    if item.get("has_file") is False:
        return False, "has_file=false"
    return True, "ok"


def sync_catalog(
    base: str,
    headers: dict[str, str],
    out_dir: Path | None,
    *,
    include_org_shared: bool = True,
    kb_type: str = "",
    limit: int = DEFAULT_LIMIT,
    download: bool = True,
    max_download: int | None = None,
) -> dict[str, Any]:
    catalog = unwrap_data(
        api_get_json(
            base,
            "/api/v1/knowledge-catalog/knowledge-bases",
            headers,
            {
                "include_org_shared": "true" if include_org_shared else "false",
                "type": kb_type,
            },
        )
    )
    kbs = catalog.get("knowledge_bases") or []
    summary: dict[str, Any] = {
        "tenant_id": catalog.get("tenant_id"),
        "knowledge_bases": len(kbs),
        "truncated": bool(catalog.get("truncated")),
        "knowledge_items": 0,
        "downloaded": [],
        "skipped_download": [],
    }
    if out_dir is not None:
        out_dir.mkdir(parents=True, exist_ok=True)

    for kb in kbs:
        kb_id = kb["id"]
        cursor = ""
        while True:
            page = unwrap_data(
                api_get_json(
                    base,
                    "/api/v1/knowledge-catalog/knowledge",
                    headers,
                    {
                        "kb_id": kb_id,
                        "limit": str(limit),
                        "cursor": cursor,
                    },
                )
            )
            items = page.get("items") or []
            summary["knowledge_items"] += len(items)
            for item in items:
                ok, reason = should_download(kb, item)
                if not ok:
                    summary["skipped_download"].append(
                        {"kb_id": kb_id, "id": item.get("id"), "reason": reason}
                    )
                    continue
                if not download:
                    summary["skipped_download"].append(
                        {"kb_id": kb_id, "id": item.get("id"), "reason": "download disabled"}
                    )
                    continue
                if max_download is not None and len(summary["downloaded"]) >= max_download:
                    summary["skipped_download"].append(
                        {"kb_id": kb_id, "id": item.get("id"), "reason": "max_download reached"}
                    )
                    continue
                dest = api_download(base, item["id"], headers, out_dir or Path("."))
                summary["downloaded"].append(
                    {"kb_id": kb_id, "id": item["id"], "path": str(dest), "bytes": dest.stat().st_size}
                )
            if not page.get("has_more"):
                break
            cursor = page.get("next_cursor") or ""
            if not cursor:
                raise CatalogError(f"kb_id={kb_id} has_more=true but next_cursor is empty")
    return summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Sync workspace knowledge catalog then download owned files")
    parser.add_argument("--host", default=os.environ.get("WEKNORA_HOST", DEFAULT_HOST))
    parser.add_argument("--api-key", default=os.environ.get("WEKNORA_API_KEY", ""))
    parser.add_argument("--out", default="./tmp/catalog-files", help="Directory for downloaded files")
    parser.add_argument("--limit", type=int, default=DEFAULT_LIMIT)
    parser.add_argument("--type", default="", help="Optional KB type filter: document / faq / wiki")
    parser.add_argument("--include-org-shared", default="true", choices=("true", "false"))
    parser.add_argument("--skip-download", action="store_true")
    parser.add_argument("--max-download", type=int, default=None)
    parser.add_argument("--insecure", action="store_true", help="Skip TLS certificate verification (local https)")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.insecure:
            enable_insecure_tls()
        if not args.api_key:
            raise CatalogError("set WEKNORA_API_KEY or pass --api-key")
        summary = sync_catalog(
            args.host,
            {
                "X-API-Key": args.api_key,
                "Accept": "application/json",
            },
            Path(args.out),
            include_org_shared=args.include_org_shared == "true",
            kb_type=args.type,
            limit=args.limit,
            download=not args.skip_download,
            max_download=args.max_download,
        )
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return 0
    except CatalogError as err:
        print(f"error: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
