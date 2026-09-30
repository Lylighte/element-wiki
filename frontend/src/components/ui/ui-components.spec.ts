import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UiButton from './UiButton.vue'
import UiCard from './UiCard.vue'

describe('UiButton', () => {
  it('applies the semantic variant, merges caller classes, and forwards native events', async () => {
    const onClick = vi.fn()
    const wrapper = mount(UiButton, {
      attrs: { class: 'custom-action', 'data-test': 'save-button', onClick },
      props: { variant: 'primary', size: 'lg', block: true },
      slots: { default: 'Save' },
    })

    expect(wrapper.classes()).toContain('bg-[var(--color-primary)]')
    expect(wrapper.classes()).toContain('custom-action')
    expect(wrapper.classes()).toContain('w-full')
    expect(wrapper.attributes('data-test')).toBe('save-button')
    await wrapper.trigger('click')
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('keeps disabled actions disabled', () => {
    const wrapper = mount(UiButton, { props: { disabled: true }, slots: { default: 'Delete' } })
    expect(wrapper.element.disabled).toBe(true)
    expect(wrapper.classes()).toContain('disabled:opacity-50')
  })
})

describe('UiCard', () => {
  it('renders the requested semantic element, padding, attributes, and content', () => {
    const wrapper = mount(UiCard, {
      attrs: { class: 'search-result', 'data-test': 'result-card' },
      props: { as: 'article', padding: 'md' },
      slots: { default: 'Document result' },
    })

    expect(wrapper.element.tagName).toBe('ARTICLE')
    expect(wrapper.classes()).toContain('p-4')
    expect(wrapper.classes()).toContain('search-result')
    expect(wrapper.attributes('data-test')).toBe('result-card')
    expect(wrapper.text()).toBe('Document result')
  })
})
