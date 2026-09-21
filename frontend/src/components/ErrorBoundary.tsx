import { Component, type ErrorInfo, type ReactNode } from 'react'
import { ShieldAlert } from 'lucide-react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-shield-50 px-4 text-center">
          <ShieldAlert className="h-8 w-8 text-status-rejected" strokeWidth={1.75} />
          <h1 className="text-lg font-semibold text-shield-950">Something went wrong</h1>
          <p className="max-w-sm text-sm text-shield-500">
            An unexpected error occurred. Your evidence and vault data are unaffected — try
            reloading the page.
          </p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="mt-2 rounded-md bg-shield-800 px-4 py-2 text-sm font-medium text-white hover:bg-shield-900"
          >
            Reload
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
