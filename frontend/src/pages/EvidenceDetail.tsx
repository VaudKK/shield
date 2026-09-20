import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  ShieldCheck,
  ShieldAlert,
  Trash2,
  ExternalLink,
  Eye,
  Sparkles,
  History,
  UserRound,
  Check,
  X,
  Plus,
  ShieldOff,
} from 'lucide-react'
import {
  addManualPII,
  analyzeEvidence,
  deleteEvidence,
  getEvidence,
  getEvidenceAnalysis,
  getEvidenceAudit,
  getEvidencePII,
  redactEvidence,
  reviewPII,
  type PIIDetection,
} from '@/lib/evidence-api'
import { ApiError } from '@/lib/api'
import { StatusBadge } from '@/components/StatusBadge'

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB']
  let value = bytes / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(1)} ${units[i]}`
}

export function EvidenceDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [revealed, setRevealed] = useState(false)

  const { data: evidence, isLoading } = useQuery({
    queryKey: ['evidence', id],
    queryFn: () => getEvidence(id!),
    enabled: !!id,
  })

  const { data: auditEvents } = useQuery({
    queryKey: ['evidence', id, 'audit'],
    queryFn: () => getEvidenceAudit(id!),
    enabled: !!id,
  })

  const {
    data: analysis,
    isLoading: analysisLoading,
    error: analysisError,
  } = useQuery({
    queryKey: ['evidence', id, 'analysis'],
    queryFn: () => getEvidenceAnalysis(id!),
    enabled: !!id,
    retry: false,
  })

  const analyzeMutation = useMutation({
    mutationFn: () => analyzeEvidence(id!),
    onSuccess: (result) => {
      queryClient.setQueryData(['evidence', id, 'analysis'], result)
      queryClient.setQueryData(['evidence', id, 'pii'], result.pii)
      queryClient.invalidateQueries({ queryKey: ['evidence', id, 'audit'] })
    },
  })

  const { data: piiItems } = useQuery({
    queryKey: ['evidence', id, 'pii'],
    queryFn: () => getEvidencePII(id!),
    enabled: !!id,
  })

  const [manualType, setManualType] = useState('')
  const [manualValue, setManualValue] = useState('')

  const reviewMutation = useMutation({
    mutationFn: ({ piiId, status }: { piiId: string; status: 'accepted' | 'rejected' }) =>
      reviewPII(id!, piiId, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['evidence', id, 'pii'] }),
  })

  const addManualMutation = useMutation({
    mutationFn: () => addManualPII(id!, { type: manualType, value: manualValue }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['evidence', id, 'pii'] })
      setManualType('')
      setManualValue('')
    },
  })

  const redactMutation = useMutation({
    mutationFn: () => redactEvidence(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['evidence', id] })
      queryClient.invalidateQueries({ queryKey: ['evidence', id, 'audit'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteEvidence(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['evidence'] })
      navigate('/dashboard/evidence', { replace: true })
    },
  })

  const notAnalyzedYet = analysisError instanceof ApiError && analysisError.code === 'ANALYSIS_NOT_FOUND'

  if (isLoading || !evidence) {
    return (
      <div className="mx-auto max-w-3xl px-8 py-10">
        <p className="text-sm text-shield-400">Loading…</p>
      </div>
    )
  }

  const originalFile = evidence.files?.find((f) => f.kind === 'original')
  const redactedFiles = evidence.files?.filter((f) => f.kind === 'redacted') ?? []
  const isImage = originalFile?.mime_type.startsWith('image/')
  const isSensitive = evidence.status === 'sensitive'
  const canShowPreview = isImage && evidence.original_url && (!isSensitive || revealed)

  return (
    <div className="mx-auto max-w-3xl px-8 py-10">
      <button
        type="button"
        onClick={() => navigate('/dashboard/evidence')}
        className="mb-6 flex items-center gap-1 text-sm text-shield-500 hover:text-shield-800"
      >
        <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
        Back to evidence
      </button>

      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-shield-950">{evidence.title}</h1>
          <p className="mt-1 text-sm text-shield-500">
            Uploaded {new Date(evidence.created_at).toLocaleString()}
          </p>
        </div>
        <StatusBadge status={evidence.status} />
      </div>

      {isSensitive && !revealed && (
        <div className="mb-6 flex flex-col items-center gap-3 rounded-lg border border-status-sensitive/30 bg-orange-50 px-6 py-10 text-center">
          <ShieldAlert className="h-6 w-6 text-status-sensitive" strokeWidth={1.75} />
          <p className="text-sm font-medium text-shield-900">
            This file was flagged as potentially sensitive.
          </p>
          <p className="max-w-sm text-xs text-shield-500">
            Sensitive content can itself be legitimate evidence, so Shield keeps it private and lets
            you decide whether to view it — it is never deleted automatically.
          </p>
          {isImage && evidence.original_url && (
            <button
              type="button"
              onClick={() => setRevealed(true)}
              className="mt-2 flex items-center gap-2 rounded-md border border-shield-300 bg-white px-3 py-2 text-sm font-medium text-shield-800 hover:bg-shield-50"
            >
              <Eye className="h-4 w-4" strokeWidth={1.75} />
              Reveal image
            </button>
          )}
        </div>
      )}

      {canShowPreview && (
        <div className="mb-6 overflow-hidden rounded-lg border border-shield-200 bg-shield-50">
          <img src={evidence.original_url} alt={evidence.title} className="max-h-96 w-full object-contain" />
        </div>
      )}

      <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
        <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-shield-900">
          <ShieldCheck className="h-4 w-4 text-shield-500" strokeWidth={1.75} />
          Integrity
        </h2>
        {originalFile && (
          <dl className="grid grid-cols-1 gap-y-2 text-sm sm:grid-cols-[120px_1fr]">
            <dt className="text-shield-500">Filename</dt>
            <dd className="truncate text-shield-900">{originalFile.original_filename}</dd>
            <dt className="text-shield-500">Type</dt>
            <dd className="text-shield-900">{originalFile.mime_type}</dd>
            <dt className="text-shield-500">Size</dt>
            <dd className="text-shield-900">{formatBytes(originalFile.size_bytes)}</dd>
            <dt className="text-shield-500">SHA-256</dt>
            <dd className="break-all font-mono text-xs text-shield-900">{originalFile.sha256}</dd>
          </dl>
        )}
        <p className="mt-3 text-xs text-shield-400">
          This hash shows whether the stored file has changed since upload — it does not prove the
          underlying evidence itself is authentic.
        </p>
        {evidence.original_url && (!isSensitive || revealed) && (
          <a
            href={evidence.original_url}
            target="_blank"
            rel="noreferrer"
            className="mt-3 inline-flex items-center gap-1 text-sm font-medium text-shield-700 hover:underline"
          >
            View original <ExternalLink className="h-3 w-3" strokeWidth={1.75} />
          </a>
        )}
      </div>

      <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-shield-900">
            <Sparkles className="h-4 w-4 text-shield-500" strokeWidth={1.75} />
            AI Summary
          </h2>
          {!analysisLoading && (
            <button
              type="button"
              onClick={() => analyzeMutation.mutate()}
              disabled={analyzeMutation.isPending}
              className="rounded-md border border-shield-300 px-3 py-1.5 text-xs font-medium text-shield-800 hover:bg-shield-50 disabled:opacity-60"
            >
              {analyzeMutation.isPending
                ? 'Analyzing…'
                : analysis
                  ? 'Re-analyze'
                  : 'Analyze evidence'}
            </button>
          )}
        </div>

        {analyzeMutation.isError && (
          <p className="mb-3 text-sm text-status-rejected">
            {analyzeMutation.error instanceof ApiError
              ? analyzeMutation.error.message
              : 'Analysis failed. Please try again.'}
          </p>
        )}

        {analysisLoading && <p className="text-sm text-shield-400">Loading…</p>}

        {!analysisLoading && notAnalyzedYet && !analysis && (
          <p className="text-sm text-shield-400">
            Not analyzed yet. Shield can extract text, generate a plain-language summary and
            timeline, and flag possible personal information — nothing here determines whether an
            incident occurred.
          </p>
        )}

        {analysis && (
          <>
            <p className="text-sm text-shield-800">{analysis.analysis.summary}</p>

            {analysis.analysis.gaps.length > 0 && (
              <div className="mt-4">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-shield-500">
                  Information gaps
                </h3>
                <ul className="space-y-1">
                  {analysis.analysis.gaps.map((g, i) => (
                    <li key={i} className="flex items-start gap-2 text-sm text-shield-700">
                      <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-shield-400" />
                      {g.description}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>

      {analysis && analysis.timeline.length > 0 && (
        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-shield-900">
            <History className="h-4 w-4 text-shield-500" strokeWidth={1.75} />
            Timeline
          </h2>
          <ul className="space-y-3">
            {analysis.timeline.map((t) => (
              <li key={t.id} className="flex gap-3 text-sm">
                <span className="w-24 shrink-0 font-mono text-xs text-shield-500">{t.date}</span>
                <span className="text-shield-800">{t.description}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {piiItems && piiItems.length > 0 && (
        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-shield-900">
            <UserRound className="h-4 w-4 text-shield-500" strokeWidth={1.75} />
            Privacy — {piiItems.length} item{piiItems.length === 1 ? '' : 's'} detected
          </h2>
          <p className="mb-3 text-xs text-shield-400">
            Review each item below. Accepted items are covered when you create a redacted copy;
            rejected items are left as-is.
          </p>
          <ul className="space-y-2">
            {piiItems.map((p) => (
              <PIIRow
                key={p.id}
                item={p}
                onReview={(status) => reviewMutation.mutate({ piiId: p.id, status })}
                pending={reviewMutation.isPending && reviewMutation.variables?.piiId === p.id}
              />
            ))}
          </ul>

          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (manualType.trim() && manualValue.trim()) addManualMutation.mutate()
            }}
            className="mt-4 flex flex-wrap items-end gap-2 border-t border-shield-100 pt-4"
          >
            <div className="flex-1">
              <label className="mb-1 block text-xs text-shield-500">Type</label>
              <input
                type="text"
                value={manualType}
                onChange={(e) => setManualType(e.target.value)}
                placeholder="e.g. name"
                className="w-full rounded-md border border-shield-200 px-2 py-1.5 text-sm outline-none focus:border-shield-500"
              />
            </div>
            <div className="flex-[2]">
              <label className="mb-1 block text-xs text-shield-500">Value</label>
              <input
                type="text"
                value={manualValue}
                onChange={(e) => setManualValue(e.target.value)}
                placeholder="e.g. Jane Doe"
                className="w-full rounded-md border border-shield-200 px-2 py-1.5 text-sm outline-none focus:border-shield-500"
              />
            </div>
            <button
              type="submit"
              disabled={addManualMutation.isPending}
              className="flex items-center gap-1 rounded-md border border-shield-300 px-3 py-1.5 text-sm font-medium text-shield-800 hover:bg-shield-50 disabled:opacity-60"
            >
              <Plus className="h-4 w-4" strokeWidth={1.75} />
              Add
            </button>
          </form>

          <div className="mt-4 border-t border-shield-100 pt-4">
            <button
              type="button"
              onClick={() => redactMutation.mutate()}
              disabled={redactMutation.isPending || !piiItems.some((p) => p.status === 'accepted')}
              className="flex items-center gap-2 rounded-md bg-shield-800 px-3 py-2 text-sm font-medium text-white hover:bg-shield-900 disabled:opacity-50"
            >
              <ShieldOff className="h-4 w-4" strokeWidth={1.75} />
              {redactMutation.isPending ? 'Creating redacted copy…' : 'Create redacted copy'}
            </button>
            {redactMutation.isError && (
              <p className="mt-2 text-sm text-status-rejected">
                {redactMutation.error instanceof ApiError
                  ? redactMutation.error.message
                  : 'Redaction failed. Please try again.'}
              </p>
            )}
            {redactMutation.isSuccess && (
              <p className="mt-2 text-sm text-status-safe">
                Redacted copy created — see it under Redacted copies below.
              </p>
            )}
          </div>
        </div>
      )}

      {redactedFiles.length > 0 && (
        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 text-sm font-semibold text-shield-900">Redacted copies</h2>
          <p className="mb-3 text-xs text-shield-400">
            The original file above is never modified. Each redacted copy is a separate derived
            file.
          </p>
          <ul className="space-y-2">
            {redactedFiles.map((f) => (
              <li key={f.id} className="flex items-center justify-between gap-3 text-sm">
                <div className="min-w-0">
                  <p className="truncate text-shield-900">{f.original_filename}</p>
                  <p className="text-xs text-shield-400">
                    {formatBytes(f.size_bytes)} · {new Date(f.created_at).toLocaleString()}
                  </p>
                </div>
                {f.url && (
                  <a
                    href={f.url}
                    target="_blank"
                    rel="noreferrer"
                    className="flex shrink-0 items-center gap-1 text-sm font-medium text-shield-700 hover:underline"
                  >
                    View <ExternalLink className="h-3 w-3" strokeWidth={1.75} />
                  </a>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
        <h2 className="mb-3 text-sm font-semibold text-shield-900">Activity</h2>
        <ul className="space-y-2">
          {auditEvents?.map((event) => (
            <li key={event.id} className="flex items-center justify-between text-sm">
              <span className="text-shield-700">{formatEventType(event.event_type)}</span>
              <span className="text-xs text-shield-400">{new Date(event.created_at).toLocaleString()}</span>
            </li>
          ))}
          {!auditEvents?.length && <li className="text-sm text-shield-400">No activity recorded yet.</li>}
        </ul>
      </div>

      <button
        type="button"
        onClick={() => {
          if (window.confirm('Remove this evidence? This cannot be undone.')) {
            deleteMutation.mutate()
          }
        }}
        disabled={deleteMutation.isPending}
        className="flex items-center gap-2 rounded-md border border-status-rejected/30 px-3 py-2 text-sm font-medium text-status-rejected transition-colors hover:bg-red-50 disabled:opacity-60"
      >
        <Trash2 className="h-4 w-4" strokeWidth={1.75} />
        {deleteMutation.isPending ? 'Removing…' : 'Remove evidence'}
      </button>
    </div>
  )
}

function formatEventType(eventType: string): string {
  return eventType
    .toLowerCase()
    .split('_')
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join(' ')
}

function PIIRow({
  item,
  onReview,
  pending,
}: {
  item: PIIDetection
  onReview: (status: 'accepted' | 'rejected') => void
  pending: boolean
}) {
  return (
    <li className="flex items-center justify-between gap-3 text-sm">
      <div className="min-w-0">
        <span className="font-medium text-shield-900">{item.type}</span>{' '}
        <span className="text-shield-600">{item.value}</span>
        <span className="ml-2 rounded-full bg-shield-100 px-2 py-0.5 text-xs text-shield-500">
          {item.detection_method}
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-1">
        {item.status === 'accepted' && (
          <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-status-safe">
            Accepted
          </span>
        )}
        {item.status === 'rejected' && (
          <span className="rounded-full bg-shield-100 px-2 py-0.5 text-xs font-medium text-shield-500">
            Rejected
          </span>
        )}
        <button
          type="button"
          onClick={() => onReview('accepted')}
          disabled={pending || item.status === 'accepted'}
          aria-label="Accept"
          className="rounded-md p-1.5 text-status-safe hover:bg-emerald-50 disabled:opacity-30"
        >
          <Check className="h-4 w-4" strokeWidth={1.75} />
        </button>
        <button
          type="button"
          onClick={() => onReview('rejected')}
          disabled={pending || item.status === 'rejected'}
          aria-label="Reject"
          className="rounded-md p-1.5 text-status-rejected hover:bg-red-50 disabled:opacity-30"
        >
          <X className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>
    </li>
  )
}
