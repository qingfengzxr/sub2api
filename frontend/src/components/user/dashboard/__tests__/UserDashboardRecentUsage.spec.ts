import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UserDashboardRecentUsage from '../UserDashboardRecentUsage.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('UserDashboardRecentUsage', () => {
  it('shows billable-first token totals', () => {
    const wrapper = mount(UserDashboardRecentUsage, {
      props: {
        loading: false,
        data: [{
          id: 1,
          model: 'gpt-test',
          created_at: '2026-09-07T00:00:00Z',
          actual_cost: 0.1,
          input_tokens: 1,
          output_tokens: 2,
          cache_creation_tokens: 3,
          cache_read_tokens: 4,
          billable_input_tokens: 10,
          billable_output_tokens: 20,
          billable_cache_creation_tokens: 30,
          billable_cache_read_tokens: 40,
        } as any],
      },
      global: {
        stubs: {
          Icon: true,
          RouterLink: true,
        },
      },
    })

    expect(wrapper.text()).toContain('100 tokens')
    expect(wrapper.text()).not.toContain('3 tokens')
  })
})
