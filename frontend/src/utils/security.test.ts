import assert from 'node:assert/strict'
import test from 'node:test'

import { collectSessionKnowledgeBaseIds, protectProviderImageSrcInHTML } from './security.ts'

test('protectProviderImageSrcInHTML uses a placeholder src for provider images', () => {
  const html = '<p><img alt="preview" src="local://10000/exports/a.jpg"></p>'
  const sanitized = protectProviderImageSrcInHTML(html)
  const renderedSrc = sanitized.match(/<img[^>]*\ssrc="([^"]+)"/)?.[1]

  assert.match(renderedSrc || '', /^data:image\/gif;base64,/)
  assert.match(sanitized, /data-protected-src="local:\/\/10000\/exports\/a\.jpg"/)
})

test('protectProviderImageSrcInHTML uses a placeholder src for storage-backend images', () => {
  const html =
    '<p><img alt="preview" src="storage://c0d93536-702c-4977-aa5e-fe670073c3cb/local://10000/exports/a.png"></p>'
  const sanitized = protectProviderImageSrcInHTML(html)
  const renderedSrc = sanitized.match(/<img[^>]*\ssrc="([^"]+)"/)?.[1]

  assert.match(renderedSrc || '', /^data:image\/gif;base64,/)
  assert.match(
    sanitized,
    /data-protected-src="storage:\/\/c0d93536-702c-4977-aa5e-fe670073c3cb\/local:\/\/10000\/exports\/a\.png"/,
  )
})

test('protectProviderImageSrcInHTML uses a placeholder src for resource references', () => {
  const html = '<p><img alt="preview" src="resource://AbCdEfGhIjKlMnOpQrStUv"></p>'
  const sanitized = protectProviderImageSrcInHTML(html)
  const renderedSrc = sanitized.match(/<img[^>]*\ssrc="([^"]+)"/)?.[1]

  assert.match(renderedSrc || '', /^data:image\/gif;base64,/)
  assert.match(
    sanitized,
    /data-protected-src="resource:\/\/AbCdEfGhIjKlMnOpQrStUv"/,
  )
})

test('collectSessionKnowledgeBaseIds merges refs, mentions, and agent stream', () => {
  const ids = collectSessionKnowledgeBaseIds(
    {
      knowledge_references: [{ knowledge_base_id: 'kb-ref' }],
      mentioned_items: [{ type: 'kb', id: 'kb-mention' }],
      knowledge_base_ids: ['kb-req'],
      agentEventStream: [
        { data: { knowledge_base_id: 'kb-tool', knowledge_base_ids: ['kb-tool-2'] } },
      ],
    },
    'kb-extra',
  )
  assert.deepEqual(ids.sort(), ['kb-extra', 'kb-mention', 'kb-ref', 'kb-req', 'kb-tool', 'kb-tool-2'].sort())
})
