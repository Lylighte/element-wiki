import { describe, expect, it } from 'vitest'
import { formatSiteDate } from './siteDate'

describe('formatSiteDate', () => {
  it('formats Unix milliseconds in the site zone, including daylight saving changes', () => {
    const before = Date.UTC(2025, 2, 30, 0, 30)
    const after = Date.UTC(2025, 2, 30, 1, 30)
    expect(formatSiteDate(before, 'en', 'Europe/Berlin')).toContain('01:30')
    expect(formatSiteDate(after, 'en', 'Europe/Berlin')).toContain('03:30')
    expect(formatSiteDate(after, 'en', 'Europe/Berlin')).toContain('(Europe/Berlin)')
    expect(formatSiteDate(after, 'zh-CN', 'Asia/Shanghai')).toContain('09:30')
  })
})
