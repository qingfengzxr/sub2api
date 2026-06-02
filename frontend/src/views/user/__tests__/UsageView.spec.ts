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
  'admin.usage.ipAddress': 'IP',
  'admin.usage.inputTokens': 'Input Tokens',
  'admin.usage.outputTokens': 'Output Tokens',
  'admin.usage.cacheCreationTokens': 'Cache Write Tokens',
  'admin.usage.cacheCreation5mTokens': 'Cache Write Tokens',
  'admin.usage.cacheCreation1hTokens': 'Cache Write Tokens',
  'admin.usage.cacheReadTokens': 'Cache Read Tokens',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.imageUnit': ' images',
  'usage.imageCount': 'Image count',
  'usage.imageBillingSize': 'Billing size',
  'usage.imageInputSize': 'Input size',
  'usage.imageOutputSize': 'Output size',
  'usage.imageSizeSource': 'Size source',
  'usage.imageSizeBreakdown': 'Size breakdown',
  'usage.imageSizeSourceOutput': 'Upstream output',
  'usage.imageSizeSourceInput': 'Request input',
  'usage.imageSizeSourceDefault': 'Default billing tier',
  'usage.imageSizeSourceLegacy': 'Legacy record',
  'usage.imageSizeSourceMissing': 'Not recorded',
  'usage.imageSizeNotRecorded': 'not recorded',
  'usage.imageSizeLegacyUnstandardized': 'legacy unstandardized',
  'usage.imageSizeUnknown': 'unknown',
  'usage.imageUnitPrice': 'Per-image price',
  'usage.imageTotalPrice': 'Image total price',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'admin.usage.billingModeToken': 'Token',
  'admin.usage.billingModePerRequest': 'Per request',
  'admin.usage.billingModeImage': 'Image',
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

  it('shows standard token prices while preserving final cost and service tier', async () => {
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
          standard_input_price_per_million: 2.5,
          standard_output_price_per_million: 15,
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
      standard_input_price_per_million: 2.5,
      standard_output_price_per_million: 15,
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
    expect(text).toContain('$2.5000 / 1M tokens')
    expect(text).toContain('$15.0000 / 1M tokens')
    expect(text).not.toContain('$5.0000 / 1M tokens')
    expect(text).not.toContain('$30.0000 / 1M tokens')
  })

  it('shows standard image unit price instead of cost-derived image price', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-image-standard-price',
          actual_cost: 0.4,
          total_cost: 0.4,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 2,
          image_size: '2K',
          standard_unit_price: 0.06,
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
      total_tokens: 0,
      total_cost: 0.4,
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
      request_id: 'req-image-standard-price',
      actual_cost: 0.4,
      total_cost: 0.4,
      rate_multiplier: 1,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
      image_count: 2,
      image_size: '2K',
      image_input_size: null,
      image_output_size: null,
      image_size_source: null,
      image_size_breakdown: null,
      standard_unit_price: 0.06,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Per-image price')
    expect(text).toContain('$0.060000')
    expect(text).not.toContain('$0.200000')
  })

  it('shows a placeholder when standard token prices are unavailable', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-missing-standard-price',
          actual_cost: 0.03,
          total_cost: 0.03,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0.01,
          output_cost: 0.02,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 1000,
          output_tokens: 1000,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
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
      total_tokens: 2000,
      total_cost: 0.03,
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
      request_id: 'req-missing-standard-price',
      actual_cost: 0.03,
      total_cost: 0.03,
      rate_multiplier: 1,
      input_cost: 0.01,
      output_cost: 0.02,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 1000,
      output_tokens: 1000,
      image_count: 0,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Input price-')
    expect(text).toContain('Output price-')
    expect(text).not.toContain('$10.0000 / 1M tokens')
    expect(text).not.toContain('$20.0000 / 1M tokens')
  })

  it('renders usage IP values with fallback for missing records', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-ip-present',
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
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
          model: 'gpt-5.4',
          ip_address: '203.0.113.10',
        },
        {
          request_id: 'req-user-ip-missing',
          actual_cost: 0.02,
          total_cost: 0.02,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 8,
          output_tokens: 3,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:01:00Z',
          model: 'gpt-5.4',
          ip_address: null,
        },
      ],
      total: 2,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 2,
      total_tokens: 28,
      total_cost: 0.03,
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

    const ipCells = wrapper.findAll('.cell-ip_address')
    expect(ipCells.map((cell) => cell.text())).toEqual(['203.0.113.10', '-'])
  })

  it('exports user csv with billable-first tokens and no internal billing columns', async () => {
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
        billable_input_tokens: 10143,
        billable_output_tokens: 253,
        billable_cache_creation_tokens: 10,
        billable_cache_read_tokens: 695680,
        billable_image_output_tokens: 0,
        billing_token_multiplier: 2.5,
        image_count: 0,
        image_size: null,
        first_token_ms: 12,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-5.4',
        reasoning_effort: null,
        ip_address: '203.0.113.10',
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

    const csv = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result))
      reader.onerror = () => reject(reader.error)
      reader.readAsText(exportedBlob!)
    })
    const [headerLine, dataLine] = csv.split('\n')
    expect(headerLine).toContain('Billing Mode,Input Tokens,Output Tokens,Cache Read Tokens,Cache Creation Tokens,Image Output Tokens,Final Cost')
    expect(headerLine.split(',')).toContain('IP')
    expect(headerLine).not.toContain('Billable')
    expect(dataLine).toContain('Token,10143,253,695680,10,0,0.09288300')
    expect(dataLine).toContain('203.0.113.10')
    expect(dataLine).not.toContain('4057,101,278272,4')
    expect(dataLine).not.toContain('2.5')

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
          Teleport: true,
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

    const text = wrapper.text()
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

  it('exports historical image rows with image billing mode derived from image_count', async () => {
    const exportedLogs = [
      {
        request_id: 'req-user-export-legacy-image',
        actual_cost: 0.2,
        total_cost: 0.2,
        rate_multiplier: 1,
        service_tier: null,
        input_cost: 0,
        output_cost: 0,
        cache_creation_cost: 0,
        cache_read_cost: 0,
        input_tokens: 0,
        output_tokens: 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        cache_creation_5m_tokens: 0,
        cache_creation_1h_tokens: 0,
        image_count: 1,
        image_size: null,
        billing_mode: null,
        first_token_ms: null,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-image-2',
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
      total_tokens: 0,
      total_cost: 0.2,
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
    const csv = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result))
      reader.onerror = () => reject(reader.error)
      reader.readAsText(exportedBlob as Blob)
    })
    expect(csv).toContain('Billing Mode')
    expect(csv).toContain('Image')
    expect(csv).not.toContain(',Token,0,0,0,0,')

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    clickSpy.mockRestore()
  })

  it('does not display a 2K fallback for historical image rows with missing size', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-legacy-missing-image',
          actual_cost: 0.2,
          total_cost: 0.2,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 1,
          image_size: null,
          image_input_size: null,
          image_output_size: null,
          image_size_source: null,
          image_size_breakdown: null,
          billing_mode: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
          model: 'gpt-image-2',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 0,
      total_cost: 0.2,
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

    const text = wrapper.text()
    expect(text).toContain('Image')
    expect(text).toContain('not recorded')
    expect(text).not.toContain('(2K)')
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
          Teleport: true,
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

    const text = wrapper.text()
    expect(text).toContain('12')
    expect(text).toContain('5')
    expect(text).toContain('3')
    expect(text).toContain('20')
    expect(text).not.toContain('Billing Check Usage')
  })

  it('shows image billing metadata in the user cost tooltip', async () => {
    query.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 0,
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

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-output-image',
      actual_cost: 0.8,
      total_cost: 0.8,
      rate_multiplier: 1,
      service_tier: null,
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      billing_mode: null,
      image_count: 2,
      image_size: '4K',
      image_input_size: '1024x1024',
      image_output_size: '3840x2160',
      image_size_source: 'output',
      image_size_breakdown: { '4K': 2 },
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Image count')
    expect(text).toContain('Billing size')
    expect(text).toContain('4K')
    expect(text).toContain('Size source')
    expect(text).toContain('Upstream output')
    expect(text).toContain('Input size')
    expect(text).toContain('1024x1024')
    expect(text).toContain('Output size')
    expect(text).toContain('3840x2160')
    expect(text).toContain('4K x 2')
  })
})
