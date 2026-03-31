import { useState } from 'react'
import { ExternalLink, RefreshCw, Merge, Link2, EyeOff, FileVideo, Check, Download, HelpCircle, ChevronDown, ChevronUp, AlertTriangle, Tag } from 'lucide-react'
import type { Group } from '../types'

export default function GroupCard({ group }: { group: Group }) {
  const [selected, setSelected] = useState<Set<string>>(() => {
    const s = new Set<string>()
    group.items.forEach((i) => { if (i.ready) s.add(i.path) })
    return s
  })
  const [errorsOpen, setErrorsOpen] = useState(false)
  const [tagInput, setTagInput] = useState('')

  const meta = group.scrape.meta
  const allReady = group.items.every((i) => i.ready)
  const anyDownloading = group.items.some((i) => !i.ready && i.downloadPct > 0)
  const busy = group.task === 'merging' || group.task === 'linking'
  const canAct = allReady && !busy && group.scrape.status === 'success'
  const totalGB = group.items.reduce((s, i) => s + i.sizeGB, 0)
  const isFailed = group.scrape.status === 'failed'
  const isUnknownLike = isFailed || !meta

  // Average download progress across all files
  const downloadingItems = group.items.filter((i) => !i.ready && i.downloadPct > 0)
  const avgDownloadPct = downloadingItems.length > 0
    ? Math.round(downloadingItems.reduce((s, i) => s + i.downloadPct, 0) / group.items.length)
    : 0

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition">
      {/* Cover area */}
      {!isUnknownLike && meta?.coverURL ? (
        <div className="relative aspect-[16/9] overflow-hidden bg-black">
          <img src={meta.coverURL} alt="" className="w-full h-full object-cover opacity-90" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {meta.pageURL && (
            <a href={meta.pageURL} target="_blank" rel="noopener"
              className="absolute top-3 right-3 p-1.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <div>
              <span className="text-white font-semibold text-sm">{group.number}</span>
              {meta.rating && (
                <span className="ml-2 text-xs text-amber-400">★ {meta.rating}{meta.reviewCount > 0 && ` (${meta.reviewCount})`}</span>
              )}
            </div>
            <StatusPill task={group.task} scrape={group.scrape.status} allReady={allReady} progress={group.taskProgress} />
          </div>
          {/* Merge progress (indigo) */}
          {group.task === 'merging' && group.taskProgress > 0 && (
            <div className="absolute bottom-0 left-0 right-0 h-1 bg-gray-800">
              <div className="h-full bg-indigo-500 transition-all" style={{ width: `${group.taskProgress}%` }} />
            </div>
          )}
          {/* Download progress (amber) */}
          {anyDownloading && !group.task && (
            <div className="absolute bottom-0 left-0 right-0 h-1 bg-gray-800">
              <div className="h-full bg-amber-500 transition-all" style={{ width: `${avgDownloadPct}%` }} />
            </div>
          )}
        </div>
      ) : (
        /* Unknown / Failed style: ? placeholder */
        <div className="relative aspect-[16/9] overflow-hidden bg-[#111] flex items-center justify-center">
          <HelpCircle size={48} className="text-gray-800" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <span className="text-white font-semibold text-sm">{group.number}</span>
            <StatusPill task={group.task} scrape={group.scrape.status} allReady={allReady} progress={group.taskProgress} />
          </div>
          {anyDownloading && !group.task && (
            <div className="absolute bottom-0 left-0 right-0 h-1 bg-gray-800">
              <div className="h-full bg-amber-500 transition-all" style={{ width: `${avgDownloadPct}%` }} />
            </div>
          )}
        </div>
      )}

      <div className="p-4 space-y-3">
        {/* Task error */}
        {group.task === 'error' && group.taskErr && (
          <div className="text-xs text-red-400">{group.taskErr}</div>
        )}

        {/* Metadata (success only) */}
        {group.scrape.status === 'success' && meta && (
          <>
            <div className="text-sm text-gray-300 line-clamp-2">{meta.title}</div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
              {meta.maker && <><dt className="text-gray-600">Info</dt><dd className="text-gray-400">{meta.maker} / {meta.year} / {meta.runtime}min</dd></>}
              {meta.actors.length > 0 && <><dt className="text-gray-600">Actors</dt><dd className="text-gray-400">{meta.actors.join(', ')}</dd></>}
            </dl>
            {meta.genres.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {meta.genres.map((g) => (
                  <span key={g} className="text-[10px] px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded">{g}</span>
                ))}
              </div>
            )}
          </>
        )}

        {/* Failed: expandable error log */}
        {isFailed && (
          <div>
            <button onClick={() => setErrorsOpen(!errorsOpen)}
              className="flex items-center gap-1.5 text-xs text-amber-400 hover:text-amber-300 transition">
              <AlertTriangle size={12} />
              Scrape failed
              {errorsOpen ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            </button>
            {errorsOpen && (
              <div className="mt-2 p-2 bg-[#111] rounded-lg text-xs text-gray-500 space-y-1">
                {Object.entries(group.scrape.errors).map(([k, v]) => (
                  <div key={k}><span className="text-gray-600">{k}:</span> {v}</div>
                ))}
              </div>
            )}
            {/* Manual tag input for failed */}
            <div className="flex items-center gap-2 mt-2">
              <input type="text" value={tagInput} onChange={(e) => setTagInput(e.target.value)} placeholder="Manual tag e.g. MDVR-336"
                className="flex-1 px-2.5 py-1.5 bg-[#111] border border-gray-700 rounded-lg text-white text-xs placeholder-gray-700 focus:border-indigo-500 focus:outline-none" />
              <button className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition">
                <Tag size={11} />Tag
              </button>
            </div>
          </div>
        )}

        {/* File list */}
        <div className="space-y-1">
          {group.items.map((item) => (
            <label key={item.path} className="flex items-center gap-2 text-xs py-1.5 px-2 rounded hover:bg-[#222] cursor-pointer">
              <input type="checkbox" checked={selected.has(item.path)} disabled={!item.ready}
                onChange={(e) => { const next = new Set(selected); e.target.checked ? next.add(item.path) : next.delete(item.path); setSelected(next) }}
                className="rounded border-gray-700 bg-transparent text-indigo-500 focus:ring-indigo-500" />
              <FileVideo size={12} className="text-gray-600 shrink-0" />
              <span className="flex-1 text-gray-400 truncate">
                {!item.ready && item.downloadPct > 0 && <span className="text-amber-400 mr-1">{item.downloadPct}%</span>}
                {item.ready && <span className="text-emerald-400 mr-1">✓</span>}
                {item.filename}
              </span>
              {item.media && (
                <span className="hidden sm:flex gap-1">
                  {item.media.resolution && <span className="px-1 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.media.resolution}</span>}
                  {item.media.videoCodec && <span className="px-1 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px]">{item.media.videoCodec}</span>}
                </span>
              )}
              <span className="text-gray-600 whitespace-nowrap">{item.sizeGB.toFixed(2)} GB</span>
            </label>
          ))}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between pt-2 border-t border-gray-800">
          <span className="text-xs text-gray-600">{group.items.length} files, {totalGB.toFixed(2)} GB</span>
          <div className="flex gap-1.5">
            {!busy && (
              <>
                <button className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-white hover:bg-[#222] rounded-lg transition" title="Rescrape">
                  <RefreshCw size={12} />Rescrape
                </button>
                {group.items.length > 1 && allReady && (
                  <button className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg transition" title="Merge">
                    <Merge size={12} />Merge
                  </button>
                )}
                {canAct && (
                  <button className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition" title="Link">
                    <Link2 size={12} />Link
                  </button>
                )}
                <button className="p-1.5 text-gray-600 hover:text-red-400 rounded-lg transition" title="Ignore">
                  <EyeOff size={14} />
                </button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function StatusPill({ task, scrape, allReady, progress }: { task: string; scrape: string; allReady: boolean; progress: number }) {
  if (task === 'merging') return <span className="text-xs px-2 py-0.5 bg-indigo-500/20 text-indigo-400 rounded animate-pulse flex items-center gap-1"><Merge size={10} />merging{progress > 0 && ` ${progress}%`}</span>
  if (task === 'linking') return <span className="text-xs px-2 py-0.5 bg-indigo-500/20 text-indigo-400 rounded animate-pulse flex items-center gap-1"><Link2 size={10} />linking</span>
  if (task === 'error') return <span className="text-xs px-2 py-0.5 bg-red-500/20 text-red-400 rounded">error</span>
  if (!allReady) return <span className="text-xs px-2 py-0.5 bg-amber-500/20 text-amber-400 rounded animate-pulse flex items-center gap-1"><Download size={10} />downloading</span>
  if (scrape === 'scraping') return <span className="text-xs px-2 py-0.5 bg-indigo-500/20 text-indigo-400 rounded animate-pulse">scraping</span>
  if (scrape === 'success') return <span className="text-xs px-2 py-0.5 bg-emerald-500/20 text-emerald-400 rounded flex items-center gap-1"><Check size={10} />ready</span>
  if (scrape === 'failed') return <span className="text-xs px-2 py-0.5 bg-amber-500/20 text-amber-400 rounded flex items-center gap-1"><AlertTriangle size={10} />failed</span>
  return null
}
