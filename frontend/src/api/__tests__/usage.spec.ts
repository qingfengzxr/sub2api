import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

import { usageAPI } from '@/api/usage'

describe('usage api', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { stats: {} } })
  })

  it('sends optional API key usage date range', async () => {
    const signal = new AbortController().signal

    await usageAPI.getDashboardApiKeysUsage([1, 2], {
      signal,
      start_date: '2026-06-01',
      end_date: '2026-06-07',
    })

    expect(post).toHaveBeenCalledWith(
      '/usage/dashboard/api-keys-usage',
      {
        api_key_ids: [1, 2],
        start_date: '2026-06-01',
        end_date: '2026-06-07',
      },
      { signal }
    )
  })
})
