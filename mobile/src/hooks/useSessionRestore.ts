import { useEffect } from 'react'

import { useSessionStore } from '../store/sessionStore'

export function useSessionRestore(): void {
  const clearSession = useSessionStore((state) => state.clearSession)

  useEffect(() => {
    // Local persistence wiring will be added with AsyncStorage in Phase 2.
    // This placeholder keeps app bootstrap behavior explicit.
    clearSession()
  }, [clearSession])
}
