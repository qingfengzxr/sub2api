import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { h, nextTick } from 'vue'

import UsageView from '../UsageView.vue'

const { query, getStatsByDateRange, list, showError, showWarning, showSuccess, showInfo } = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  list: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'usage.tokenDetails': 'Token Breakdown',
  'usage.totalTokens': 'Total Tokens',
  'usage.cacheTtlOverriddenLabel': 'TTL Override',
  'usage.cacheTtlOverridden5m': 'Billed as 5m',
  'usage.cacheTtlOverridden1h': 'Billed as 1h',
  'usage.imageOutputTokens': 'Image Output Tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
  'admin.usage.inputTokens': 'Input Tokens',
  'admin.usage.outputTokens': 'Output Tokens',
  'admin.usage.cacheCreationTokens': 'Cache Write Tokens',
  'admin.usage.cacheCreation5mTokens': 'Cache Write Tokens',
  'admin.usage.cacheCreation1hTokens': 'Cache Write Tokens',
  'admin.usage.cacheReadTokens': 'Cache Read Tokens',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot name="pagination" /><slot /></div>',
}
const DataTableStub = {
  props: ['columns', 'data'],
  setup(props: { columns: Array<{ key: string }>; data: Array<Record<string, unknown>> }, { slots }: any) {
    return () =>
      h(
        'div',
        props.data.flatMap((row) =>
          props.columns.map((column) => {
            const slot = slots[`cell-${column.key}`]
            return h('div', { class: `cell-${column.key}` }, slot ? slot({ row, value: row[column.key] }) : String(row[column.key] ?? ''))
          })
        )
      )
  },
}

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    query.mockReset()
    getStatsByDateRange.mockReset()
    list.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

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

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    })
  })

  it('shows final cost and service tier in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 0,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('$0.092883')
    expect(text).not.toContain('Rate')
    expect(text).not.toContain('1.00x')
    expect(text).not.toContain('Billed')
    expect(text).not.toContain('$5.0000 / 1M tokens')
    expect(text).not.toContain('$30.0000 / 1M tokens')
  })

  it('exports csv with input and output unit price columns', async () => {
    const exportedLogs = [
      {
        request_id: 'req-user-export',
        actual_cost: 0.092883,
        total_cost: 0.092883,
        rate_multiplier: 1,
        service_tier: 'priority',
        input_cost: 0.020285,
        output_cost: 0.00303,
        cache_creation_cost: 0.000001,
        cache_read_cost: 0.069568,
        input_tokens: 4057,
        output_tokens: 101,
        cache_creation_tokens: 4,
        cache_read_tokens: 278272,
        cache_creation_5m_tokens: 0,
        cache_creation_1h_tokens: 0,
        image_count: 0,
        image_size: null,
        first_token_ms: 12,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-5.4',
        reasoning_effort: null,
        api_key: { name: 'demo-key' },
      },
    ]

    query.mockResolvedValue({
      items: exportedLogs,
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    let exportedBlob: Blob | null = null
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    await setupState.exportToCSV()

    expect(exportedBlob).not.toBeNull()
    const hasSortedExportQuery = query.mock.calls.some((call) => {
      const params = call[0] as Record<string, unknown> | undefined
      const config = call[1]
      return (
        params?.page_size === 100 &&
        params?.sort_by === 'created_at' &&
        params?.sort_order === 'desc' &&
        config === undefined
      )
    })
    expect(hasSortedExportQuery).toBe(true)
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    clickSpy.mockRestore()
  })

  it('shows billable tokens in token tooltip without separate billing-check section', async () => {
    query.mockResolvedValue({
      items: [
        {
          id: 1,
          request_id: 'req-token-tooltip',
          actual_cost: 0.01,
          total_cost: 0.01,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 604,
          output_tokens: 269,
          cache_creation_tokens: 0,
          cache_read_tokens: 175488,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          billable_input_tokens: 1510,
          billable_output_tokens: 673,
          billable_cache_creation_tokens: 0,
          billable_cache_read_tokens: 438720,
          billable_image_output_tokens: 0,
          billing_token_multiplier: 2.5,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
          model: 'gpt-5.4',
          reasoning_effort: null,
          api_key: { name: 'demo-key' },
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const tableText = wrapper.text()
    expect(tableText).toContain('1,510')
    expect(tableText).toContain('673')
    expect(tableText).toContain('438.7K')
    expect(tableText).not.toContain('604')
    expect(tableText).not.toContain('269')
    expect(tableText).not.toContain('175.5K')

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tokenTooltipData = {
      id: 1,
      request_id: 'req-token-tooltip',
      input_tokens: 604,
      output_tokens: 269,
      cache_creation_tokens: 0,
      cache_read_tokens: 175488,
      cache_creation_5m_tokens: 0,
      cache_creation_1h_tokens: 0,
      billable_input_tokens: 1510,
      billable_output_tokens: 673,
      billable_cache_creation_tokens: 0,
      billable_cache_read_tokens: 438720,
      billable_image_output_tokens: 0,
      billing_token_multiplier: 2.5,
    }
    setupState.tokenTooltipVisible = true
    await nextTick()

    const text = document.body.textContent || ''
    expect(text).toContain('1,510')
    expect(text).toContain('673')
    expect(text).toContain('438,720')
    expect(text).toContain('440,903')
    expect(text).not.toContain('604')
    expect(text).not.toContain('269')
    expect(text).not.toContain('175,488')
    expect(text).not.toContain('Billing Check Usage')
    expect(text).not.toContain('2.5')
  })

  it('falls back to raw tokens when billable token fields are absent', async () => {
    query.mockResolvedValue({
      items: [
        {
          id: 2,
          request_id: 'req-token-tooltip-legacy',
          actual_cost: 0.01,
          total_cost: 0.01,
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
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          billable_input_tokens: 0,
          billable_output_tokens: 0,
          billable_cache_creation_tokens: 0,
          billable_cache_read_tokens: 0,
          billable_image_output_tokens: 0,
          billing_token_multiplier: 1,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
          model: 'gpt-5.4',
          reasoning_effort: null,
          api_key: { name: 'demo-key' },
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          Select: true,
          DateRangePicker: true,
          DataTable: DataTableStub,
          Icon: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const tableText = wrapper.text()
    expect(tableText).toContain('12')
    expect(tableText).toContain('5')
    expect(tableText).toContain('3')

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tokenTooltipData = {
      id: 2,
      request_id: 'req-token-tooltip-legacy',
      input_tokens: 12,
      output_tokens: 5,
      cache_creation_tokens: 0,
      cache_read_tokens: 3,
      cache_creation_5m_tokens: 0,
      cache_creation_1h_tokens: 0,
      billable_input_tokens: 0,
      billable_output_tokens: 0,
      billable_cache_creation_tokens: 0,
      billable_cache_read_tokens: 0,
      billable_image_output_tokens: 0,
      billing_token_multiplier: 1,
    }
    setupState.tokenTooltipVisible = true
    await nextTick()

    const text = document.body.textContent || ''
    expect(text).toContain('12')
    expect(text).toContain('5')
    expect(text).toContain('3')
    expect(text).toContain('20')
    expect(text).not.toContain('Billing Check Usage')
  })
})
