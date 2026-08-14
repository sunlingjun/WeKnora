import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveMemberDisplayName, resolveMemberEmailLine } from './memberDisplayName.ts'

const unknown = '未知用户'

test('CAS mailbox username uses local-part as title and keeps email as subtitle', () => {
  const user = { name: 'zhangsanTest@nxin.local', email: 'zhangsanTest@nxin.local' }
  const title = resolveMemberDisplayName(user, unknown)
  assert.equal(title, 'zhangsanTest')
  assert.equal(resolveMemberEmailLine(user, title), 'zhangsanTest@nxin.local')
})

test('cas_real_name wins over mailbox username', () => {
  const user = {
    username: 'zhangsanTest@nxin.local',
    cas_real_name: '张三',
    email: 'zhangsanTest@nxin.local',
  }
  const title = resolveMemberDisplayName(user, unknown)
  assert.equal(title, '张三')
  assert.equal(resolveMemberEmailLine(user, title), 'zhangsanTest@nxin.local')
})

test('distinct username and email both show', () => {
  const user = { username: 'alice', email: 'alice@nxin.local' }
  const title = resolveMemberDisplayName(user, unknown)
  assert.equal(title, 'alice')
  assert.equal(resolveMemberEmailLine(user, title), 'alice@nxin.local')
})

test('identical title and email hide the duplicate subtitle', () => {
  const user = { cas_real_name: 'bob@nxin.local', email: 'bob@nxin.local' }
  const title = resolveMemberDisplayName(user, unknown)
  assert.equal(title, 'bob@nxin.local')
  assert.equal(resolveMemberEmailLine(user, title), '')
})

test('missing user falls back to unknown label', () => {
  assert.equal(resolveMemberDisplayName(undefined, unknown), unknown)
  assert.equal(resolveMemberEmailLine(undefined, unknown), '')
})
