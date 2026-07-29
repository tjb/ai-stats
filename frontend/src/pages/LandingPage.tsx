import { Link } from 'react-router-dom'
import { GitBranch, Zap, BarChart3, TrendingUp, DollarSign, ArrowRight, FileText } from 'lucide-react'

export function LandingPage() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Nav */}
      <header className="border-b border-slate-800/60 sticky top-0 bg-slate-950/90 backdrop-blur z-10">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-600">
              <span className="text-sm font-bold text-white">A</span>
            </div>
            <span className="text-lg font-bold text-white">AI Stats</span>
          </div>
          <div className="flex items-center gap-6">
            <a
              href="https://github.com/tjb/ai-stats"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-sm text-slate-300 hover:text-white transition-colors"
            >
              <GitBranch className="h-4 w-4" />
              Open source
            </a>
            <Link
              to="/login"
              className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-violet-500 to-purple-600 px-4 py-2 text-sm font-semibold text-white hover:opacity-90 transition-opacity"
            >
              Sign in with GitHub
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="max-w-4xl mx-auto px-6 pt-20 pb-16 text-center">
        {/* Badge */}
        <div className="inline-flex items-center gap-2 rounded-full border border-slate-700 bg-slate-900 px-4 py-1.5 text-sm text-slate-300 mb-10">
          <Zap className="h-3.5 w-3.5 text-violet-400" />
          Supports Copilot, Claude Code, and more
        </div>

        {/* Headline */}
        <h1 className="text-6xl font-extrabold text-white leading-[1.1] mb-6">
          Know exactly what your{' '}
          <span className="bg-gradient-to-r from-violet-400 to-purple-400 bg-clip-text text-transparent">
            AI coding tools
          </span>{' '}
          are worth
        </h1>

        <p className="text-lg text-slate-400 max-w-2xl mx-auto mb-10">
          Connect in 60 seconds. Measure adoption across every AI coding tool, calculate hours
          saved, and generate board-ready ROI reports — automatically.
        </p>

        {/* CTAs */}
        <div className="flex items-center justify-center gap-4 mb-4">
          <Link
            to="/login"
            className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-6 py-3 text-sm font-semibold text-white hover:opacity-90 transition-opacity"
          >
            <GitBranch className="h-4 w-4" />
            Start free with GitHub
            <ArrowRight className="h-4 w-4" />
          </Link>
          <a
            href="#open-source"
            className="inline-flex items-center rounded-xl border border-slate-700 bg-slate-900 px-6 py-3 text-sm font-semibold text-white hover:bg-slate-800 transition-colors"
          >
            View on GitHub
          </a>
        </div>
        <p className="text-sm text-slate-500">Free & open source · Self-hosted · Setup in under 2 minutes</p>
      </section>

      {/* Browser mockup */}
      <section className="max-w-6xl mx-auto px-6 pb-24">
        <div className="rounded-2xl border border-slate-700 bg-slate-900 overflow-hidden shadow-2xl shadow-violet-500/5">
          {/* Browser chrome */}
          <div className="flex items-center gap-3 border-b border-slate-800 bg-slate-900 px-5 py-3">
            <div className="flex gap-1.5">
              <div className="h-3 w-3 rounded-full bg-red-500/80" />
              <div className="h-3 w-3 rounded-full bg-yellow-500/80" />
              <div className="h-3 w-3 rounded-full bg-green-500/80" />
            </div>
            <div className="flex-1 mx-4">
              <div className="rounded-md bg-slate-800 px-3 py-1 text-xs text-slate-400 max-w-xs">
                localhost:8080/dashboard
              </div>
            </div>
          </div>

          {/* Dashboard content */}
          <div className="p-6 bg-slate-950">
            {/* Metric cards */}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 mb-6">
              {[
                { label: 'Active Users', value: '47', change: '+12%', green: true },
                { label: 'Acceptance Rate', value: '38.4%', change: '+3.2%', green: true },
                { label: 'Lines Accepted', value: '142,891', change: '+28%', green: true },
                { label: 'Est. Value Saved', value: '$71,446', change: 'this month', green: false },
              ].map((m) => (
                <div key={m.label} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
                  <p className="text-xs text-slate-500 mb-1">{m.label}</p>
                  <p className="text-2xl font-bold text-white">{m.value}</p>
                  <p className={`text-xs mt-1 ${m.green ? 'text-emerald-400' : 'text-violet-400'}`}>{m.change}</p>
                </div>
              ))}
            </div>

            {/* Mini charts */}
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
                <p className="text-xs text-slate-500 mb-3">Acceptance Rate — Last 90 Days</p>
                <div className="flex items-end gap-0.5 h-20">
                  {[28,30,29,32,31,34,32,35,33,36,34,37,35,38,36,38,37,39,38,40,38,41,39,42,40,43,41,42,40,43,41,44,42,45,43,44,43,46,44,46,45,47,45,48,46,47,45,48,46,49,47,48,47,50,48,49,48,51,49,50,49,52,50,51,50,52,51,53,51,54,52,53,52,54,53,55,53,54,53,55,54,55,54,56,55,56,55,57,55,56].map((v, i) => (
                    <div
                      key={i}
                      className="flex-1 rounded-sm min-w-0"
                      style={{
                        height: `${(v / 65) * 100}%`,
                        backgroundColor: `rgba(139, 92, 246, ${0.25 + (v / 60) * 0.6})`,
                      }}
                    />
                  ))}
                </div>
              </div>
              <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
                <p className="text-xs text-slate-500 mb-3">Daily Active Users</p>
                <div className="flex items-end gap-0.5 h-20">
                  {[35,37,36,38,37,40,38,41,39,42,40,43,41,42,40,43,41,44,42,45,43,44,43,46,44,46,45,47,45,48,46,47,45,48,46,49,47,48,47,50,48,49,48,51,49,50,49,52,50,51,50,52,51,53,51,54,52,53,52,54,53,55,53,54,53,55,54,55,54,56,55,56,55,57,55,56,55,57,56,57,55,56,55,58,56,57,56,58,57,58].map((v, i) => (
                    <div
                      key={i}
                      className="flex-1 rounded-sm min-w-0 bg-violet-500/35"
                      style={{ height: `${(v / 65) * 100}%` }}
                    />
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="max-w-6xl mx-auto px-6 pb-24">
        <h2 className="text-3xl font-bold text-white text-center mb-4">
          Everything you need to measure AI coding ROI
        </h2>
        <p className="text-slate-400 text-center mb-12">
          Built for engineering leaders who need to justify AI tooling spend.
        </p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {[
            {
              Icon: BarChart3,
              title: 'Acceptance Rate Trends',
              description: 'Track how often developers accept AI suggestions over time, broken down by language and editor.',
            },
            {
              Icon: DollarSign,
              title: 'Dollar Value Saved',
              description: 'Configurable hourly rate model turns lines of accepted code into real cost savings.',
            },
            {
              Icon: TrendingUp,
              title: 'Team Utilization',
              description: 'See what share of your engineering team is actively using AI tools and how that changes over time.',
            },
            {
              Icon: GitBranch,
              title: 'Multi-Tool Breakdown',
              description: 'Compare adoption across Copilot, Claude Code, and any custom tool via the push ingest API.',
            },
            {
              Icon: FileText,
              title: 'Monthly ROI Reports',
              description: 'One-click shareable reports for engineering leadership and board presentations.',
            },
            {
              Icon: Zap,
              title: 'Automatic Sync',
              description: 'Connects to GitHub and your AI tools — syncs hourly with no manual exports needed.',
            },
          ].map(({ Icon, title, description }) => (
            <div key={title} className="rounded-xl border border-slate-800 bg-slate-900 p-6">
              <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-violet-500/10 border border-violet-500/20">
                <Icon className="h-5 w-5 text-violet-400" />
              </div>
              <h3 className="text-sm font-semibold text-white mb-2">{title}</h3>
              <p className="text-sm text-slate-400">{description}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Open source */}
      <section id="open-source" className="max-w-6xl mx-auto px-6 pb-24">
        <div className="rounded-2xl border border-slate-800 bg-slate-900 p-12 text-center">
          <div className="mx-auto mb-6 flex h-12 w-12 items-center justify-center rounded-xl bg-violet-500/10 border border-violet-500/20">
            <GitBranch className="h-6 w-6 text-violet-400" />
          </div>
          <h2 className="text-3xl font-bold text-white mb-4">Free, open source, and self-hosted</h2>
          <p className="text-slate-400 max-w-xl mx-auto mb-8">
            AI Stats is MIT-licensed. Run it on your own infrastructure with your own database — no
            seat limits, no subscription, no vendor lock-in.
          </p>
          <div className="flex items-center justify-center gap-4">
            <a
              href="https://github.com/tjb/ai-stats"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-6 py-3 text-sm font-semibold text-white hover:opacity-90 transition-opacity"
            >
              <GitBranch className="h-4 w-4" />
              View source on GitHub
            </a>
            <Link
              to="/login"
              className="inline-flex items-center rounded-xl border border-slate-700 bg-slate-950 px-6 py-3 text-sm font-semibold text-white hover:bg-slate-800 transition-colors"
            >
              Start free with GitHub
            </Link>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="max-w-6xl mx-auto px-6 pb-24">
        <div className="rounded-2xl border border-violet-500/20 bg-gradient-to-br from-violet-500/10 to-purple-600/5 p-12 text-center">
          <h2 className="text-3xl font-bold text-white mb-4">Ready to justify your AI investment?</h2>
          <p className="text-slate-400 mb-8">Connect your tools in under 2 minutes.</p>
          <Link
            to="/login"
            className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-violet-500 to-purple-600 px-8 py-3 text-sm font-semibold text-white hover:opacity-90 transition-opacity"
          >
            <GitBranch className="h-4 w-4" />
            Start free with GitHub
            <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-800 py-8">
        <div className="max-w-6xl mx-auto px-6 flex items-center justify-between text-sm text-slate-500">
          <span>© 2026 AI Stats</span>
          <div className="flex gap-6">
            <a href="/privacy" className="hover:text-slate-300 transition-colors">Privacy</a>
            <a href="/terms" className="hover:text-slate-300 transition-colors">Terms</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
