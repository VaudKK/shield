import { apiFetch } from '@/lib/api'

export interface Disclosure {
  id: string
  title: string
  include_timeline: boolean
  include_summary: boolean
  include_photos: boolean
  remove_phone_numbers: boolean
  remove_emails: boolean
  remove_id_numbers: boolean
  blur_faces: boolean
  remove_metadata: boolean
  created_at: string
  evidence_ids?: string[]
  url?: string
}

export interface CreateDisclosureInput {
  title: string
  evidence_ids: string[]
  include_timeline: boolean
  include_summary: boolean
  include_photos: boolean
  remove_phone_numbers: boolean
  remove_emails: boolean
  remove_id_numbers: boolean
  blur_faces: boolean
  remove_metadata: boolean
}

export function listDisclosures() {
  return apiFetch<Disclosure[]>('/disclosures/')
}

export function getDisclosure(id: string) {
  return apiFetch<Disclosure>(`/disclosures/${id}`)
}

export function createDisclosure(input: CreateDisclosureInput) {
  return apiFetch<Disclosure>('/disclosures/', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getDisclosureDownloadURL(id: string) {
  return apiFetch<{ url: string }>(`/disclosures/${id}/download`)
}
