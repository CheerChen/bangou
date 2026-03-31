import { ExternalLink, RefreshCw, Unlink, Trash2, CheckCircle, AlertCircle } from 'lucide-react'
import * as api from '../api/client'
import type { LibraryItemResponse } from '../api/client'

interface Props {
  item: LibraryItemResponse
  onAction: () => void
}

export default function LibraryCard({ item, onAction }: Props) {
  const handleRescrape = async () => {
    try { await api.libraryRescrape(item.number) } catch { /* */ }
  }
  const handleUnlink = async () => {
    if (!confirm(`Unlink ${item.number}?`)) return
    try { await api.unlinkOutput(item.id, item.number); onAction() } catch (e: any) { alert(e.message) }
  }
  const handleDelete = async () => {
    if (!confirm(`Delete ${item.number} from library?`)) return
    try { await api.deleteOutput(item.id); onAction() } catch (e: any) { alert(e.message) }
  }

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition">
      {item.coverURL && (
        <div className="relative aspect-[16/9] overflow-hidden bg-black">
          <img src={item.coverURL} alt="" className="w-full h-full object-cover opacity-90" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {item.pageURL && (
            <a href={item.pageURL} target="_blank" rel="noopener"
              className="absolute top-3 right-3 p-1.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <div>
              <span className="text-white font-semibold text-sm">{item.number}</span>
              {item.rating && <span className="ml-2 text-xs text-amber-400">★ {item.rating}{item.reviewCount > 0 && ` (${item.reviewCount})`}</span>}
            </div>
            {item.alive
              ? <span className="text-xs px-2 py-0.5 bg-emerald-500/20 text-emerald-400 rounded flex items-center gap-1"><CheckCircle size={10} />alive</span>
              : <span className="text-xs px-2 py-0.5 bg-red-500/20 text-red-400 rounded flex items-center gap-1"><AlertCircle size={10} />missing</span>}
          </div>
        </div>
      )}
      <div className="p-4 space-y-3">
        {item.title && <div className="text-sm text-gray-300 line-clamp-2">{item.title}</div>}
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          {item.maker && <><dt className="text-gray-600">Info</dt><dd className="text-gray-400">{item.maker} / {item.year} / {item.runtime}min</dd></>}
          {item.actors && <><dt className="text-gray-600">Actors</dt><dd className="text-gray-400">{item.actors}</dd></>}
        </dl>
        {item.genres && item.genres.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {item.genres.map((g) => <span key={g} className="text-[10px] px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded">{g}</span>)}
          </div>
        )}
        <div className="text-xs text-gray-600 space-y-0.5">
          <div><span className="text-gray-500">{item.linkType}</span>{item.srcPath && <span>: <code className="text-gray-500">{item.srcPath}</code></span>}</div>
          <div>→ <code className="text-gray-500">{item.linkPath}</code></div>
        </div>
        <div className="flex gap-1.5 pt-2 border-t border-gray-800">
          <button onClick={handleRescrape} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-white hover:bg-[#222] rounded-lg transition">
            <RefreshCw size={12} />Rescrape
          </button>
          <button onClick={handleUnlink} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-amber-400 hover:bg-[#222] rounded-lg transition">
            <Unlink size={12} />Unlink
          </button>
          <button onClick={handleDelete} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-red-400 hover:bg-[#222] rounded-lg transition">
            <Trash2 size={12} />Remove
          </button>
        </div>
      </div>
    </div>
  )
}
