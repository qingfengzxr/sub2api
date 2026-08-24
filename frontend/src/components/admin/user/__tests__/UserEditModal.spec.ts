import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserEditModal from '../UserEditModal.vue'

const { updateUser, showError, showSuccess } = vi.hoisted(() => ({
  updateUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { update: updateUser },
    userAttributes: { updateUserAttributeValues: vi.fn() }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() })
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: (operation: () => unknown) => operation() }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => ''
}))

function mountModal(overdraftLimit?: number, concurrency = 1) {
  return mount(UserEditModal, {
    props: {
      show: true,
      user: {
        id: 42,
        email: 'user@example.com',
        username: 'user',
        notes: '',
        role: 'user',
        concurrency,
        rpm_limit: 0,
        overdraft_limit: overdraftLimit
      } as any
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        UserAttributeForm: true,
        TotpStepUpDialog: true,
        Icon: true
      }
    }
  })
}

describe('UserEditModal overdraft limit', () => {
  beforeEach(() => {
    updateUser.mockReset()
    updateUser.mockResolvedValue({})
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('loads the saved limit and submits an explicit zero', async () => {
    const wrapper = mountModal(75)
    const overdraftInput = wrapper.findAll('input[type="number"]')[2]

    expect((overdraftInput.element as HTMLInputElement).value).toBe('75')
    await overdraftInput.setValue('0')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(42, expect.objectContaining({ overdraft_limit: 0 }))
  })

  it('defaults a missing value to zero and rejects negative input', async () => {
    const wrapper = mountModal()
    const overdraftInput = wrapper.findAll('input[type="number"]')[2]

    expect((overdraftInput.element as HTMLInputElement).value).toBe('0')
    await overdraftInput.setValue('-1')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(updateUser).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.users.overdraftLimitInvalid')
  })
})

describe('UserEditModal concurrency', () => {
  beforeEach(() => {
    updateUser.mockReset()
    updateUser.mockResolvedValue({})
    showError.mockReset()
  })

  it('saves an unlimited (0) concurrency instead of blocking the form', async () => {
    const wrapper = mountModal(undefined, 0)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(showError).not.toHaveBeenCalled()
    expect(updateUser).toHaveBeenCalledWith(42, expect.objectContaining({ concurrency: 0 }))
  })

  it('still rejects a negative concurrency', async () => {
    const wrapper = mountModal(undefined, 3)
    await wrapper.find('[data-test="concurrency-input"]').setValue('-1')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.users.concurrencyNonNegative')
    expect(updateUser).not.toHaveBeenCalled()
  })
})
