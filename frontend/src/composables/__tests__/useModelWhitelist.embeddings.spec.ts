import { describe, expect, it } from 'vitest'
import { getModelsByPlatform } from '../useModelWhitelist'

describe('OpenAI Embeddings model candidates', () => {
  it('offers all three models without adding them to other platforms', () => {
    for (const model of ['text-embedding-3-small', 'text-embedding-3-large', 'text-embedding-ada-002']) {
      expect(getModelsByPlatform('openai')).toContain(model)
      expect(getModelsByPlatform('anthropic')).not.toContain(model)
      expect(getModelsByPlatform('gemini')).not.toContain(model)
    }
  })
})
