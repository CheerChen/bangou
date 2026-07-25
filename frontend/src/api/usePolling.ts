import { useState, useEffect, useCallback, useRef } from 'react'
import { errorMessage } from './client'

export function usePolling<T>(fetcher: () => Promise<T>, intervalMs = 3000) {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const mountedRef = useRef(true)

  const refresh = useCallback(async () => {
    try {
      const result = await fetcher()
      if (mountedRef.current) {
        setData(result)
        setError(null)
      }
    } catch (e) {
      if (mountedRef.current) {
        setError(errorMessage(e))
      }
    } finally {
      if (mountedRef.current) {
        setLoading(false)
      }
    }
  }, [fetcher])

  useEffect(() => {
    mountedRef.current = true
    refresh()
    const timer = setInterval(refresh, intervalMs)
    return () => {
      mountedRef.current = false
      clearInterval(timer)
    }
  }, [refresh, intervalMs])

  return { data, error, loading, refresh }
}
