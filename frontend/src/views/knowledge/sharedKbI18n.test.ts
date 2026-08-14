import assert from 'node:assert/strict'
import test from 'node:test'

import { getLocaleValueAtPath, LOCALE_BUNDLES } from '../../i18n/localeKeyAudit.ts'

/**
 * Shared knowledge-base square + list tags.
 *
 * Official i18n prune uses zh-CN as source and drops unused namespaces.
 * localeKeyAudit's scan regex only matches *existing* top-level namespaces,
 * so `$t('sharedKbSquare.title')` is invisible once the tree is gone.
 * This test is the merge-baseline lock: do not rely on localeKeyAudit alone.
 */
const REQUIRED = [
  'sharedKbSquare.title',
  'sharedKbSquare.subtitle',
  'sharedKbSquare.searchPlaceholder',
  'sharedKbSquare.search',
  'sharedKbSquare.join',
  'sharedKbSquare.empty',
  'sharedKbSquare.noSearchResult',
  'sharedKbSquare.noDescription',
  'sharedKbSquare.memberCount',
  'sharedKbSquare.knowledgeCount',
  'sharedKbSquare.fetchFailed',
  'knowledgeList.sharedTag',
  'knowledgeList.leave',
  'knowledgeList.role.owner',
  'knowledgeList.role.editor',
  'knowledgeList.role.viewer',
  'knowledgeList.sections.joinedShared',
  'knowledgeList.messages.leftSuccess',
  'knowledgeList.messages.leftFailed',
  'knowledgeList.messages.joinedSuccess',
  'knowledgeList.messages.joinedFailed',
] as const

for (const [loc, bundle] of Object.entries(LOCALE_BUNDLES)) {
  test(`shared knowledge base i18n present in ${loc} (merge baseline)`, () => {
    const missing = REQUIRED.filter((key) => typeof getLocaleValueAtPath(bundle, key) !== 'string')
    assert.deepEqual(missing, [], `missing keys in ${loc}: ${missing.join(', ')}`)
  })
}
