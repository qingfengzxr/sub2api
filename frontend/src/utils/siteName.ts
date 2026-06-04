const DEFAULT_SITE_NAME = 'BOBRAI'
const DEFAULT_SHORT_SITE_NAME = 'BOBR'
const LEGACY_DEFAULT_SITE_NAME = 'Sub2API'
const ZH_DEFAULT_SITE_NAME = '河狸AI'
const ZH_DEFAULT_SHORT_SITE_NAME = '河狸'

export function isDefaultSiteName(name?: string): boolean {
  const trimmed = name?.trim()
  return !trimmed || trimmed === DEFAULT_SITE_NAME || trimmed === DEFAULT_SHORT_SITE_NAME || trimmed === LEGACY_DEFAULT_SITE_NAME
}

export function getDefaultDisplaySiteName(locale?: string): string {
  return locale?.toLowerCase().startsWith('zh') ? ZH_DEFAULT_SITE_NAME : DEFAULT_SITE_NAME
}

export function displaySiteName(name?: string, locale?: string): string {
  const trimmed = name?.trim()
  const isZh = locale?.toLowerCase().startsWith('zh')

  if (trimmed === DEFAULT_SHORT_SITE_NAME) {
    return isZh ? ZH_DEFAULT_SHORT_SITE_NAME : DEFAULT_SHORT_SITE_NAME
  }

  if (trimmed && !isDefaultSiteName(trimmed)) {
    return trimmed
  }

  return getDefaultDisplaySiteName(locale)
}
