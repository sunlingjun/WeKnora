#!/usr/bin/env python3
"""Load repo .env and start cmd/server against local docker-compose.dev infra."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load_env(path: Path) -> None:
    for line in path.read_text(encoding="utf-8").splitlines():
        text = line.strip()
        if not text or text.startswith("#") or "=" not in text:
            continue
        key, value = text.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key:
            os.environ[key] = value


def main() -> int:
    env_file = ROOT / ".env"
    if not env_file.exists():
        print("missing .env", file=sys.stderr)
        return 1
    load_env(env_file)
    os.environ["DB_HOST"] = "localhost"
    os.environ["REDIS_ADDR"] = "localhost:6379"
    os.environ["DOCREADER_ADDR"] = "localhost:50051"
    os.environ["MINIO_ENDPOINT"] = "localhost:9000"
    os.environ["NEO4J_URI"] = "bolt://localhost:7687"
    os.environ.setdefault("DOCREADER_TRANSPORT", "grpc")
    storage = os.environ.get("LOCAL_STORAGE_BASE_DIR", "")
    if not storage or storage == "/data/files":
        storage = str(ROOT / ".local-data" / "files")
        os.environ["LOCAL_STORAGE_BASE_DIR"] = storage
    Path(storage).mkdir(parents=True, exist_ok=True)
    print("starting server DB_HOST=%s PORT=%s STORAGE=%s" % (
        os.environ.get("DB_HOST"), os.environ.get("APP_PORT"), storage,
    ))
    return subprocess.call(["go", "run", "./cmd/server"], cwd=str(ROOT))


if __name__ == "__main__":
    raise SystemExit(main())
