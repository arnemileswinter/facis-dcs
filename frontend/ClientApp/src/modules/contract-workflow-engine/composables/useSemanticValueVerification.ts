import type { SemanticConditionValue } from "@/models/contract-data";
import type { SemanticCondition } from "@/modules/template-repository/models/contract-templace";

export interface VerificationResult {
  isValid: boolean
  errors: {
    blockId: string
    subBlockId?: string
    conditionId: string
    parameterName: string
    message: string
  }[]
}

export function useSemanticValueVerification() {

  function validateParameterType(value: string | number, type: string): boolean {
    switch (type) {
      case 'string':
        return typeof value === 'string'
      case 'integer':
        return typeof value === 'number' && Number.isInteger(value)
      case 'decimal':
        return typeof value === 'number' && !Number.isNaN(value)
      case 'date':
        return typeof value === 'string' && !isNaN(Date.parse(value))
      default:
        return false
    }
  }

  function verifySemanticValue(semanticConditions: SemanticCondition[], semanticConditionValues: SemanticConditionValue[]): VerificationResult {
    const errors: VerificationResult['errors'] = []
    let isValid = false

    semanticConditionValues.forEach((value) => {
      const fieldName = value.parameterName || 'this field'
      const condition = semanticConditions.find((cond) => cond.conditionId === value.conditionId)
      // check if the condition exists, if not, it's an error
      if (!condition) {
        errors.push({
          blockId: value.blockId,
          subBlockId: value.subBlockId,
          conditionId: value.conditionId,
          parameterName: value.parameterName,
          message: 'Semantic rule not found.',
        })
        return
      }
      // check if the parameter exists in the condition, if not, it's an error
      const parameter = condition.parameters.find((param) => param.parameterName === value.parameterName)
      if (!parameter) {
        errors.push({
          blockId: value.blockId,
          subBlockId: value.subBlockId,
          conditionId: value.conditionId,
          parameterName: value.parameterName,
          message: `"${fieldName}" is not defined in the selected semantic rule.`,
        })
        return
      }
      // check if the parameter value is provided, if the parameter is required, it's an error if not provided
      if (parameter.isRequired && (value.parameterValue === undefined || value.parameterValue === null)) {
        errors.push({
          blockId: value.blockId,
          subBlockId: value.subBlockId,
          conditionId: value.conditionId,
          parameterName: value.parameterName,
          message: `"${fieldName}" is required but has no value.`,
        })
        return
      }
      // check if the parameter value type matches the parameter type, if not, it's an error
      if (value.parameterValue !== undefined && value.parameterValue !== null) {
        const isTypeValid = validateParameterType(value.parameterValue, parameter.type)
        if (!isTypeValid) {
          errors.push({
            blockId: value.blockId,
            subBlockId: value.subBlockId,
            conditionId: value.conditionId,
            parameterName: value.parameterName,
            message: `"${fieldName}" has an invalid value type. Expected ${parameter.type}.`,
          })
          return
        }
      }
    })
    if (errors.length === 0) {
      isValid = true
    }
    return { isValid, errors }
  }

  return { verifySemanticValue }
}