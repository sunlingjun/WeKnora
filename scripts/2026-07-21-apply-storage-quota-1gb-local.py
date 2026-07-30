# -*- coding: utf-8 -*-
from pathlib import Path

for root in [r"E:/Tencent/NXIN-WEKNORA", r"E:/Tencent/WeKnora-slj"]:
    p = Path(root) / "frontend/src/i18n/locales/ru-RU.ts"
    t = p.read_text(encoding="utf-8")
    t2 = t.replace(
        "встроенное значение по умолчанию 10 ГБ",
        "встроенное значение по умолчанию 1 ГБ",
    )
    p.write_text(t2, encoding="utf-8", newline="\n")
    print(p, "changed" if t != t2 else "skip")

env = {}
for line in Path(r"E:/Tencent/NXIN-WEKNORA/.env").read_text(encoding="utf-8", errors="ignore").splitlines():
    line = line.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    k, v = line.split("=", 1)
    env[k.strip()] = v.strip().strip('"').strip("'")

try:
    import psycopg2
except ImportError:
    import subprocess, sys
    subprocess.check_call([sys.executable, "-m", "pip", "install", "psycopg2-binary", "-q"])
    import psycopg2

conn = psycopg2.connect(
    host=env.get("DB_HOST", "localhost"),
    port=env.get("DB_PORT", "5432"),
    user=env.get("DB_USER", "postgres"),
    password=env.get("DB_PASSWORD", ""),
    dbname=env.get("DB_NAME", "WeKnora"),
)
cur = conn.cursor()
cur.execute("SELECT key, value FROM system_settings WHERE key=%s", ("tenant.default_storage_quota_gb",))
rows = cur.fetchall()
print("before", rows)
if rows:
    cur.execute(
        "UPDATE system_settings SET value=%s, updated_at=NOW() WHERE key=%s",
        ("1", "tenant.default_storage_quota_gb"),
    )
    conn.commit()
    cur.execute("SELECT key, value FROM system_settings WHERE key=%s", ("tenant.default_storage_quota_gb",))
    print("after", cur.fetchall())
else:
    print("no DB override; code Default=1 applies after restart")
cur.close()
conn.close()
