import { compactOdrlIdentifier } from '@template-repository/utils/odrl-vocabulary'
import { ONTOLOGY_DOMAIN_FIELDS } from '@/modules/template-repository/utils/ontology-domain-fields'
import type { SemanticValueConstraint } from '@/modules/template-repository/models/contract-template'

export function resolveAllowedValues(constraint?: SemanticValueConstraint): readonly string[] {
  if (!constraint) return []
  if (constraint.allowedValues?.length) return constraint.allowedValues

  const ref = normalizeAllowedValuesRef(constraint.allowedValuesRef)
  if (!ref) return []

  return (
    ONTOLOGY_DOMAIN_FIELDS.find((field) => {
      const fieldConstraint = field.valueConstraint
      return (
        normalizeAllowedValuesRef(fieldConstraint?.allowedValuesRef) === ref && !!fieldConstraint?.allowedValues?.length
      )
    })?.valueConstraint?.allowedValues ?? []
  )
}

export function resolveValueConstraintOptions(
  constraint?: SemanticValueConstraint,
): SemanticValueConstraint['valueOptions'] {
  if (!constraint) return []
  if (constraint.valueOptions?.length) return constraint.valueOptions

  const ref = normalizeAllowedValuesRef(constraint.allowedValuesRef)
  if (!ref) return []

  return (
    ONTOLOGY_DOMAIN_FIELDS.find((field) => {
      const fieldConstraint = field.valueConstraint
      return (
        normalizeAllowedValuesRef(fieldConstraint?.allowedValuesRef) === ref && !!fieldConstraint?.valueOptions?.length
      )
    })?.valueConstraint?.valueOptions ?? []
  )
}

export function resolveConstraintForLeftOperand(leftOperand: string): SemanticValueConstraint | undefined {
  const fieldConstraint = ONTOLOGY_DOMAIN_FIELDS.find((field) => field.ontologyId === leftOperand)?.valueConstraint
  if (fieldConstraint) return fieldConstraint
  const normalizedLeftOperand = compactOdrlIdentifier(leftOperand)
  return ONTOLOGY_DOMAIN_FIELDS.find((field) =>
    field.valueConstraint?.odrlLeftOperands?.some(
      (operand) => compactOdrlIdentifier(operand) === normalizedLeftOperand,
    ),
  )?.valueConstraint
}

function normalizeAllowedValuesRef(value?: string) {
  return value?.trim().replace(/\s+/g, ' ').toLowerCase() ?? ''
}
