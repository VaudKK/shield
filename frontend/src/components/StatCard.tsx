import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface StatCardProps {
  label: string
  value: number | string
  icon: LucideIcon
  tone?: 'default' | 'warning' | 'success'
}

const TONE_STYLES: Record<NonNullable<StatCardProps['tone']>, string> = {
  default: 'text-shield-700 bg-shield-100',
  warning: 'text-status-review bg-amber-50',
  success: 'text-status-safe bg-emerald-50',
}

export function StatCard({ label, value, icon: Icon, tone = 'default' }: StatCardProps) {
  return (
    <div className="flex items-center gap-4 rounded-lg border border-shield-200 bg-white p-5">
      <div className={cn('flex h-10 w-10 items-center justify-center rounded-md', TONE_STYLES[tone])}>
        <Icon className="h-5 w-5" strokeWidth={1.75} />
      </div>
      <div>
        <p className="text-2xl font-semibold text-shield-950">{value}</p>
        <p className="text-sm text-shield-500">{label}</p>
      </div>
    </div>
  )
}
