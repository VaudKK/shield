import { Routes, Route } from 'react-router-dom'
import { AppShell } from '@/components/AppShell'
import { Dashboard } from '@/pages/Dashboard'
import { Placeholder } from '@/pages/Placeholder'

function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<Dashboard />} />
        <Route
          path="/evidence"
          element={
            <Placeholder
              title="Evidence"
              description="Upload, organize, and review preserved evidence."
            />
          }
        />
        <Route
          path="/timeline"
          element={
            <Placeholder
              title="Timeline"
              description="A chronological view of events across your evidence."
            />
          }
        />
        <Route
          path="/disclosures"
          element={
            <Placeholder
              title="Disclosures"
              description="Build and manage controlled disclosure packages."
            />
          }
        />
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
    </Routes>
  )
}

export default App
