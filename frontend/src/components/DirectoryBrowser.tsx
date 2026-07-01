import { useReducer, useEffect } from 'react'
import { Folder, ChevronRight, ArrowUp, Loader2 } from 'lucide-react'
import Modal from './Modal'
import { browseDirectory, type BrowseEntry } from '../api/client'

interface DirectoryBrowserProps {
  open: boolean
  onClose: () => void
  onSelect: (path: string) => void
  initialPath?: string
}

type State = {
  path: string
  entries: BrowseEntry[]
  loading: boolean
  error: string
}

type Action =
  | { type: 'reset'; path: string }
  | { type: 'navigate'; path: string }
  | { type: 'loadStart' }
  | { type: 'loadSuccess'; entries: BrowseEntry[] }
  | { type: 'loadError'; error: string }

function init(path: string): State {
  return { path, entries: [], loading: false, error: '' }
}

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'reset':
      return init(action.path)
    case 'navigate':
      return { ...state, path: action.path }
    case 'loadStart':
      return { ...state, loading: true, error: '' }
    case 'loadSuccess':
      return { ...state, loading: false, entries: action.entries }
    case 'loadError':
      return { ...state, loading: false, error: action.error }
  }
}

export default function DirectoryBrowser({ open, onClose, onSelect, initialPath }: DirectoryBrowserProps) {
  const [state, dispatch] = useReducer(reducer, initialPath || '/', init)

  // Reset to initialPath when the dialog opens (or initialPath changes).
  // Runs once per open transition rather than on every path navigation.
  useEffect(() => {
    if (!open) return
    dispatch({ type: 'reset', path: initialPath || '/' })
  }, [open, initialPath])

  // Fetch entries whenever the dialog is open and the current path changes.
  useEffect(() => {
    if (!open) return
    let cancelled = false
    dispatch({ type: 'loadStart' })
    browseDirectory(state.path)
      .then((data) => { if (!cancelled) dispatch({ type: 'loadSuccess', entries: data }) })
      .catch((e) => { if (!cancelled) dispatch({ type: 'loadError', error: e.message }) })
    return () => { cancelled = true }
  }, [open, state.path])

  const goUp = () => {
    const parent = state.path.replace(/\/[^/]+\/?$/, '') || '/'
    dispatch({ type: 'navigate', path: parent })
  }

  const handleSelect = () => {
    onSelect(state.path)
    onClose()
  }

  return (
    <Modal open={open} onClose={onClose} title="Select Directory">
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <button type="button" onClick={goUp} disabled={state.path === '/'}
            aria-label="Go up one directory"
            className="p-2 border border-gray-700 text-gray-400 hover:text-white disabled:opacity-30 rounded-lg transition">
            <ArrowUp size={14} />
          </button>
          <code className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-sm text-gray-300 truncate">
            {state.path}
          </code>
        </div>

        <div className="bg-[#111] border border-gray-700 rounded-lg max-h-64 overflow-y-auto">
          {state.loading ? (
            <div className="flex justify-center py-8"><Loader2 size={18} className="animate-spin text-gray-500" /></div>
          ) : state.error ? (
            <div className="px-4 py-3 text-xs text-red-400">{state.error}</div>
          ) : state.entries.length === 0 ? (
            <div className="px-4 py-3 text-xs text-gray-600">No subdirectories</div>
          ) : (
            state.entries.map((entry) => (
              <button type="button" key={entry.path} onClick={() => dispatch({ type: 'navigate', path: entry.path })}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 transition text-left">
                <Folder size={14} className="text-amber-400 shrink-0" />
                <span className="flex-1 truncate">{entry.name}</span>
                <ChevronRight size={12} className="text-gray-600 shrink-0" />
              </button>
            ))
          )}
        </div>

        <div className="flex justify-end gap-2 pt-2 border-t border-gray-800">
          <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">Cancel</button>
          <button type="button" onClick={handleSelect}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">
            Select
          </button>
        </div>
      </div>
    </Modal>
  )
}
