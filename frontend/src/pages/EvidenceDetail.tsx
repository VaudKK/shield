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
} from 'lucide-react'
import {
  analyzeEvidence,
  deleteEvidence,
  getEvidence,
  getEvidenceAnalysis,
  getEvidenceAudit,
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
      queryClient.invalidateQueries({ queryKey: ['evidence', id, 'audit'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteEvidence(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['evidence'] })
      navigate('/evidence', { replace: true })
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
  const isImage = originalFile?.mime_type.startsWith('image/')
  const isSensitive = evidence.status === 'sensitive'
  const canShowPreview = isImage && evidence.original_url && (!isSensitive || revealed)

  return (
    <div className="mx-auto max-w-3xl px-8 py-10">
      <button
        type="button"
        onClick={() => navigate('/evidence')}
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

      {analysis && analysis.pii.length > 0 && (
        <div className="mb-6 rounded-lg border border-shield-200 bg-white p-5">
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-shield-900">
            <UserRound className="h-4 w-4 text-shield-500" strokeWidth={1.75} />
            Privacy — {analysis.pii.length} item{analysis.pii.length === 1 ? '' : 's'} detected
          </h2>
          <ul className="space-y-2">
            {analysis.pii.map((p) => (
              <li key={p.id} className="flex items-center justify-between gap-3 text-sm">
                <div className="min-w-0">
                  <span className="font-medium text-shield-900">{p.type}</span>{' '}
                  <span className="text-shield-600">{p.value}</span>
                </div>
                <span className="shrink-0 rounded-full bg-shield-100 px-2 py-0.5 text-xs text-shield-500">
                  {p.detection_method}
                </span>
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
