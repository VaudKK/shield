import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Send, Plus, Download } from 'lucide-react'
import { getDisclosureDownloadURL, listDisclosures } from '@/lib/disclosure-api'
import { ApiError } from '@/lib/api'

export function Disclosures() {
  const { data: disclosures, isLoading } = useQuery({
    queryKey: ['disclosures'],
    queryFn: listDisclosures,
  })

  return (
    <div className="mx-auto max-w-4xl px-8 py-10">
      <header className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-shield-950">Disclosures</h1>
          <p className="mt-1 text-sm text-shield-500">
            Safe Disclosure Packages you've created — you decide exactly what leaves Shield.
          </p>
        </div>
        <Link
          to="/dashboard/disclosures/new"
          className="flex shrink-0 items-center gap-2 rounded-md bg-shield-800 px-3 py-2 text-sm font-medium text-white hover:bg-shield-900"
        >
          <Plus className="h-4 w-4" strokeWidth={1.75} />
          New package
        </Link>
      </header>

      {isLoading && <p className="text-sm text-shield-400">Loading…</p>}

      {!isLoading && disclosures?.length === 0 && (
        <div className="flex h-48 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-shield-200 text-shield-400">
          <Send className="h-6 w-6" strokeWidth={1.5} />
          <p className="text-sm">No disclosure packages yet.</p>
        </div>
      )}

      <ul className="space-y-3">
        {disclosures?.map((d) => (
          <DisclosureRow key={d.id} id={d.id} title={d.title} createdAt={d.created_at} />
        ))}
      </ul>
    </div>
  )
}

function DisclosureRow({ id, title, createdAt }: { id: string; title: string; createdAt: string }) {
  const [error, setError] = useState<string | null>(null)

  const downloadMutation = useMutation({
    mutationFn: () => getDisclosureDownloadURL(id),
    onSuccess: (result) => {
      window.open(result.url, '_blank', 'noopener,noreferrer')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Could not generate a download link.')
    },
  })

  return (
    <li className="flex items-center justify-between gap-3 rounded-lg border border-shield-200 bg-white p-4">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-shield-950">{title}</p>
        <p className="text-xs text-shield-400">{new Date(createdAt).toLocaleString()}</p>
        {error && <p className="mt-1 text-xs text-status-rejected">{error}</p>}
      </div>
      <button
        type="button"
        onClick={() => {
          setError(null)
          downloadMutation.mutate()
        }}
        disabled={downloadMutation.isPending}
        className="flex shrink-0 items-center gap-2 rounded-md border border-shield-300 px-3 py-1.5 text-sm font-medium text-shield-800 hover:bg-shield-50 disabled:opacity-60"
      >
        <Download className="h-4 w-4" strokeWidth={1.75} />
        {downloadMutation.isPending ? 'Preparing…' : 'Download'}
      </button>
    </li>
  )
}
