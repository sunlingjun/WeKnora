from pathlib import Path
import re

zh = Path(r"E:/Tencent/NXIN-WEKNORA/frontend/src/i18n/locales/zh-CN.ts").read_text(encoding="utf-8")
checks = [
    "欢迎使用ZSK",
    "Hi，我是 ZSK",
    "首次使用 ZSK",
    "欢迎使用WeKnora",
    "WeKnoraCloud",
    "农信知识库",
]
print("zh-CN checks:")
for s in checks:
    print(f"  {s!r}: {zh.count(s)}")

env = Path(r"E:/Tencent/NXIN-WEKNORA/frontend/.env.development").read_bytes()
print("env.development BOM/start:", env[:80])
print("env has utf8 农信:", "农信知识库".encode("utf-8") in env)

print("\nleftover prompt brand:")
root = Path(r"E:/Tencent/NXIN-WEKNORA/config/prompt_templates")
for f in sorted(root.glob("*.yaml")):
    txt = f.read_text(encoding="utf-8")
    for pat in ["You are WeKnora", "developed by Tencent"]:
        if pat in txt:
            print(f"  {f.name}: still has {pat} x{txt.count(pat)}")

print("\nZSK/Nxin counts in prompts:")
for f in sorted(root.glob("*.yaml")):
    txt = f.read_text(encoding="utf-8")
    z = txt.count("You are ZSK")
    n = txt.count("developed by Nxin")
    if z or n:
        print(f"  {f.name}: ZSK={z} Nxin={n}")
