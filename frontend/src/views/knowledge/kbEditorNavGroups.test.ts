import assert from 'node:assert/strict'
import test from 'node:test'

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
