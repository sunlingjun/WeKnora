import re
from pathlib import Path

files = [
  Path(r"E:/Tencent/WeKnora-slj/frontend/src/views/knowledge/KnowledgeBaseList.vue"),
  Path(r"E:/Tencent/WeKnora-slj/frontend/src/views/agent/AgentList.vue"),
  Path(r"E:/Tencent/NXIN-WEKNORA/frontend/src/views/knowledge/KnowledgeBaseList.vue"),
  Path(r"E:/Tencent/NXIN-WEKNORA/frontend/src/views/agent/AgentList.vue"),
]

patterns = [
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

for path in files:
  raw = path.read_text(encoding="utf-8")
  new = raw
  counts = []
  for rx in patterns:
    before = len(rx.findall(new))
    new = rx.sub(repl, new)
    after = len(rx.findall(new))
    counts.append(f"{before}->{after}")
  if new != raw:
    path.write_text(new, encoding="utf-8", newline="\n")
  ok_cn = ("知识库" in new) or ("智能体" in new) or ("未初始化" in new)
  print(f"{path}: {' '.join(counts)} chinese_ok={ok_cn}")
