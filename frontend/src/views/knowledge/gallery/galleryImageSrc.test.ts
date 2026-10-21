import assert from 'node:assert/strict'
import test from 'node:test'

import { galleryImageRequest } from './galleryImageSrc.ts'

const KB_PROXY = '/api/v1/knowledge-bases/kb-1/files?file_path='

// The default local storage writes local:// handles; they, and every other
// storage scheme, must reach the knowledge-base proxy instead of <img src>.
for (const url of [
  'local://10001/exports/a.png',
  'minio://bucket/a.png',
  'cos://bucket/a.png',
  's3://bucket/a.png',
  'resource://AbCdEfGhIjKlMnOpQrStUv',
  'storage://backend-1/local://10001/a.png',
]) {
  test(`${url.split('://')[0]}:// images load through the knowledge-base file proxy`, () => {
    assert.equal(galleryImageRequest(url, 'kb-1')?.url, `${KB_PROXY}${encodeURIComponent(url)}`)
  })
}

test('browser-renderable URLs are used as is', () => {
  assert.equal(galleryImageRequest('https://cdn.example.com/a.png', 'kb-1'), null)
  assert.equal(galleryImageRequest('', 'kb-1'), null)
})

const STORED_MINIO = 'http://localhost:9000/nxinweknora/images/a00fbdb2461240f99855168f2432b3c4.png'

test('private MinIO HTTP images load through the gallery content endpoint', () => {
  const req = galleryImageRequest(STORED_MINIO, 'kb-1')
  assert.equal(
    req?.url,
    `/api/v1/knowledge-bases/kb-1/images/content?url=${encodeURIComponent(STORED_MINIO)}`,
  )
})

for (const url of [
  'http://127.0.0.1:9000/nxinweknora/images/a.png',
  'http://minio:9000/nxinweknora/images/a.png',
  'https://host.docker.internal:9000/nxinweknora/images/a.png',
]) {
  test(`${url} is an internal MinIO image`, () => {
    assert.ok(galleryImageRequest(url, 'kb-1')?.url.includes('/images/content?url='))
  })
}

for (const url of [
  'http://localhost/nxinweknora/images/a.png',
  'http://evil.example:9000/nxinweknora/images/a.png',
  'http://localhost:9000/nxinweknora/exports/a.png',
  'http://localhost:9000/nxinweknora/images/../secret.png',
]) {
  test(`${url} stays out of the gallery content endpoint`, () => {
    assert.equal(galleryImageRequest(url, 'kb-1'), null)
  })
}
