import { apiFetch, apiUpload } from '@/lib/api'

export type EvidenceStatus = 'quarantined' | 'safe' | 'review' | 'sensitive' | 'rejected'

export interface EvidenceFile {
  id: string
  kind: string
  original_filename: string
  mime_type: string
  size_bytes: number
  sha256: string
  created_at: string
}

export interface Evidence {
  id: string
  title: string
  status: EvidenceStatus
  created_at: string
  updated_at: string
  files?: EvidenceFile[]
  original_url?: string
}

export interface AuditEvent {
  id: string
  event_type: string
  actor_id?: string
  metadata: Record<string, unknown>
  created_at: string
}

export function listEvidence() {
  return apiFetch<Evidence[]>('/evidence/')
}

export function getEvidence(id: string) {
  return apiFetch<Evidence>(`/evidence/${id}`)
}

export function deleteEvidence(id: string) {
  return apiFetch<void>(`/evidence/${id}`, { method: 'DELETE' })
}

export function getEvidenceAudit(id: string) {
  return apiFetch<AuditEvent[]>(`/audit/${id}`)
}

export function uploadEvidence(file: File, title: string) {
  const formData = new FormData()
  if (title) formData.append('title', title)
  formData.append('file', file)
  return apiUpload<Evidence>('/evidence/', formData)
}
