export interface ContractNegotiationDecision {
  /** Counterpart who has to decide this negotiation decision */
  counterpart: string;
  decision?: string;
  rejection_reason?: string;
}
