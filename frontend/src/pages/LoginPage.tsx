import { GitBranch } from 'lucide-react'

export function LoginPage() {
  function handleGitHubLogin() {
    window.location.href = '/api/auth/github'
  }

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-2 mb-6">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-purple-600">
              <span className="text-base font-bold text-white">C</span>
            </div>
            <span className="text-2xl font-bold text-white">AI Stats</span>
          </div>
          <h1 className="text-2xl font-bold text-white">Sign in to AI Stats</h1>
          <p className="mt-2 text-sm text-slate-400">
            Connect your GitHub organization to get started
          </p>
        </div>

        <div className="rounded-2xl border border-slate-800 bg-slate-900 p-6">
          <button
            onClick={handleGitHubLogin}
            className="flex w-full items-center justify-center gap-3 rounded-xl bg-slate-800 border border-slate-700 px-4 py-3 text-sm font-semibold text-white hover:bg-slate-700 transition-colors"
          >
            <GitBranch className="h-5 w-5" />
            Continue with GitHub
          </button>

          <p className="mt-4 text-center text-xs text-slate-500">
            By signing in, you agree to our{' '}
            <a href="/terms" className="text-violet-400 hover:underline">
              Terms of Service
            </a>{' '}
            and{' '}
            <a href="/privacy" className="text-violet-400 hover:underline">
              Privacy Policy
            </a>
            .
          </p>
        </div>

        <p className="mt-6 text-center text-sm text-slate-500">
          No credit card required · 14-day free trial
        </p>
      </div>
    </div>
  )
}
