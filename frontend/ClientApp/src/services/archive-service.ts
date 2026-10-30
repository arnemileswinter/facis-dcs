import http from '@/api/http'
import type { Contract } from '@/models/contract/contract'

/** An archived contract entry as returned by /archive/retrieve. */
export interface ArchivedContract {
  did: string
  contract_version: number
  state: string
  name?: string
  description?: string
  created_by: string
  created_at: string
  updated_at: string
  exp_date?: string
  archive_summary?: string
  archive_tags?: string[]
}

/** The stored annotation of an archive entry (DCS-FR-CSA-11). */
export interface ArchiveAnnotation {
  did: string
  summary: string
  tags?: string[]
}

/** The erasure-handshake state of one counterparty instance (DCS-NFR-COMP-03). */
export interface ArchiveErasurePeerStatus {
  peer_did: string
  status: 'pending' | 'confirmed'
  requested_at: string
  confirmed_at?: string
  retry_count: number
  last_tried_at?: string
}

/** The erasure state of a contract's content-encryption keys (DCS-NFR-SEC-13). */
export interface ArchiveErasureStatus {
  did: string
  local_status: 'live' | 'shredded'
  shredded_at?: string
  shredded_by?: string
  shred_reason?: string
  peers: ArchiveErasurePeerStatus[]
}

/** The record /archive/store wrote (DCS-IR-CSA-02). */
export interface ArchiveStoreResult {
  did: string
  kind: 'contract' | 'evidence'
  contract_version?: number
  evidence_id?: string
  content_hash: string
}

/** An evidence document stored for an archived contract. `content` is base64 and absent once erased. */
export interface ArchiveEvidenceItem {
  id: string
  did: string
  name: string
  media_type: string
  description?: string
  content_hash: string
  size_bytes: number
  stored_by: string
  stored_at: string
  content?: string
}

/** One archive compliance rule result (DCS-FR-CSA-19). */
export interface ArchiveComplianceCheck {
  rule: 'retention' | 'signature_policy' | 'metadata_completeness' | 'evidence_completeness'
  passed: boolean
  reason?: string
}

/** The compliance verdict of one live archive entry (DCS-FR-CSA-19). */
export interface ArchiveComplianceItem {
  did: string
  contract_version: number
  name?: string
  flagged: boolean
  checks: ArchiveComplianceCheck[]
}

/** The filters of /archive/search (DCS-FR-CSA-10, DCS-FR-CSA-13, DCS-FR-CSA-22). */
export interface ArchiveSearchQuery {
  did?: string
  state?: string
  name?: string
  description?: string
  contract_data?: string
  tag?: string
  party?: string
  valid_from?: string
  valid_until?: string
  contract_version?: number
}

/** The /archive/search filters under the name the archive views use. */
export type ArchiveSearch = ArchiveSearchQuery

export const archiveService = {
  async detail(did: string, contract_version: number): Promise<Contract> {
    return http.get<Contract>('/archive/detail', { params: { did, contract_version } }).then((res) => res.data)
  },
  async retrieve(): Promise<ArchivedContract[]> {
    return http.get<{ contracts?: ArchivedContract[] }>('/archive/retrieve').then((res) => res.data.contracts ?? [])
  },

  async search(query: ArchiveSearchQuery): Promise<ArchivedContract[]> {
    return http.get<ArchivedContract[]>('/archive/search', { params: query }).then((res) => res.data ?? [])
  },

  /** Archives the current version of a SIGNED, TERMINATED or EXPIRED contract (DCS-IR-CSA-02). */
  async storeContract(did: string): Promise<ArchiveStoreResult> {
    return http.post<ArchiveStoreResult>('/archive/store', { kind: 'contract', did }).then((res) => res.data)
  },

  /** Stores an evidence document for an archived contract; `content` is base64 (DCS-IR-CSA-02). */
  async storeEvidence(
    did: string,
    evidence: { name: string; media_type: string; content: string; description?: string },
  ): Promise<ArchiveStoreResult> {
    return http
      .post<ArchiveStoreResult>('/archive/store', { kind: 'evidence', did, ...evidence })
      .then((res) => res.data)
  },

  async evidence(did: string): Promise<ArchiveEvidenceItem[]> {
    return http.get<ArchiveEvidenceItem[]>('/archive/evidence', { params: { did } }).then((res) => res.data ?? [])
  },

  async compliance(): Promise<ArchiveComplianceItem[]> {
    return http.get<ArchiveComplianceItem[]>('/archive/compliance').then((res) => res.data ?? [])
  },

  async erasureStatus(did: string): Promise<ArchiveErasureStatus> {
    return http.get<ArchiveErasureStatus>('/archive/erasure-status', { params: { did } }).then((res) => res.data)
  },

  /**
   * Sets the entry's summary and tag set (DCS-FR-CSA-11). Omitting the summary
   * has the backend generate one from the archived contract's metadata; tags
   * replace the stored set. Only the annotation is mutable.
   */
  async annotate(did: string, summary?: string, tags?: string[]): Promise<ArchiveAnnotation> {
    return http.post<ArchiveAnnotation>('/archive/annotate', { did, summary, tags }).then((res) => res.data)
  },

  /**
   * Soft-deletes the archive entry and destroys the contract's content
   * encryption keys on both instances; the justification is recorded as the
   * shred reason (DCS-FR-CSA-17, DCS-NFR-SEC-13).
   */
  async delete(did: string, justification: string): Promise<number> {
    return http.delete<number>('/archive/delete', { params: { did, justification } }).then((res) => res.data)
  },
}
