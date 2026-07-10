import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BubbleMap from './BubbleMap.vue'

describe('BubbleMap', () => {
  it('mounts without error', () => {
    const wrapper = mount(BubbleMap)
    expect(wrapper.exists()).toBe(true)
  })

  it('renders the placeholder text', () => {
    const wrapper = mount(BubbleMap)
    expect(wrapper.text()).toContain('BubbleMap coming soon')
  })
})
