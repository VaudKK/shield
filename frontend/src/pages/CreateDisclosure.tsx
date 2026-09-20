import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ShieldCheck, Download, Info } from 'lucide-react'
import { listEvidence } from '@/lib/evidence-api'
import { createDisclosure, type Disclosure } from '@/lib/disclosure-api'
import { ApiError } from '@/lib/api'

export function CreateDisclosure() {
  const navigate = useNavigate()
  const { data: evidenceList, isLoading } = useQuery({
    queryKey: ['evidence'],
    queryFn: listEvidence,
  })

  const [title, setTitle] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const [includeTimeline, setIncludeTimeline] = useState(true)
  const [includeSummary, setIncludeSummary] = useState(true)
  const [includePhotos, setIncludePhotos] = useState(true)

  const [removePhoneNumbers, setRemovePhoneNumbers] = useState(true)
  const [removeEmails, setRemoveEmails] = useState(true)
  const [removeIdNumbers, setRemoveIdNumbers] = useState(true)
  const [removeMetadata, setRemoveMetadata] = useState(true)

  const [result, setResult] = useState<Disclosure | null>(null)

  const createMutation = useMutation({
    mutationFn: () =>
      createDisclosure({
        title: title.trim() || 'Untitled Disclosure Package',
        evidence_ids: Array.from(selected),
        include_timeline: includeTimeline,
        include_summary: includeSummary,
        include_photos: includePhotos,
        remove_phone_numbers: removePhoneNumbers,
        remove_emails: removeEmails,
        remove_id_numbers: removeIdNumbers,
        blur_faces: false,
        remove_metadata: removeMetadata,
      }),
    onSuccess: (disclosure) => setResult(disclosure),
  })

  const toggleEvidence = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (result) {
    return (
      <div className="mx-auto max-w-2xl px-8 py-10">
        <div className="rounded-lg border border-shield-200 bg-white p-8 text-center">
          <ShieldCheck className="mx-auto mb-3 h-8 w-8 text-status-safe" strokeWidth={1.75} />
          <h1 className="text-xl font-semibold text-shield-950">Package created</h1>
          <p className="mt-2 text-sm text-shield-500">
            "{result.title}" is ready. Nothing was sent anywhere — the download link below is
            yours to share on your own terms.
          </p>
          {result.url && (
            <a
              href={result.url}
              target="_blank"
              rel="noreferrer"
              className="mt-6 inline-flex items-center gap-2 rounded-md bg-shield-800 px-4 py-2 text-sm font-medium text-white hover:bg-shield-900"
            >
              <Download className="h-4 w-4" strokeWidth={1.75} />
              Download package
            </a>
          )}
          <div className="mt-6">
            <Link to="/disclosures" className="text-sm font-medium text-shield-700 hover:underline">
              Back to disclosures
            </Link>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-2xl px-8 py-10">
      <button
        type="button"
        onClick={() => navigate('/disclosures')}
        className="mb-6 flex items-center gap-1 text-sm text-shield-500 hover:text-shield-800"
      >
        <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
        Back to disclosures
      </button>

      <h1 className="mb-1 text-xl font-semibold tracking-tight text-shield-950">
        Create a Safe Disclosure Package
      </h1>
      <p className="mb-6 text-sm text-shield-500">
        Choose what to share and how to protect it. Nothing is sent until you download and share
        the package yourself.
      </p>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (selected.size > 0) createMutation.mutate()
        }}
      >
        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <label className="mb-1 block text-sm font-medium text-shield-900">Package title</label>
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g. Incident report — September 2026"
            className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
          />
        </div>

        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 text-sm font-semibold text-shield-900">Select evidence</h2>
          {isLoading && <p className="text-sm text-shield-400">Loading…</p>}
          {!isLoading && evidenceList?.length === 0 && (
            <p className="text-sm text-shield-400">No evidence to include yet.</p>
          )}
          <ul className="max-h-64 space-y-1 overflow-y-auto">
            {evidenceList?.map((e) => (
              <li key={e.id}>
                <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-shield-50">
                  <input
                    type="checkbox"
                    checked={selected.has(e.id)}
                    onChange={() => toggleEvidence(e.id)}
                    className="h-4 w-4 rounded border-shield-300"
                  />
                  <span className="truncate text-shield-800">{e.title}</span>
                  <span className="ml-auto shrink-0 text-xs text-shield-400">{e.status}</span>
                </label>
              </li>
            ))}
          </ul>
        </div>

        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 text-sm font-semibold text-shield-900">What do you want to share?</h2>
          <div className="space-y-2 text-sm">
            <Checkbox label="Evidence timeline" checked={includeTimeline} onChange={setIncludeTimeline} />
            <Checkbox label="Incident summary" checked={includeSummary} onChange={setIncludeSummary} />
            <Checkbox label="Selected evidence files" checked={includePhotos} onChange={setIncludePhotos} />
          </div>
        </div>

        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 text-sm font-semibold text-shield-900">Privacy protection</h2>
          <div className="space-y-2 text-sm">
            <Checkbox label="Remove phone numbers" checked={removePhoneNumbers} onChange={setRemovePhoneNumbers} />
            <Checkbox label="Remove email addresses" checked={removeEmails} onChange={setRemoveEmails} />
            <Checkbox label="Remove ID numbers" checked={removeIdNumbers} onChange={setRemoveIdNumbers} />
            <label className="flex cursor-not-allowed items-center gap-2 text-shield-400">
              <input type="checkbox" disabled className="h-4 w-4 rounded border-shield-300" />
              Blur faces <span className="text-xs">(not available in this version)</span>
            </label>
            <Checkbox
              label="Remove sensitive file metadata"
              checked={removeMetadata}
              onChange={setRemoveMetadata}
            />
          </div>
          <p className="mt-3 flex items-start gap-1.5 text-xs text-shield-400">
            <Info className="mt-0.5 h-3 w-3 shrink-0" strokeWidth={1.75} />
            Protections apply to items that haven't been explicitly excluded from redaction during
            per-evidence review. Rejecting an item there keeps it visible here too.
          </p>
        </div>

        {createMutation.isError && (
          <p className="mb-4 text-sm text-status-rejected">
            {createMutation.error instanceof ApiError
              ? createMutation.error.message
              : 'Could not create the package. Please try again.'}
          </p>
        )}

        <button
          type="submit"
          disabled={selected.size === 0 || createMutation.isPending}
          className="w-full rounded-md bg-shield-800 px-4 py-2.5 text-sm font-medium text-white hover:bg-shield-900 disabled:opacity-50"
        >
          {createMutation.isPending
            ? 'Creating package…'
            : `Create package${selected.size ? ` (${selected.size} item${selected.size === 1 ? '' : 's'})` : ''}`}
        </button>
      </form>
    </div>
  )
}

function Checkbox({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-shield-800">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 rounded border-shield-300"
      />
      {label}
    </label>
  )
}
