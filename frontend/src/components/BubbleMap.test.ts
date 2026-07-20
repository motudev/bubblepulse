import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import BubbleMap from './BubbleMap.vue'

// jsdom has no ResizeObserver; the component observes its SVG element on mount.
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  }
)

vi.mock('@/services/api', () => ({
  api: {
    getDashboard: vi.fn().mockResolvedValue({
      users: [],
      topics: [],
      similarity_matrix: [],
    }),
    getMe: vi.fn().mockRejectedValue(new Error('unauthenticated')),
  },
}))

function mountBubbleMap() {
  return mount(BubbleMap, {
    global: { plugins: [createPinia()] },
  })
}

describe('BubbleMap', () => {
  it('mounts without error', () => {
    const wrapper = mountBubbleMap()
    expect(wrapper.exists()).toBe(true)
  })

  it('renders the pulse board region', () => {
    const wrapper = mountBubbleMap()
    expect(wrapper.find('[role="region"]').exists()).toBe(true)
  })
})
