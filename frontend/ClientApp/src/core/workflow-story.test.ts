import { describe, expect, it } from 'vitest'
import { templateStory } from './workflow-story'

describe('template workflow story', () => {
  it('explains that an approved Component is composed into Contract Templates', () => {
    const story = templateStory('APPROVED', { templateType: 'COMPONENT' })

    expect(story.narrative).toBe(
      'Approved. This Component can be added to Contract Templates but cannot be used to create contracts directly.',
    )
    expect(story.actionHints).toEqual([{ label: 'Open Template Catalogue', routeName: 'template.catalogues.list' }])
  })

  it('keeps the registration guidance for an approved Contract Template', () => {
    const story = templateStory('APPROVED', { templateType: 'CONTRACT_TEMPLATE' })

    expect(story.narrative).toBe(
      'Approved — one step left: a Template Manager registers it in the Template Catalogue; only registered templates can be used to create contracts.',
    )
    expect(story.actionHints).toEqual([{ label: 'Open Template Catalogue', routeName: 'template.catalogues.list' }])
  })
})
