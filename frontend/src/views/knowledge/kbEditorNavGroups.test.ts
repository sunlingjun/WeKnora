import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'

import {
  KB_EDITOR_INTEGRATION_NAV_KEYS,
  pickKbEditorNavItems,
} from './kbEditorNavGroups.ts'

test('integration nav keys keep members before share (merge baseline)', () => {
  assert.deepEqual([...KB_EDITOR_INTEGRATION_NAV_KEYS], ['members', 'share'])
})

test('pickKbEditorNavItems drops missing keys but keeps members when present', () => {
  const items = [
    { key: 'share', label: '共享管理' },
    { key: 'members', label: '成员' },
  ]
  assert.deepEqual(pickKbEditorNavItems(items, KB_EDITOR_INTEGRATION_NAV_KEYS), [
    { key: 'members', label: '成员' },
    { key: 'share', label: '共享管理' },
  ])
})

test('pickKbEditorNavItems still shows share when members not injected', () => {
  const items = [{ key: 'share', label: '共享管理' }]
  assert.deepEqual(pickKbEditorNavItems(items, KB_EDITOR_INTEGRATION_NAV_KEYS), [
    { key: 'share', label: '共享管理' },
  ])
})

test('KnowledgeBaseEditorModal actually wires members into grouped nav (merge baseline)', () => {
  const modal = readFileSync(new URL('./KnowledgeBaseEditorModal.vue', import.meta.url), 'utf8')
  assert.match(modal, /import KnowledgeBaseMembers from '\.\/settings\/KnowledgeBaseMembers\.vue'/)
  assert.match(modal, /KB_EDITOR_INTEGRATION_NAV_KEYS/)
  assert.match(modal, /pickItems\(\[\.\.\.KB_EDITOR_INTEGRATION_NAV_KEYS\]\)/)
  assert.match(modal, /key: 'members'/)
  assert.match(modal, /currentSection === 'members'/)
  assert.match(modal, /<KnowledgeBaseMembers/)
})

test('create shared KB still uses dedicated API and visibility UI (merge baseline)', () => {
  const modal = readFileSync(new URL('./KnowledgeBaseEditorModal.vue', import.meta.url), 'utf8')
  assert.match(modal, /createSharedKnowledgeBase/)
  assert.match(modal, /formData\.visibility === 'shared'/)
  assert.match(modal, /knowledgeEditor\.basic\.visibilityLabel/)
})
