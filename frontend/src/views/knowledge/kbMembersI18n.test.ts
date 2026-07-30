import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '../../i18n/locales')

/** Keys referenced by KnowledgeBaseMembers.vue — must stay in knowledgeList. */
const REQUIRED = [
  'knowledgeList.members.title',
  'knowledgeList.members.description',
  'knowledgeList.members.searchPlaceholder',
  'knowledgeList.members.empty',
  'knowledgeList.members.unknownUser',
  'knowledgeList.members.joinedAt',
  'knowledgeList.members.confirmRemoveTitle',
  'knowledgeList.members.confirmRemoveMessage',
  'knowledgeList.members.actions.setEditor',
  'knowledgeList.members.actions.setViewer',
  'knowledgeList.members.actions.remove',
  'knowledgeList.messages.fetchMembersFailed',
  'knowledgeList.messages.roleUpdated',
  'knowledgeList.messages.roleUpdateFailed',
  'knowledgeList.messages.memberRemoved',
  'knowledgeList.messages.memberRemoveFailed',
]

function hasLeaf(src: string, path: string): boolean {
  // Ensure the leaf key name appears inside a knowledgeList.members / messages section.
  const leaf = path.split('.').pop()!
  const section = path.includes('.members.actions.')
    ? /members:\s*\{[\s\S]*?actions:\s*\{[\s\S]*?\}/
    : path.includes('.members.')
      ? /members:\s*\{[\s\S]*?actions:\s*\{/
      : /messages:\s*\{[\s\S]*?memberRemoveFailed/
  const m = src.match(/knowledgeList:\s*\{[\s\S]*?\r?\n  \},?\r?\n  sharedKbSquare:/)
  const block = m?.[0] ?? ''
  if (!section.test(block) && path.includes('.members.')) return false
  return new RegExp(`${leaf}\\s*:`).test(block)
}

for (const loc of ['zh-CN', 'en-US', 'ko-KR', 'ru-RU']) {
  test(`knowledgeList.members i18n present in ${loc} (merge baseline)`, () => {
    const src = readFileSync(join(root, `${loc}.ts`), 'utf8')
    const missing = REQUIRED.filter((k) => !hasLeaf(src, k))
    assert.deepEqual(missing, [], `missing keys in ${loc}: ${missing.join(', ')}`)
  })
}
