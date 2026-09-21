import { Routes, Route } from 'react-router-dom'
import { AppShell } from '@/components/AppShell'
import { RequireAuth } from '@/components/RequireAuth'
import { Landing } from '@/pages/Landing'
import { CreateVault } from '@/pages/CreateVault'
import { RecoverVault } from '@/pages/RecoverVault'
import { Dashboard } from '@/pages/Dashboard'
import { Evidence } from '@/pages/Evidence'
import { EvidenceDetail } from '@/pages/EvidenceDetail'
import { Disclosures } from '@/pages/Disclosures'
import { CreateDisclosure } from '@/pages/CreateDisclosure'
import { Login } from '@/pages/Login'
import { Register } from '@/pages/Register'
import { Placeholder } from '@/pages/Placeholder'
import { NotFound } from '@/pages/NotFound'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/create-vault" element={<CreateVault />} />
      <Route path="/recover-vault" element={<RecoverVault />} />

      {/* Optional email/password path — not linked from the default
          landing flow, but kept available (see internal/auth on the
          backend, which still supports it unchanged). */}
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route path="/dashboard" element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route index element={<Dashboard />} />
          <Route path="evidence" element={<Evidence />} />
          <Route path="evidence/:id" element={<EvidenceDetail />} />
          <Route
            path="timeline"
            element={
              <Placeholder
                title="Timeline"
                description="A chronological view of events across your evidence."
              />
            }
          />
          <Route path="disclosures" element={<Disclosures />} />
          <Route path="disclosures/new" element={<CreateDisclosure />} />
          <Route
            path="activity"
            element={
              <Placeholder
                title="Activity"
                description="An append-only audit trail of everything that happened."
              />
            }
          />
          <Route
            path="settings"
            element={<Placeholder title="Settings" description="Manage your account and preferences." />}
          />
        </Route>
      </Route>

      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}

export default App
