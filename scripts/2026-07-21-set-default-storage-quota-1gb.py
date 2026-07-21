# -*- coding: utf-8 -*-
"""Change new-workspace default storage quota from 10GB to 1GB."""
from pathlib import Path

ROOTS = [
    Path(r"E:/Tencent/NXIN-WEKNORA"),
    Path(r"E:/Tencent/WeKnora-slj"),
]


def patch_system_setting(root: Path) -> None:
    p = root / "internal/application/service/system_setting.go"
    t = p.read_text(encoding="utf-8")
    t2 = t.replace(
        "// 0 or negative = use the in-code default (10 GB).",
        "// 0 or negative = use the in-code default (1 GB).",
    )
    t2 = t2.replace(
        'EnvName:  "WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\tDefault:  int64(10),',
        'EnvName:  "WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\tDefault:  int64(1),',
    )
    t2 = t2.replace(
        '"0 或负数表示使用内置默认值 10GB。",',
        '"0 或负数表示使用内置默认值 1GB。",',
    )
    if t2 == t:
        raise SystemExit(f"system_setting unchanged: {p}")
    p.write_text(t2, encoding="utf-8", newline="\n")
    print("ok", p)


def patch_getint_fallback(path: Path) -> None:
    t = path.read_text(encoding="utf-8")
    needle = (
        '"tenant.default_storage_quota_gb",\n'
        '\t\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n'
        "\t\t\t10,\n"
    )
    repl = (
        '"tenant.default_storage_quota_gb",\n'
        '\t\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n'
        "\t\t\t1,\n"
    )
    # tenant.go uses 3 tabs before args; system.go uses 2 tabs
    variants = [
        (
            '"tenant.default_storage_quota_gb",\n\t\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\t\t10,',
            '"tenant.default_storage_quota_gb",\n\t\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\t\t1,',
        ),
        (
            '"tenant.default_storage_quota_gb",\n\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\t10,',
            '"tenant.default_storage_quota_gb",\n\t\t"WEKNORA_TENANT_DEFAULT_STORAGE_QUOTA_GB",\n\t\t1,',
        ),
    ]
    t2 = t
    for a, b in variants:
        t2 = t2.replace(a, b)
    # gb <= 0 fallback near storage quota only — replace all gb = 10 that follow GetInt for this key is fragile;
    # do targeted: after STORAGE_QUOTA context, replace `gb = 10` that appears in those functions.
    # Safer: replace the pair blocks.
    t2 = t2.replace("\tif gb <= 0 {\n\t\tgb = 10\n\t}", "\tif gb <= 0 {\n\t\tgb = 1\n\t}")
    t2 = t2.replace("\t\tif gb <= 0 {\n\t\t\tgb = 10\n\t\t}", "\t\tif gb <= 0 {\n\t\t\tgb = 1\n\t\t}")
    if t2 == t:
        raise SystemExit(f"handler unchanged: {path}")
    path.write_text(t2, encoding="utf-8", newline="\n")
    print("ok", path)


def patch_client(root: Path) -> None:
    p = root / "client/tenant.go"
    t = p.read_text(encoding="utf-8")
    t2 = t.replace("default is 10GB", "default is 1GB").replace(
        "default:10737418240", "default:1073741824"
    )
    if t2 == t:
        print("skip client", p)
        return
    p.write_text(t2, encoding="utf-8", newline="\n")
    print("ok", p)


def patch_i18n(root: Path) -> None:
    files = [
        root / "frontend/src/i18n/locales/zh-CN.ts",
        root / "frontend/src/i18n/locales/en-US.ts",
        root / "frontend/src/i18n/locales/ko-KR.ts",
        root / "frontend/src/i18n/locales/ru-RU.ts",
    ]
    for p in files:
        if not p.exists():
            continue
        t = p.read_text(encoding="utf-8")
        t2 = t.replace("内置默认值 10GB", "内置默认值 1GB")
        t2 = t2.replace("built-in default of 10 GB", "built-in default of 1 GB")
        # ko / ru may have different wording — leave if absent
        if t2 != t:
            p.write_text(t2, encoding="utf-8", newline="\n")
            print("ok", p)
        else:
            print("skip i18n", p.name)


def main() -> None:
    for root in ROOTS:
        patch_system_setting(root)
        patch_getint_fallback(root / "internal/handler/tenant.go")
        patch_getint_fallback(root / "internal/handler/system.go")
        patch_client(root)
        patch_i18n(root)


if __name__ == "__main__":
    main()
