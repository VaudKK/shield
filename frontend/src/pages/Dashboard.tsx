import { useQuery } from '@tanstack/react-query'
import { FileStack, ShieldAlert, ShieldCheck, Send, Lock, AlertTriangle } from 'lucide-react'
import { StatCard } from '@/components/StatCard'
import { getHealth } from '@/lib/api'
import { listEvidence } from '@/lib/evidence-api'
import { listDisclosures } from '@/lib/disclosure-api'

export function Dashboard() {
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: getHealth,
  })

  const {
    data: evidenceList,
    isLoading: evidenceLoading,
    isError: evidenceErrored,
  } = useQuery({
    queryKey: ['evidence'],
    queryFn: listEvidence,
  })

  const {
    data: disclosures,
    isLoading: disclosuresLoading,
    isError: disclosuresErrored,
  } = useQuery({
    queryKey: ['disclosures'],
    queryFn: listDisclosures,
  })

  const reviewCount = evidenceList?.filter((e) => e.status === 'review').length ?? 0
  const safeCount = evidenceList?.filter((e) => e.status === 'safe').length ?? 0
  const statsLoading = evidenceLoading || disclosuresLoading
  const statsErrored = evidenceErrored || disclosuresErrored

  return (
    <div className="mx-auto max-w-5xl px-8 py-10">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-shield-950">
          Dashboard
        </h1>
        <p className="mt-1 text-sm text-shield-500">
          An overview of your preserved evidence and disclosure activity.
        </p>
      </header>

      <div className="mb-8 flex items-center gap-2 rounded-md border border-shield-200 bg-white px-4 py-3 text-sm text-shield-600">
        <Lock className="h-4 w-4 shrink-0 text-shield-500" strokeWidth={1.75} />
        Original evidence is preserved privately. You control what gets disclosed.
      </div>

      {statsErrored && (
        <div className="mb-4 flex items-center gap-2 rounded-md border border-status-rejected/30 bg-red-50 px-4 py-3 text-sm text-status-rejected">
          <AlertTriangle className="h-4 w-4 shrink-0" strokeWidth={1.75} />
          Some stats couldn't be loaded. Try refreshing the page.
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Evidence" value={statsLoading ? '—' : (evidenceList?.length ?? 0)} icon={FileStack} />
        <StatCard
          label="Needs Review"
          value={statsLoading ? '—' : reviewCount}
          icon={ShieldAlert}
          tone="warning"
        />
        <StatCard label="Protected" value={statsLoading ? '—' : safeCount} icon={ShieldCheck} tone="success" />
        <StatCard
          label="Disclosure Packages"
          value={statsLoading ? '—' : (disclosures?.length ?? 0)}
          icon={Send}
        />
      </div>

      <div className="mt-8 rounded-lg border border-shield-200 bg-white p-6">
        <h2 className="mb-2 text-sm font-semibold text-shield-900">System status</h2>
        <dl className="grid grid-cols-2 gap-y-2 text-sm sm:grid-cols-4">
          <dt className="text-shield-500">API</dt>
          <dd className="text-shield-900">{health?.status ?? 'checking…'}</dd>
          <dt className="text-shield-500">Database</dt>
          <dd className="text-shield-900">{health?.database ?? 'checking…'}</dd>
          <dt className="text-shield-500">Version</dt>
          <dd className="text-shield-900">{health?.version ?? '—'}</dd>
        </dl>
      </div>
    </div>
  )
}
