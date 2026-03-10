import { create } from 'zustand'

type SessionState = {
  token: string | null
  tenantId: string | null
  userId: string | null
  setSession: (token: string, tenantId: string, userId: string) => void
  clearSession: () => void
}

export const useSessionStore = create<SessionState>((set) => ({
  token: null,
  tenantId: null,
  userId: null,
  setSession: (token, tenantId, userId) => set({ token, tenantId, userId }),
  clearSession: () => set({ token: null, tenantId: null, userId: null })
}))
