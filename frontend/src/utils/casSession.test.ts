import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  isNxinCASHost,
  needsCasIdentityReconcileFor,
  shouldAttachStoredJWT,
} from './casSession.ts'

test('nxin hosts are the only CAS source of truth', () => {
  assert.equal(isNxinCASHost('zsk.t.nxin.com'), true)
  assert.equal(isNxinCASHost('zsk.nxin.com'), true)
  assert.equal(isNxinCASHost('localhost'), false)
  assert.equal(isNxinCASHost('127.0.0.1'), false)
})

test('nxin attaches stored JWT only after this page load reconciled CAS', () => {
  assert.equal(shouldAttachStoredJWT('zsk.t.nxin.com', false), false)
  assert.equal(shouldAttachStoredJWT('zsk.t.nxin.com', true), true)
  assert.equal(shouldAttachStoredJWT('localhost', false), true)
})

test('nxin reconciles CAS once per full page load', () => {
  assert.equal(needsCasIdentityReconcileFor('zsk.t.nxin.com', false), true)
  assert.equal(needsCasIdentityReconcileFor('zsk.t.nxin.com', true), false)
  assert.equal(needsCasIdentityReconcileFor('localhost', false), false)
})
