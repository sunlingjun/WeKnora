import assert from 'node:assert/strict'
import test from 'node:test'

import { galleryImageRequest } from './galleryImageSrc.ts'

const KB_PROXY = '/api/v1/knowledge-bases/kb-1/files?file_path='
const CONTENT = '/api/v1/knowledge-bases/kb-1/images/content?url='

// Handles that are not an images/ or {tenant}/exports/ object stay on the
// binding-checked file proxy. resource:// is one of those.
for (const url of [
  'minio://bucket/a.png',
  'cos://bucket/a.png',
  's3://bucket/a.png',
  'resource://AbCdEfGhIjKlMnOpQrStUv',
  'storage://backend-1/local://10001/a.png',
]) {
  test(`${url.split('://')[0]}:// images stay on the knowledge-base file proxy`, () => {
    assert.equal(galleryImageRequest(url, 'kb-1')?.url, `${KB_PROXY}${encodeURIComponent(url)}`)
  })
}

for (const url of [
  'local://10001/exports/a.png',
  'minio://nxinweknora/10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg',
  'minio://nxinweknora/images/a00fbdb2461240f99855168f2432b3c4.png',
  'cos://bucket/10005/exports/a.png',
  's3://bucket/10005/exports/a.png',
  'tos://bucket/exports/10005/a.png',
  'oss://bucket/10005/exports/a.png',
  'ks3://bucket/10005/exports/a.png',
  'obs://bucket/10005/exports/a.png',
  'storage://backend-1/minio://nxinweknora/10005/exports/a.jpg',
  'http://localhost:9000/nxinweknora/10005/exports/096384ee-423d-44fd-bc17-4b6e473dfffe.jpg',
]) {
  test(`${url} loads through the gallery content endpoint`, () => {
    assert.equal(galleryImageRequest(url, 'kb-1')?.url, `${CONTENT}${encodeURIComponent(url)}`)
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
  'http://localhost:9000/nxinweknora/10005/raw/secret.jpg',
  'http://localhost:9000/nxinweknora/images/../secret.png',
]) {
  test(`${url} stays out of the gallery content endpoint`, () => {
    assert.equal(galleryImageRequest(url, 'kb-1'), null)
  })
}
