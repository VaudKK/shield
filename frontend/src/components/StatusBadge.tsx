import { cn } from '@/lib/utils'
import type { EvidenceStatus } from '@/lib/evidence-api'

const STATUS_LABELS: Record<EvidenceStatus, string> = {
  quarantined: 'Quarantined',
  safe: 'Safe',
  review: 'Needs review',
  sensitive: 'Sensitive',
  rejected: 'Rejected',
}

const STATUS_STYLES: Record<EvidenceStatus, string> = {
  quarantined: 'bg-shield-100 text-shield-700',
  safe: 'bg-emerald-50 text-status-safe',
  review: 'bg-amber-50 text-status-review',
  sensitive: 'bg-orange-50 text-status-sensitive',
  rejected: 'bg-red-50 text-status-rejected',
}

export function StatusBadge({ status }: { status: EvidenceStatus }) {
  return (
    <span
      className={cn(
        'shrink-0 rounded-full px-2 py-0.5 text-xs font-medium',
        STATUS_STYLES[status],
      )}
    >
      {STATUS_LABELS[status]}
    </span>
  )
}
