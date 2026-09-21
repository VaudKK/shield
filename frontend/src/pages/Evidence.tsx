import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { UploadCloud, FileStack, Lock, Sparkles } from 'lucide-react'
import { analyzeEvidence, listEvidence, uploadEvidence, type Evidence as EvidenceRecord } from '@/lib/evidence-api'
import { ApiError } from '@/lib/api'
import { StatusBadge } from '@/components/StatusBadge'
import { cn } from '@/lib/utils'

export function Evidence() {
  const queryClient = useQueryClient()
  const [isDragging, setIsDragging] = useState(false)
  const [pendingTitle, setPendingTitle] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: evidenceList, isLoading } = useQuery({
    queryKey: ['evidence'],
    queryFn: listEvidence,
  })

  const uploadMutation = useMutation({
    mutationFn: ({ file, title }: { file: File; title: string }) => uploadEvidence(file, title),
    onSuccess: (uploaded) => {
      queryClient.invalidateQueries({ queryKey: ['evidence'] })
      setPendingTitle('')

      // "Safe" evidence continues to OCR/AI automatically — the user
      // shouldn't have to know that opening the detail page is what
      // starts it. review/sensitive evidence still waits for an explicit
      // "Continue Processing" click (see EvidenceDetail's processing
      // gate), so it's deliberately not auto-analyzed here.
      if (uploaded.status === 'safe') {
        analyzeEvidence(uploaded.id)
          .catch(() => {
            // Best-effort: if this fails, the detail page's own
            // auto-analyze effect (or a manual re-analyze) will retry it.
          })
          .finally(() => {
            queryClient.invalidateQueries({ queryKey: ['evidence'] })
          })
      }
    },
  })

  const handleFiles = (files: FileList | null) => {
    const file = files?.[0]
    if (!file) return
    uploadMutation.mutate({ file, title: pendingTitle })
  }

  return (
    <div className="mx-auto max-w-5xl px-8 py-10">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-shield-950">Evidence</h1>
        <p className="mt-1 text-sm text-shield-500">
          Upload, organize, and review preserved evidence.
        </p>
      </header>

      <div className="mb-4 flex items-center gap-2 rounded-md border border-shield-200 bg-white px-4 py-3 text-sm text-shield-600">
        <Lock className="h-4 w-4 shrink-0 text-shield-500" strokeWidth={1.75} />
        Your original files are preserved without compression.
      </div>

      <input
        type="text"
        placeholder="Optional label (e.g. Text messages, 12 Sep)"
        value={pendingTitle}
        onChange={(e) => setPendingTitle(e.target.value)}
        className="mb-3 w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
      />

      <div
        onDragOver={(e) => {
          e.preventDefault()
          setIsDragging(true)
        }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setIsDragging(false)
          handleFiles(e.dataTransfer.files)
        }}
        onClick={() => fileInputRef.current?.click()}
        className={cn(
          'flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed px-6 py-12 text-center transition-colors',
          isDragging ? 'border-shield-500 bg-shield-100' : 'border-shield-200 bg-white hover:border-shield-300',
        )}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp,application/pdf"
          className="hidden"
          onChange={(e) => handleFiles(e.target.files)}
        />
        <UploadCloud className="h-8 w-8 text-shield-400" strokeWidth={1.5} />
        {uploadMutation.isPending ? (
          <p className="text-sm font-medium text-shield-700">Uploading…</p>
        ) : (
          <>
            <p className="text-sm font-medium text-shield-700">Drop evidence here, or click to browse</p>
            <p className="text-xs text-shield-400">Images and PDFs. Originals are never compressed.</p>
          </>
        )}
      </div>

      {uploadMutation.isError && (
        <p className="mt-3 text-sm text-status-rejected">
          {uploadMutation.error instanceof ApiError
            ? uploadMutation.error.message
            : 'Upload failed. Please try again.'}
        </p>
      )}

      <div className="mt-10">
        {isLoading && <p className="text-sm text-shield-400">Loading…</p>}

        {!isLoading && evidenceList?.length === 0 && (
          <div className="flex h-40 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-shield-200 text-shield-400">
            <FileStack className="h-6 w-6" strokeWidth={1.5} />
            <p className="text-sm">No evidence yet.</p>
          </div>
        )}

        <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {evidenceList?.map((e) => (
            <EvidenceCard key={e.id} evidence={e} />
          ))}
        </ul>
      </div>
    </div>
  )
}

function EvidenceCard({ evidence }: { evidence: EvidenceRecord }) {
  const needsWarning = evidence.status === 'review' || evidence.status === 'sensitive'
  const analyzing = !evidence.analyzed && evidence.status === 'safe'

  return (
    <li>
      <Link
        to={`/dashboard/evidence/${evidence.id}`}
        className="block rounded-lg border border-shield-200 bg-white p-4 transition-colors hover:border-shield-400"
      >
        <div className="mb-2 flex items-start justify-between gap-2">
          <p className="truncate text-sm font-medium text-shield-950">{evidence.title}</p>
          <StatusBadge status={evidence.status} />
        </div>
        <p className="mb-2 text-xs text-shield-400">
          {new Date(evidence.created_at).toLocaleString()}
        </p>
        {!evidence.analyzed && (
          <p className="flex items-center gap-1 text-xs font-medium text-status-review">
            <Sparkles className="h-3 w-3 shrink-0" strokeWidth={1.75} />
            {analyzing
              ? 'Analyzing…'
              : needsWarning
                ? 'Click to review and continue processing'
                : 'Not analyzed — click to analyze'}
          </p>
        )}
      </Link>
    </li>
  )
}
