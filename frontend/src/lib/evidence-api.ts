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
  url?: string
}

export interface Evidence {
  id: string
  title: string
  status: EvidenceStatus
  analyzed: boolean
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

export interface Gap {
  description: string
  confidence: 'low' | 'medium' | 'high'
}

export interface TimelineEvent {
  id: string
  date: string
  description: string
  source: 'ai' | 'user'
}

export interface PIIDetection {
  id: string
  type: string
  value: string
  location: string
  detection_method: 'regex' | 'ai'
  status: 'detected' | 'accepted' | 'rejected'
}

export interface EvidenceAnalysis {
  analysis: {
    summary: string
    gaps: Gap[]
    model: string
    analyzed_at: string
  }
  timeline: TimelineEvent[]
  pii: PIIDetection[]
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

export function analyzeEvidence(id: string) {
  return apiFetch<EvidenceAnalysis>(`/evidence/${id}/analyze`, { method: 'POST' })
}

export function getEvidenceAnalysis(id: string) {
  return apiFetch<EvidenceAnalysis>(`/evidence/${id}/analysis`)
}

export function getEvidenceTimeline(id: string) {
  return apiFetch<TimelineEvent[]>(`/evidence/${id}/timeline`)
}

export function getEvidencePII(id: string) {
  return apiFetch<PIIDetection[]>(`/evidence/${id}/pii`)
}

export function reviewPII(evidenceId: string, piiId: string, status: 'accepted' | 'rejected') {
  return apiFetch<PIIDetection>(`/evidence/${evidenceId}/pii/${piiId}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export function addManualPII(evidenceId: string, input: { type: string; value: string; location?: string }) {
  return apiFetch<PIIDetection>(`/evidence/${evidenceId}/pii`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export interface RedactionResult {
  file: EvidenceFile
  redactions: { pii_detection_id: string; applied: boolean }[]
}

export function redactEvidence(id: string) {
  return apiFetch<RedactionResult>(`/evidence/${id}/redact`, { method: 'POST' })
}

export interface ModerationBoundingBox {
  x: number
  y: number
  width: number
  height: number
}

export interface ModerationResult {
  status: 'safe' | 'review' | 'sensitive'
  confidence: number
  labels: string[]
  bounding_boxes: ModerationBoundingBox[]
  created_at: string
}

export function getEvidenceModeration(id: string) {
  return apiFetch<ModerationResult>(`/evidence/${id}/moderation`)
}
