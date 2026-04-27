import { useState, useEffect } from 'react'
import { Folder, ChevronRight, ArrowUp, Loader2 } from 'lucide-react'
import Modal from './Modal'
import { browseDirectory, type BrowseEntry } from '../api/client'

interface DirectoryBrowserProps {
  open: boolean
  onClose: () => void
  onSelect: (path: string) => void
  initialPath?: string
}

export default function DirectoryBrowser({ open, onClose, onSelect, initialPath }: DirectoryBrowserProps) {
  const [currentPath, setCurrentPath] = useState(initialPath || '/')
  const [entries, setEntries] = useState<BrowseEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setCurrentPath(initialPath || '/')
  }, [open, initialPath])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setLoading(true)
    setError('')
    browseDirectory(currentPath)
      .then((data) => { if (!cancelled) setEntries(data) })
      .catch((e) => { if (!cancelled) setError(e.message) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [open, currentPath])

  const goUp = () => {
    const parent = currentPath.replace(/\/[^/]+\/?$/, '') || '/'
    setCurrentPath(parent)
  }

  const handleSelect = () => {
    onSelect(currentPath)
    onClose()
  }

  return (
    <Modal open={open} onClose={onClose} title="Select Directory">
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <button onClick={goUp} disabled={currentPath === '/'}
            className="p-2 border border-gray-700 text-gray-400 hover:text-white disabled:opacity-30 rounded-lg transition">
            <ArrowUp size={14} />
          </button>
          <code className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-sm text-gray-300 truncate">
            {currentPath}
          </code>
        </div>

        <div className="bg-[#111] border border-gray-700 rounded-lg max-h-64 overflow-y-auto">
          {loading ? (
            <div className="flex justify-center py-8"><Loader2 size={18} className="animate-spin text-gray-500" /></div>
          ) : error ? (
            <div className="px-4 py-3 text-xs text-red-400">{error}</div>
          ) : entries.length === 0 ? (
            <div className="px-4 py-3 text-xs text-gray-600">No subdirectories</div>
          ) : (
            entries.map((entry) => (
              <button key={entry.path} onClick={() => setCurrentPath(entry.path)}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 transition text-left">
                <Folder size={14} className="text-amber-400 shrink-0" />
                <span className="flex-1 truncate">{entry.name}</span>
                <ChevronRight size={12} className="text-gray-600 shrink-0" />
              </button>
            ))
          )}
        </div>

        <div className="flex justify-end gap-2 pt-2 border-t border-gray-800">
          <button onClick={onClose} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">Cancel</button>
          <button onClick={handleSelect}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">
            Select
          </button>
        </div>
      </div>
    </Modal>
  )
}
