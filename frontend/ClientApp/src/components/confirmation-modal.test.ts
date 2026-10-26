import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import ConfirmationModal from './ConfirmationModal.vue'

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = vi.fn()
  HTMLDialogElement.prototype.close = vi.fn()
})

it.each([undefined, 'Summary', 'Tags'])('labels the editor %s while preserving required validation', async (label) => {
  const wrapper = mount(ConfirmationModal, { global: { stubs: { teleport: true } } })
  const result = wrapper.vm.reveal({ message: 'Edit value', editor: { requiredText: true, label } })
  await flushPromises()
  const fieldLabel = wrapper.get('label')
  expect(fieldLabel.text()).toBe(label ?? 'Comment')
  expect(fieldLabel.attributes('for')).toBe(wrapper.get('textarea').attributes('id'))
  expect(wrapper.get('[data-testid="confirmation-confirm"]').attributes('disabled')).toBeDefined()
  await wrapper.get('textarea').setValue('  ')
  expect(wrapper.get('[data-testid="confirmation-confirm"]').attributes('disabled')).toBeDefined()
  await wrapper.get('textarea').setValue('Provided value')
  expect(wrapper.get('[data-testid="confirmation-confirm"]').attributes('disabled')).toBeUndefined()
  await wrapper.get('dialog').trigger('cancel')
  expect((await result).isCanceled).toBe(true)
  wrapper.unmount()
})
