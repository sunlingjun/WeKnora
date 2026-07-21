"""Restore shared-space (OrganizationList) brand tinting after upstream merge.

Replaces hardcoded WeKnora green/blue rgba with color-mix(... var(--td-brand-color) ...).
Infinity create icon should use inline SVG + currentColor (not organization-green.svg #07C05F).
"""
from __future__ import annotations

import re
from pathlib import Path

ROOTS = [
    Path(r"E:/Tencent/WeKnora-slj"),
    Path(r"E:/Tencent/NXIN-WEKNORA"),
]

REL = Path("frontend/src/views/organization/OrganizationList.vue")

PATTERNS = [
    re.compile(r"rgba\(\s*7\s*,\s*192\s*,\s*95\s*,\s*([0-9]*\.?[0-9]+)\s*\)"),
    re.compile(r"rgba\(\s*0\s*,\s*82\s*,\s*217\s*,\s*([0-9]*\.?[0-9]+)\s*\)"),
]


def pct(a: float) -> str:
    p = round(a * 100, 2)
    if p == int(p):
        return str(int(p))
    return ("%s" % p).rstrip("0").rstrip(".")


def repl(m: re.Match) -> str:
    return f"color-mix(in srgb, var(--td-brand-color) {pct(float(m.group(1)))}%, transparent)"


def main() -> None:
    for root in ROOTS:
        path = root / REL
        if not path.exists():
            print(f"skip missing {path}")
            continue
        raw = path.read_text(encoding="utf-8")
        new = raw
        for rx in PATTERNS:
            before = len(rx.findall(new))
            new = rx.sub(repl, new)
            print(f"{path}: {before} replacements via {rx.pattern[:40]}...")
        leftover_img = "organization-green.svg" in new
        leftover_rgba = bool(
            re.search(r"rgba\(\s*7\s*,\s*192\s*,\s*95", new)
            or re.search(r"rgba\(\s*0\s*,\s*82\s*,\s*217", new)
        )
        if new != raw:
            path.write_text(new, encoding="utf-8", newline="\n")
        print(
            f"  written={new != raw} leftover_img={leftover_img} leftover_rgba={leftover_rgba}"
        )


if __name__ == "__main__":
    main()
