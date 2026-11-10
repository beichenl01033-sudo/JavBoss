import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import AppThemeProvider, { useAppTheme } from '@/app/theme/AppThemeProvider'
import HighlightModal from '@/features/highlights/components/HighlightModal'
import HighlightToolbar from '@/features/highlights/components/HighlightToolbar'
import JavEntityWorksModal from '@/features/jav/components/JavEntityWorksModal'
import ClipTasksView from '@/features/clips/components/ClipTasksView'
import { useStore } from '@/store'
import '@/index.css'

localStorage.setItem('javboss.theme', 'dark')
window.appErrors = []
window.addEventListener('error', (event) => window.appErrors.push(event.message))
useStore.setState({ config: { mpv_enabled: 'true' } })
window.fetch = async (input) => {
  const url = new URL(input, location.origin)
  if (url.pathname === '/videos/1/highlights')
    return Response.json({ version: 1, revision: 'one', items: [{ time: 12, label: 'Favorite' }] })
  if (url.pathname === '/clips')
    return Response.json({
      items: [
        {
          id: 'one',
          status: 'completed',
          progress: 100,
          output_filename: 'clip.mp4',
          output_path: '/videos/clip.mp4',
          start_seconds: 0,
          end_seconds: 10,
          created_at: '2026-11-10T00:00:00Z',
          network_optimized: true,
        },
      ],
      counts: { active: 0, completed: 1, failed: 0 },
    })
  return Response.json({ items: [], total: 0 })
}

function Fixture() {
  const { preference, setPreference } = useAppTheme()
  const [view, setView] = useState('highlights')
  return (
    <>
      <nav>
        {['highlights', 'queue', 'entity', 'clips'].map((value) => (
          <button key={value} onClick={() => setView(value)}>
            {value}
          </button>
        ))}
        <button onClick={() => setPreference(preference === 'dark' ? 'light' : 'dark')}>
          Toggle theme
        </button>
      </nav>
      {view === 'highlights' && (
        <HighlightModal
          videos={[{ id: 1, filename: 'movie.mp4' }]}
          onClose={() => setView('queue')}
        />
      )}
      {view === 'queue' && <HighlightToolbar />}
      {view === 'entity' && (
        <JavEntityWorksModal
          entity={{ id: 1, type: 'idol', name: 'Example' }}
          onClose={() => setView('queue')}
        />
      )}
      {view === 'clips' && <ClipTasksView />}
    </>
  )
}

createRoot(document.getElementById('root')).render(
  <AppThemeProvider>
    <Fixture />
  </AppThemeProvider>
)
