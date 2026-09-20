import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useLogin } from '@/hooks/useAuth'
import { ApiError } from '@/lib/api'

const schema = z.object({
  email: z.string().email('Enter a valid email address.'),
  password: z.string().min(1, 'Password is required.'),
})

type FormValues = z.infer<typeof schema>

export function Login() {
  const navigate = useNavigate()
  const loginMutation = useLogin()
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = (values: FormValues) => {
    loginMutation.mutate(values, {
      onSuccess: () => navigate('/', { replace: true }),
    })
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <ShieldCheck className="h-8 w-8 text-shield-700" strokeWidth={1.75} />
          <h1 className="text-xl font-semibold tracking-tight text-shield-950">Sign in to Shield</h1>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="rounded-lg border border-shield-200 bg-white p-6">
          <div className="mb-4">
            <label htmlFor="email" className="mb-1 block text-sm font-medium text-shield-900">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
              {...register('email')}
            />
            {errors.email && <p className="mt-1 text-xs text-status-rejected">{errors.email.message}</p>}
          </div>

          <div className="mb-4">
            <label htmlFor="password" className="mb-1 block text-sm font-medium text-shield-900">
              Password
            </label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
              {...register('password')}
            />
            {errors.password && <p className="mt-1 text-xs text-status-rejected">{errors.password.message}</p>}
          </div>

          {loginMutation.isError && (
            <p className="mb-4 text-sm text-status-rejected">
              {loginMutation.error instanceof ApiError
                ? loginMutation.error.message
                : 'Something went wrong. Please try again.'}
            </p>
          )}

          <button
            type="submit"
            disabled={loginMutation.isPending}
            className="w-full rounded-md bg-shield-800 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-shield-900 disabled:opacity-60"
          >
            {loginMutation.isPending ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <p className="mt-4 text-center text-sm text-shield-500">
          Don't have an account?{' '}
          <Link to="/register" className="font-medium text-shield-800 hover:underline">
            Create one
          </Link>
        </p>
      </div>
    </div>
  )
}
