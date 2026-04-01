import type { NegotiationDecision } from "@/types/negotiation-decision";

export interface ContractNegotiationDecision {
  /** Counterpart who has to decide this negotiation decision */
  counterpart: string;
  decision?: NegotiationDecision;
  rejection_reason?: string;
}
