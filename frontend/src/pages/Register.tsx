import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, useNavigate } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import { useRegister } from '@/hooks/useAuth'
import { ApiError } from '@/lib/api'

const schema = z.object({
  display_name: z.string().trim().min(1, 'Display name is required.').max(100),
  email: z.string().email('Enter a valid email address.'),
  password: z.string().min(10, 'Password must be at least 10 characters long.'),
})

type FormValues = z.infer<typeof schema>

export function Register() {
  const navigate = useNavigate()
  const registerMutation = useRegister()
  const {
    register: registerField,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const onSubmit = (values: FormValues) => {
    registerMutation.mutate(values, {
      onSuccess: () => navigate('/', { replace: true }),
    })
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-shield-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <ShieldCheck className="h-8 w-8 text-shield-700" strokeWidth={1.75} />
          <h1 className="text-xl font-semibold tracking-tight text-shield-950">Create your Shield account</h1>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="rounded-lg border border-shield-200 bg-white p-6">
          <div className="mb-4">
            <label htmlFor="display_name" className="mb-1 block text-sm font-medium text-shield-900">
              Display name
            </label>
            <input
              id="display_name"
              type="text"
              autoComplete="name"
              className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
              {...registerField('display_name')}
            />
            {errors.display_name && <p className="mt-1 text-xs text-status-rejected">{errors.display_name.message}</p>}
          </div>

          <div className="mb-4">
            <label htmlFor="email" className="mb-1 block text-sm font-medium text-shield-900">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
              {...registerField('email')}
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
              autoComplete="new-password"
              className="w-full rounded-md border border-shield-200 px-3 py-2 text-sm outline-none focus:border-shield-500"
              {...registerField('password')}
            />
            {errors.password && <p className="mt-1 text-xs text-status-rejected">{errors.password.message}</p>}
            <p className="mt-1 text-xs text-shield-400">At least 10 characters.</p>
          </div>

          {registerMutation.isError && (
            <p className="mb-4 text-sm text-status-rejected">
              {registerMutation.error instanceof ApiError
                ? registerMutation.error.message
                : 'Something went wrong. Please try again.'}
            </p>
          )}

          <button
            type="submit"
            disabled={registerMutation.isPending}
            className="w-full rounded-md bg-shield-800 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-shield-900 disabled:opacity-60"
          >
            {registerMutation.isPending ? 'Creating account…' : 'Create account'}
          </button>
        </form>

        <p className="mt-4 text-center text-sm text-shield-500">
          Already have an account?{' '}
          <Link to="/login" className="font-medium text-shield-800 hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  )
}
