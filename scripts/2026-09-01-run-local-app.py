#!/usr/bin/env python3
"""Load repo .env and start the WeKnora API from source (Windows-friendly)."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load_env(path: Path) -> None:
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key:
            os.environ[key] = value


def main() -> int:
    env_file = ROOT / ".env"
    if not env_file.exists():
        print(f"missing {env_file}", file=sys.stderr)
        return 1
    load_env(env_file)
    os.environ.setdefault("DOCREADER_TRANSPORT", "grpc")
    storage = os.environ.get("LOCAL_STORAGE_BASE_DIR", "")
    if not storage or storage == "/data/files":
        storage = str(ROOT / ".local-data" / "files")
        os.environ["LOCAL_STORAGE_BASE_DIR"] = storage
    Path(storage).mkdir(parents=True, exist_ok=True)
    print(f"starting go run ./cmd/server  DB={os.environ.get('DB_HOST')}:{os.environ.get('DB_PORT')} STORAGE={storage}")
    return subprocess.call(["go", "run", "./cmd/server"], cwd=ROOT)


if __name__ == "__main__":
    raise SystemExit(main())
