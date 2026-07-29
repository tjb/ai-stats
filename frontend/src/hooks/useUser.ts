import { useState, useEffect } from 'react'
import { api, type User } from '../api/client'

interface UseUserResult {
  user: User | null
  loading: boolean
  error: string | null
}

// Simple module-level cache so we don't refetch on every mount
let cachedUser: User | null = null

export function useUser(): UseUserResult {
  const [user, setUser] = useState<User | null>(cachedUser)
  const [loading, setLoading] = useState(cachedUser === null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (cachedUser) return
    api
      .get<User>('/api/me')
      .then((u) => {
        cachedUser = u
        setUser(u)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  return { user, loading, error }
}

export function clearUserCache() {
  cachedUser = null
}
