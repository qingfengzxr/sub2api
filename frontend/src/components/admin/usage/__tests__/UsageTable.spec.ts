import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageTable from '../UsageTable.vue'

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.tokenDetails': 'Token Breakdown',
  'usage.totalTokens': 'Total Tokens',
  'usage.rawTokenUsage': 'Raw Usage',
  'usage.billableTokenUsage': 'Billable Usage',
  'usage.billingTokenMultiplier': 'Multiplier Snapshot',
  'usage.billableInputTokens': 'Billable Input Tokens',
  'usage.billableOutputTokens': 'Billable Output Tokens',
  'usage.billableCacheCreationTokens': 'Billable Cache Write Tokens',
  'usage.billableCacheReadTokens': 'Billable Cache Read Tokens',
  'usage.billableImageOutputTokens': 'Billable Image Output Tokens',
  'usage.imageOutputTokens': 'Image Output Tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.accountMultiplier': 'Account rate',
  'usage.original': 'Original',
  'usage.userBilled': 'User billed',
  'usage.accountBilled': 'Account billed',
  'admin.usage.inputTokens': 'Input Tokens',
  'admin.usage.outputTokens': 'Output Tokens',
  'admin.usage.cacheCreationTokens': 'Cache Write Tokens',
  'admin.usage.cacheReadTokens': 'Cache Read Tokens',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.request_id">
        <slot name="cell-model" :row="row" :value="row.model" />
        <slot name="cell-tokens" :row="row" />
        <slot name="cell-cost" :row="row" />
      </div>
    </div>
  `,
}

describe('admin UsageTable tooltip', () => {
  beforeEach(() => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)
  })

  it('shows service tier and billing breakdown in cost tooltip', async () => {
    const row = {
      request_id: 'req-admin-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await wrapper.findAll('.group.relative')[1].trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Account rate')
    expect(text).toContain('User billed')
    expect(text).toContain('Account billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
    expect(text).toContain('$0.069568')
  })

  it('shows raw and billable token audit details in token tooltip', async () => {
    const row = {
      request_id: 'req-admin-token-1',
      actual_cost: 0.113297,
      total_cost: 0.102997,
      account_rate_multiplier: 1,
      rate_multiplier: 1.1,
      service_tier: null,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 1938,
      output_tokens: 597,
      cache_creation_tokens: 0,
      cache_read_tokens: 235904,
      image_output_tokens: 4,
      billable_input_tokens: 4845,
      billable_output_tokens: 1493,
      billable_cache_creation_tokens: 0,
      billable_cache_read_tokens: 589760,
      billable_image_output_tokens: 10,
      billing_token_multiplier: 2.5,
      image_count: 0,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await wrapper.findAll('.group.relative')[0].trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Raw Usage')
    expect(text).toContain('Input Tokens')
    expect(text).toContain('1,938')
    expect(text).toContain('597')
    expect(text).toContain('235,904')
    expect(text).toContain('238,443')
    expect(text).toContain('Billable Usage')
    expect(text).toContain('Billable Input Tokens')
    expect(text).toContain('4,845')
    expect(text).toContain('1,493')
    expect(text).toContain('589,760')
    expect(text).toContain('10')
    expect(text).toContain('596,108')
    expect(text).toContain('Multiplier Snapshot')
    expect(text).toContain('2.50x')
  })

  it('falls back to raw token details when admin billable fields are absent', async () => {
    const row = {
      request_id: 'req-admin-token-legacy',
      actual_cost: 0.01,
      total_cost: 0.01,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      service_tier: null,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 12,
      output_tokens: 5,
      cache_creation_tokens: 0,
      cache_read_tokens: 3,
      image_output_tokens: 0,
      billable_input_tokens: 0,
      billable_output_tokens: 0,
      billable_cache_creation_tokens: 0,
      billable_cache_read_tokens: 0,
      billable_image_output_tokens: 0,
      billing_token_multiplier: 0,
      image_count: 0,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await wrapper.findAll('.group.relative')[0].trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Raw Usage')
    expect(text).toContain('Billable Usage')
    expect(text).toContain('12')
    expect(text).toContain('5')
    expect(text).toContain('3')
    expect(text).toContain('20')
    expect(text).toContain('1.00x')
  })

  it('shows requested and upstream models separately for admin rows', () => {
    const row = {
      request_id: 'req-admin-model-1',
      model: 'claude-sonnet-4',
      upstream_model: 'claude-sonnet-4-20250514',
      actual_cost: 0,
      total_cost: 0,
      account_rate_multiplier: 1,
      rate_multiplier: 1,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
    }

    const wrapper = mount(UsageTable, {
      props: {
        data: [row],
        loading: false,
        columns: [],
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('claude-sonnet-4')
    expect(text).toContain('claude-sonnet-4-20250514')
  })
})
