import { useRef } from 'react'
import { ExternalLink, RefreshCw, Unlink, Trash2, CheckCircle, AlertCircle, Layers, Info } from 'lucide-react'
import * as api from '../api/client'
import type { LibraryItemResponse } from '../api/client'
import { useLightbox } from './Lightbox'

interface Props {
  item: LibraryItemResponse
  onAction: () => void
}

export default function LibraryCard({ item, onAction }: Props) {
  const lightbox = useLightbox()
  const imgRef = useRef<HTMLImageElement>(null)
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
        <div className="relative aspect-[16/9] overflow-hidden bg-black group/cover cursor-pointer"
          onClick={() => lightbox.open([item.coverURL!, ...(item.sampleImages || [])], 0, imgRef.current || undefined)}>
          <img ref={imgRef} src={item.coverURL} alt="" className="w-full h-full object-cover opacity-90 group-hover/cover:scale-105 transition-transform duration-300" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {item.pageURL && (
            <a href={item.pageURL} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()}
              className="absolute top-3 right-3 p-1.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          {(() => { const count = 1 + (item.sampleImages?.length || 0); return count > 1 ? (
            <span className="absolute top-3 left-3 flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-amber-500/80 text-[10px] font-bold text-white">
              <Layers size={10} />{count}
            </span>
          ) : null })()}
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
          {item.maker && <><dt className="text-gray-600">Info</dt><dd className="text-gray-400">{item.maker} / {item.premiered || item.year} / {item.runtime}min</dd></>}
          {item.actors && <><dt className="text-gray-600">Actors</dt><dd className="text-gray-400">{item.actors}</dd></>}
        </dl>
        {/* Media tags — like pending cards */}
        <div className="flex flex-wrap gap-1">
          {item.resolution && <span className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.resolution}</span>}
          {item.videoCodec && <span className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.videoCodec}</span>}
          {item.audioCodec && <span className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.audioCodec}</span>}
          {item.bitrate && <span className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.bitrate}</span>}
          {item.fileSize > 0 && <span className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{(item.fileSize / (1024*1024*1024)).toFixed(2)} GB</span>}
          {item.genres?.map((g) => <span key={g} className="px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{g}</span>)}
        </div>
        <div className="flex gap-1.5 pt-2 border-t border-gray-800">
          <div className="relative group/info">
            <button className="p-1.5 text-gray-600 hover:text-white hover:bg-[#222] rounded-lg transition">
              <Info size={14} />
            </button>
            <div className="absolute bottom-full left-0 mb-1 hidden group-hover/info:block z-10 w-64 p-2.5 bg-[#111] border border-gray-700 rounded-lg shadow-xl text-xs text-gray-500 space-y-0.5">
              <div><span className="text-gray-600">{item.linkType}</span>{item.srcPath && <>: <code>{item.srcPath}</code></>}</div>
              <div>→ <code>{item.linkPath}</code></div>
            </div>
          </div>
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
