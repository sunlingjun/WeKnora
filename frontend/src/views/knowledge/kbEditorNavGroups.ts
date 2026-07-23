/**
 * Knowledge-base editor sidebar group key lists.
 *
 * navItems may inject conditional entries (e.g. members for shared+owner);
 * navGroups must pick every key that should appear, or the entry is silently
 * dropped from the sidebar (regression: members list "lost" after grouped nav).
 */

/** 发布集成：库成员（条件项）在前，共享管理在后。 */
export const KB_EDITOR_INTEGRATION_NAV_KEYS = ['members', 'share'] as const

export function pickKbEditorNavItems<T extends { key: string }>(
  items: T[],
  keys: readonly string[],
): T[] {
  const map = new Map(items.map((item) => [item.key, item]))
  return keys.map((key) => map.get(key)).filter(Boolean) as T[]
}
