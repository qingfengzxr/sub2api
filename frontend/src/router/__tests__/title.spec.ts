import { describe, expect, it } from 'vitest'
import { i18n } from '@/i18n'
import { resolveDocumentTitle } from '@/router/title'

describe('resolveDocumentTitle', () => {
  it('路由存在标题时，使用“路由标题 - 站点名”格式', () => {
    expect(resolveDocumentTitle('Usage Records', 'My Site')).toBe('Usage Records - My Site')
  })

  it('路由无标题时，回退到站点名', () => {
    expect(resolveDocumentTitle(undefined, 'My Site')).toBe('My Site')
  })

  it('站点名为空时，回退默认站点名', () => {
    i18n.global.locale.value = 'en'

    expect(resolveDocumentTitle('Dashboard', '')).toBe('Dashboard - BOBRAI')
    expect(resolveDocumentTitle(undefined, '   ')).toBe('BOBRAI')
  })

  it('中文环境下默认品牌名使用中文显示', () => {
    i18n.global.locale.value = 'zh'

    expect(resolveDocumentTitle('Dashboard', 'BOBRAI')).toBe('Dashboard - 河狸AI')
    expect(resolveDocumentTitle(undefined, 'BOBR')).toBe('河狸')
  })

  it('站点名变更时仅影响后续路由标题计算', () => {
    const before = resolveDocumentTitle('Admin Dashboard', 'Alpha')
    const after = resolveDocumentTitle('Admin Dashboard', 'Beta')

    expect(before).toBe('Admin Dashboard - Alpha')
    expect(after).toBe('Admin Dashboard - Beta')
  })
})
