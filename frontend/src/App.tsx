import { Routes, Route } from 'react-router-dom'
import { AppShell } from '@/components/AppShell'
import { RequireAuth } from '@/components/RequireAuth'
import { Dashboard } from '@/pages/Dashboard'
import { Evidence } from '@/pages/Evidence'
import { EvidenceDetail } from '@/pages/EvidenceDetail'
import { Disclosures } from '@/pages/Disclosures'
import { CreateDisclosure } from '@/pages/CreateDisclosure'
import { Login } from '@/pages/Login'
import { Register } from '@/pages/Register'
import { Placeholder } from '@/pages/Placeholder'

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/evidence" element={<Evidence />} />
          <Route path="/evidence/:id" element={<EvidenceDetail />} />
          <Route
            path="/timeline"
            element={
              <Placeholder
                title="Timeline"
                description="A chronological view of events across your evidence."
              />
            }
          />
          <Route path="/disclosures" element={<Disclosures />} />
          <Route path="/disclosures/new" element={<CreateDisclosure />} />
          <Route
            path="/activity"
            element={
              <Placeholder
                title="Activity"
                description="An append-only audit trail of everything that happened."
              />
            }
          />
          <Route
            path="/settings"
            element={<Placeholder title="Settings" description="Manage your account and preferences." />}
          />
        </Route>
      </Route>
    </Routes>
  )
}

export default App
