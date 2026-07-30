import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { type GroupDraft, newAtomic } from '@template-repository/components/clauses-editor/constraint-draft'
import OdrlRuleBuilder from '@template-repository/components/clauses-editor/OdrlRuleBuilder.vue'
import type { OdrlRule } from '@/models/dcs-jsonld'

/**
 * The builder is mounted once and authors one clause after another: the host
 * clears the rule it binds when a clause is saved. What the builder holds must
 * follow the clause, not the component's lifetime.
 */

const TEMPLATE_IRI = 'urn:uuid:template'

interface RuleBuilderDraft {
  type: string
  actions: string[]
  assigneeId: string
  assignerId: string
  targetId: string
  root: GroupDraft
  duties: { action: string; root: GroupDraft; consequences: { action: string; root: GroupDraft }[] }[]
}

/** A host binding the builder with v-model, as the clause editor does. */
const Host = defineComponent({
  setup(_props, { expose }) {
    const rule = ref<OdrlRule | null>(null)
    expose({ rule })
    return () =>
      h(OdrlRuleBuilder, {
        modelValue: rule.value,
        'onUpdate:modelValue': (value: OdrlRule | null) => {
          rule.value = value
        },
        fields: [{ id: `${TEMPLATE_IRI}#field-nodes`, label: 'Provisioned nodes' }],
        assets: [],
        parties: [
          { id: `${TEMPLATE_IRI}#party-provider`, label: 'Provider' },
          { id: `${TEMPLATE_IRI}#party-customer`, label: 'Customer' },
        ],
        proseId: `${TEMPLATE_IRI}#block-clause`,
        contractTargetId: TEMPLATE_IRI,
      })
  },
})

function mountHost() {
  const wrapper = mount(Host)
  const host = wrapper.vm as unknown as { rule: OdrlRule | null }
  const draft = (wrapper.findComponent(OdrlRuleBuilder).vm as unknown as { draft: RuleBuilderDraft }).draft
  return { wrapper, host, draft }
}

/** Authors a complete permission with a constraint and a duty. */
function authorPermission(draft: RuleBuilderDraft): void {
  draft.type = 'odrl:Permission'
  draft.actions = ['odrl:execute']
  draft.assignerId = `${TEMPLATE_IRI}#party-provider`
  draft.assigneeId = `${TEMPLATE_IRI}#party-customer`
  draft.root.children.push(newAtomic(`${TEMPLATE_IRI}#field-nodes`, 'odrl:lteq'))
  draft.duties.push({
    action: 'odrl:compensate',
    root: { kind: 'group', combine: 'and', children: [newAtomic(`${TEMPLATE_IRI}#field-nodes`, 'odrl:gt')] },
    consequences: [],
  })
}

describe('OdrlRuleBuilder draft lifetime', () => {
  it('resets to a pristine draft when the host clears the rule (clause saved)', async () => {
    const { wrapper, host, draft } = mountHost()
    authorPermission(draft)
    await wrapper.vm.$nextTick()
    const firstRule = host.rule
    expect(firstRule).not.toBeNull()

    // Saving the clause clears the bound rule.
    host.rule = null
    await wrapper.vm.$nextTick()

    expect(draft.duties).toHaveLength(0)
    expect(draft.root.children).toHaveLength(0)
    expect(draft.actions).toEqual(['odrl:use'])
    expect(draft.type).toBe('odrl:Permission')
    // The pristine draft is not pushed back out as a rule the author never made.
    expect(host.rule).toBeNull()

    // The next clause's rule is a new rule, not an overwrite of the saved one.
    draft.actions = ['odrl:distribute']
    await wrapper.vm.$nextTick()
    expect(host.rule).not.toBeNull()
    expect(host.rule!['@id']).not.toBe(firstRule!['@id'])
    expect(host.rule!['odrl:duty']).toBeUndefined()
    expect(host.rule!['odrl:constraint']).toBeUndefined()
  })

  it('keeps the draft when the rule reads incomplete mid-edit', async () => {
    const { wrapper, host, draft } = mountHost()
    authorPermission(draft)
    await wrapper.vm.$nextTick()
    expect(host.rule).not.toBeNull()

    // Clearing the assignee makes the rule incomplete: the builder emits null
    // while the author is still editing.
    draft.assigneeId = ''
    await wrapper.vm.$nextTick()
    expect(host.rule).toBeNull()

    expect(draft.duties).toHaveLength(1)
    expect(draft.root.children).toHaveLength(1)
    expect(draft.actions).toEqual(['odrl:execute'])
  })

  it('reseeds from a rule handed in from outside', async () => {
    const { wrapper, host, draft } = mountHost()
    const existing: OdrlRule = {
      '@id': `${TEMPLATE_IRI}#rule-existing`,
      '@type': 'odrl:Prohibition',
      'odrl:action': { '@id': 'odrl:archive' },
      'odrl:assigner': { '@id': `${TEMPLATE_IRI}#party-provider` },
      'odrl:assignee': { '@id': `${TEMPLATE_IRI}#party-customer` },
      'odrl:target': { '@id': TEMPLATE_IRI },
      'dcs:prose': { '@id': `${TEMPLATE_IRI}#block-clause` },
    }
    host.rule = existing
    await wrapper.vm.$nextTick()

    expect(draft.type).toBe('odrl:Prohibition')
    expect(draft.actions).toEqual(['odrl:archive'])

    draft.actions = ['odrl:index']
    await wrapper.vm.$nextTick()
    expect(host.rule?.['@id']).toBe(existing['@id'])
  })
})
