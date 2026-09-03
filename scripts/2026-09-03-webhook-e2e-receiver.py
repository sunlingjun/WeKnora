#!/usr/bin/env python3
"""Local HMAC receiver for WeKnora workspace webhook E2E."""
from __future__ import annotations

from http.server import BaseHTTPRequestHandler, HTTPServer
import hashlib
import hmac
import json
import os
import time
from pathlib import Path

SECRET = os.environ.get("WEKNORA_WEBHOOK_SECRET", "whsec_contrib_e2e_secret_32ch").encode()
PORT = int(os.environ.get("WEBHOOK_E2E_PORT", "18081"))
LOG = Path(os.environ.get("WEBHOOK_E2E_LOG", "tmp-webhook-e2e.jsonl"))
WINDOW = 300


class Handler(BaseHTTPRequestHandler):
    def do_POST(self) -> None:
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        ts = self.headers.get("X-WeKnora-Timestamp", "")
        sig = self.headers.get("X-WeKnora-Signature", "")
        event = self.headers.get("X-WeKnora-Event", "")
        delivery = self.headers.get("X-WeKnora-Delivery", "")
        ok = False
        reason = ""
        try:
            unix = int(ts)
            if abs(time.time() - unix) > WINDOW:
                reason = "stale"
            else:
                expected = "sha256=" + hmac.new(
                    SECRET, f"{unix}.".encode() + raw, hashlib.sha256
                ).hexdigest()
                if len(sig) == len(expected) and hmac.compare_digest(sig, expected):
                    ok = True
                else:
                    reason = "bad_sig"
        except Exception as exc:  # noqa: BLE001
            reason = f"err:{exc}"

        row = {
            "ok": ok,
            "reason": reason,
            "event": event,
            "delivery": delivery,
            "bytes": len(raw),
            "ts": ts,
        }
        try:
            row["body_type"] = json.loads(raw.decode("utf-8")).get("type")
        except Exception:
            row["body_type"] = None
        with LOG.open("a", encoding="utf-8") as f:
            f.write(json.dumps(row, ensure_ascii=False) + "\n")
        if not ok:
            self.send_error(401, reason or "unauthorized")
            return
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, fmt: str, *args) -> None:
        return


if __name__ == "__main__":
    LOG.write_text("", encoding="utf-8")
    print(f"listening http://127.0.0.1:{PORT}/hooks/weknora log={LOG}")
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
