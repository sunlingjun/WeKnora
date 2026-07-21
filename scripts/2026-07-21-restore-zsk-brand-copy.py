#!/usr/bin/env python3
"""Restore NXIN brand copy (ZSK / Nxin) lost during WeKnora-slj upgrades.

Safe rules:
- Replace user-facing WeKnora / Tencent brand phrases.
- Do NOT touch WeKnoraCloud, WEKNORA_*, aud=weknora, X-WeKnora-*, clawhub weknora ids.
"""

from __future__ import annotations

import re
from pathlib import Path

ROOTS = [
    Path(r"E:/Tencent/WeKnora-slj"),
    Path(r"E:/Tencent/NXIN-WEKNORA"),
]

# Order matters: longer / more specific first.
PROMPT_REPLACEMENTS = [
    ("You are WeKnora Wiki Researcher, an intelligent retrieval assistant developed by Tencent.",
     "You are ZSK Wiki Researcher, an intelligent retrieval assistant developed by Nxin."),
    ("You are the WeKnora Wiki Fixer, a specialized AI agent responsible for",
     "You are the ZSK Wiki Fixer, a specialized AI agent responsible for"),
    ("You are WeKnora Hybrid Researcher, an intelligent retrieval assistant developed by Tencent.",
     "You are ZSK Hybrid Researcher, an intelligent retrieval assistant developed by Nxin."),
    ("You are WeKnora Data Analyst, an intelligent data analysis assistant developed by Tencent,",
     "You are ZSK Data Analyst, an intelligent data analysis assistant developed by Nxin,"),
    ("You are WeKnora, an intelligent retrieval assistant developed by Tencent,",
     "You are ZSK, an intelligent retrieval assistant developed by Nxin,"),
    ("You are WeKnora, an intelligent assistant developed by Tencent,",
     "You are ZSK, an intelligent assistant developed by Nxin,"),
    ("You are WeKnora, a professional intelligent information retrieval assistant developed by Tencent.",
     "You are ZSK, a professional intelligent information retrieval assistant developed by Nxin."),
    ("You are WeKnora, a senior domain expert assistant developed by Tencent,",
     "You are ZSK, a senior domain expert assistant developed by Nxin,"),
    ("You are WeKnora, a professional and friendly customer service assistant developed by Tencent,",
     "You are ZSK, a professional and friendly customer service assistant developed by Nxin,"),
    ("You are WeKnora, a professional technical support engineer developed by Tencent,",
     "You are ZSK, a professional technical support engineer developed by Nxin,"),
    ("You are WeKnora, an intelligent conversational assistant developed by Tencent,",
     "You are ZSK, an intelligent conversational assistant developed by Nxin,"),
    ("You are WeKnora, an intelligent assistant developed by Tencent with web search capabilities,",
     "You are ZSK, an intelligent assistant developed by Nxin with web search capabilities,"),
    ("You are WeKnora, a professional and friendly AI assistant developed by Tencent.",
     "You are ZSK, a professional and friendly AI assistant developed by Nxin."),
    ("You are WeKnora, a professional and friendly intelligent assistant developed by Tencent.",
     "You are ZSK, a professional and friendly intelligent assistant developed by Nxin."),
    ("You are WeKnora, a professional and friendly intelligent assistant developed by Tencent,",
     "You are ZSK, a professional and friendly intelligent assistant developed by Nxin,"),
    ("You are WeKnora, a professional intelligent assistant developed by Tencent.",
     "You are ZSK, a professional intelligent assistant developed by Nxin."),
    ("You are WeKnora, a professional intelligent assistant developed by Tencent,",
     "You are ZSK, a professional intelligent assistant developed by Nxin,"),
    # Generic leftovers inside prompt templates only
    ("developed by Tencent", "developed by Nxin"),
    ("You are WeKnora,", "You are ZSK,"),
    ("You are WeKnora ", "You are ZSK "),
    ("the WeKnora Wiki Fixer", "the ZSK Wiki Fixer"),
]

I18N_REPLACEMENTS = [
    ('welcome: "欢迎使用WeKnora"', 'welcome: "欢迎使用ZSK"'),
    ("welcome: '欢迎使用WeKnora'", "welcome: '欢迎使用ZSK'"),
    ('welcome: "Welcome to WeKnora"', 'welcome: "Welcome to ZSK"'),
    ("welcome: 'Welcome to WeKnora'", "welcome: 'Welcome to ZSK'"),
    ('title: "欢迎使用 WeKnora"', 'title: "欢迎使用 ZSK"'),
    ("title: 'Welcome to WeKnora'", "title: 'Welcome to ZSK'"),
    ('title: "Hi，我是 WeKnora，让你的知识触手可及"', 'title: "Hi，我是 ZSK，让你的知识触手可及"'),
    ("title: \"Hi, I'm WeKnora", "title: \"Hi, I'm ZSK"),
    ('firstTime: "首次使用 WeKnora？"', 'firstTime: "首次使用 ZSK？"'),
    ("firstTime: 'First time using WeKnora?'", "firstTime: 'First time using ZSK?'"),
    ('registerSubtitle: "创建账户并开始使用 WeKnora"', 'registerSubtitle: "创建账户并开始使用 ZSK"'),
    ("registerSubtitle: 'Create an account and start using WeKnora'",
     "registerSubtitle: 'Create an account and start using ZSK'"),
    ("WeKnora 会自动解析", "ZSK 会自动解析"),
    ("WeKnora will automatically", "ZSK will automatically"),
]

# Protect tokens that must remain WeKnora*
# (applied in protect())
def protect(s: str) -> tuple[str, dict[str, str]]:
    mapping: dict[str, str] = {}
    idx = 0

    def wrap(rx: re.Pattern[str], prefix: str, text: str) -> str:
        nonlocal idx

        def _sub(m: re.Match[str]) -> str:
            nonlocal idx
            key = f"__NXIN_PROTECT_{prefix}_{idx}__"
            mapping[key] = m.group(0)
            idx += 1
            return key

        return rx.sub(_sub, text)

    s = wrap(re.compile(r"WeKnoraCloud", re.I), "WC", s)
    s = wrap(re.compile(r"WeKnora Cloud", re.I), "WCS", s)
    s = wrap(re.compile(r"WEKNORA_[A-Z0-9_]+"), "ENV", s)
    s = wrap(re.compile(r"aud=weknora"), "AUD", s)
    s = wrap(re.compile(r"X-WeKnora-[A-Za-z0-9-]+"), "HDR", s)
    s = wrap(re.compile(r"@lyingbug/weknora"), "CLAW", s)
    s = wrap(re.compile(r"weknora_token", re.I), "TOK", s)
    return s, mapping


def unprotect(s: str, mapping: dict[str, str]) -> str:
    for k, v in mapping.items():
        s = s.replace(k, v)
    return s


def apply_pairs(text: str, pairs: list[tuple[str, str]]) -> tuple[str, int]:
    n = 0
    for old, new in pairs:
        c = text.count(old)
        if c:
            text = text.replace(old, new)
            n += c
    return text, n


def process_file(path: Path, pairs: list[tuple[str, str]]) -> int:
    raw = path.read_text(encoding="utf-8")
    protected, mapping = protect(raw)
    new, n = apply_pairs(protected, pairs)
    new = unprotect(new, mapping)
    if n and new != raw:
        path.write_text(new, encoding="utf-8", newline="\n")
    return n


def fix_index_html(root: Path) -> bool:
    p = root / "frontend" / "index.html"
    if not p.exists():
        return False
    raw = p.read_text(encoding="utf-8")
    new = re.sub(r"<title>\s*WeKnora\s*</title>", "<title>NXIN-ZSK</title>", raw)
    if new != raw:
        p.write_text(new, encoding="utf-8", newline="\n")
        return True
    return False


def fix_env_utf8(root: Path) -> list[str]:
    fixed = []
    mapping = {
        "frontend/.env.development": "frontend/env.development.example",
        "frontend/.env.production": "frontend/env.production.example",
    }
    # Prefer NXIN committed examples or rewrite titles only if mojibake
    for target, example in mapping.items():
        tp = root / target
        ep = root / example
        if not tp.exists():
            continue
        text = tp.read_text(encoding="utf-8", errors="replace")
        if "农信知识库" in text and "" not in text:
            # already ok
            continue
        if "֪ʶ" in text or "农信知识" not in text or "" in text:
            if ep.exists():
                tp.write_text(ep.read_text(encoding="utf-8"), encoding="utf-8", newline="\n")
                fixed.append(target)
            else:
                # minimal repair
                text2 = text
                text2 = re.sub(r"^VITE_APP_TITLE=.*$", "VITE_APP_TITLE=农信知识库", text2, flags=re.M)
                text2 = re.sub(r"^VITE_APP_NAME=.*$", "VITE_APP_NAME=WeKnora-Nxin", text2, flags=re.M)
                if text2 != text:
                    tp.write_text(text2, encoding="utf-8", newline="\n")
                    fixed.append(target)
    return fixed


def main() -> None:
    for root in ROOTS:
        print(f"== {root} ==")
        # prompts
        prompt_dir = root / "config" / "prompt_templates"
        total = 0
        if prompt_dir.exists():
            for p in sorted(prompt_dir.glob("*.yaml")):
                n = process_file(p, PROMPT_REPLACEMENTS)
                if n:
                    print(f"  prompt {p.name}: {n}")
                    total += n
        # i18n
        for loc in ["zh-CN.ts", "en-US.ts", "ko-KR.ts", "ru-RU.ts"]:
            p = root / "frontend" / "src" / "i18n" / "locales" / loc
            if p.exists():
                n = process_file(p, I18N_REPLACEMENTS)
                # extra locale-specific patterns
                raw = p.read_text(encoding="utf-8")
                protected, mapping = protect(raw)
                extra = 0
                # Korean welcome remnants
                for old, new in [
                    ('welcome: "WeKnora에 오신 것을 환영합니다"', 'welcome: "ZSK에 오신 것을 환영합니다"'),
                    ("welcome: 'WeKnora에 오신 것을 환영합니다'", "welcome: 'ZSK에 오신 것을 환영합니다'"),
                    ('welcome: "Добро пожаловать в WeKnora"', 'welcome: "Добро пожаловать в ZSK"'),
                ]:
                    c = protected.count(old)
                    if c:
                        protected = protected.replace(old, new)
                        extra += c
                protected = unprotect(protected, mapping)
                if protected != raw:
                    p.write_text(protected, encoding="utf-8", newline="\n")
                    n += extra
                if n:
                    print(f"  i18n {loc}: {n}")
                    total += n
        if fix_index_html(root):
            print("  index.html title -> NXIN-ZSK")
            total += 1
        env_fixed = fix_env_utf8(root)
        for e in env_fixed:
            print(f"  env utf8 fixed: {e}")
        print(f"  total replacements≈{total}")


if __name__ == "__main__":
    main()
