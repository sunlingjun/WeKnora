from pathlib import Path

pairs = [
    ("firstTime: 'New to WeKnora?'", "firstTime: 'New to ZSK?'"),
    ("registerSubtitle: 'Create your account and start using WeKnora'",
     "registerSubtitle: 'Create your account and start using ZSK'"),
    ("New to WeKnora?", "New to ZSK?"),
    ("start using WeKnora", "start using ZSK"),
    ("I am WeKnora", "I am ZSK"),
    ("I'm WeKnora", "I'm ZSK"),
    ('firstTime: "首次使用 WeKnora？"', 'firstTime: "首次使用 ZSK？"'),
]

for root in [Path(r"E:/Tencent/NXIN-WEKNORA"), Path(r"E:/Tencent/WeKnora-slj")]:
    for loc in ["en-US.ts", "ko-KR.ts", "ru-RU.ts", "zh-CN.ts"]:
        p = root / "frontend/src/i18n/locales" / loc
        if not p.exists():
            continue
        t = p.read_text(encoding="utf-8")
        orig = t
        for a, b in pairs:
            t = t.replace(a, b)
        if t != orig:
            p.write_text(t, encoding="utf-8", newline="\n")
            print("updated", p)
        else:
            print("nochange", root.name, loc)
