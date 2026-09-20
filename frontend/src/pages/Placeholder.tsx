interface PlaceholderProps {
  title: string
  description: string
}

export function Placeholder({ title, description }: PlaceholderProps) {
  return (
    <div className="mx-auto max-w-5xl px-8 py-10">
      <h1 className="text-2xl font-semibold tracking-tight text-shield-950">{title}</h1>
      <p className="mt-1 text-sm text-shield-500">{description}</p>
      <div className="mt-8 flex h-48 items-center justify-center rounded-lg border border-dashed border-shield-200 text-sm text-shield-400">
        Coming in a later phase
      </div>
    </div>
  )
}
