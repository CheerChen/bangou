import { useState, useRef } from 'react'
import { ExternalLink, RefreshCw, Merge, Link2, FileVideo, Check, Download, HelpCircle, ChevronDown, ChevronUp, AlertTriangle, Tag, Layers } from 'lucide-react'
import * as api from '../api/client'
import type { GroupResponse } from '../api/client'
import { useLightbox, type LightboxItem } from './Lightbox'

interface Props {
  group: GroupResponse
  pipelineId: number
  onAction: () => void
}

export default function GroupCard({ group, pipelineId, onAction }: Props) {
  const [selected, setSelected] = useState<Set<string>>(() => {
    const s = new Set<string>()
    group.items.forEach((i) => { if (i.ready) s.add(i.path) })
    return s
  })
  const [errorsOpen, setErrorsOpen] = useState(false)
  const [tagInput, setTagInput] = useState('')
  const lightbox = useLightbox()
  const imgRef = useRef<HTMLImageElement>(null)

  const meta = group.scrape.meta
  const allReady = group.allReady
  const busy = group.task === 'merging' || group.task === 'linking'
  const canAct = allReady && !busy && group.scrape.status === 'success'
  const groupHasMixedMkvMp4 = hasMixedMkvAndMp4(group.items)
  const selectedExtCount = countSelectedExtensions(group.items, selected)
  const canLinkSelection = canAct && selected.size > 0 && selectedExtCount === 1
  const totalGB = group.totalSizeGB
  const isFailed = group.scrape.status === 'failed'
  const isUnknownLike = isFailed || !meta

  const galleryItems: LightboxItem[] = meta?.coverURL
    ? [
        {
          src: meta.coverURL,
          width: imgRef.current?.naturalWidth || undefined,
          height: imgRef.current?.naturalHeight || undefined,
          msrc: imgRef.current?.currentSrc || undefined,
        },
        ...(meta.sampleImages || []).filter(Boolean).map((src) => ({ src })),
      ]
    : []

  const handleLink = async () => {
    const paths = [...selected]
    if (paths.length === 0) return
    if (selectedExtCount !== 1) return
    try { await api.groupLink(pipelineId, group.number, paths); onAction() } catch (e: any) { alert(e.message) }
  }
  const handleMerge = async () => {
    const paths = [...selected]
    if (paths.length < 2) return
    try { await api.groupMerge(pipelineId, group.number, paths); onAction() } catch (e: any) { alert(e.message) }
  }
  const handleRescrape = async () => {
    try { await api.groupRescrape(pipelineId, group.number); onAction() } catch { /* */ }
  }
  const handleTag = async () => {
    if (!tagInput.trim()) return
    try { await api.groupTag(pipelineId, tagInput.trim().toUpperCase(), group.items[0]?.path || ''); onAction() } catch { /* */ }
  }

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition flex flex-col">
      {!isUnknownLike && meta?.coverURL ? (
        <div className="relative aspect-[16/9] overflow-hidden bg-black group/cover cursor-pointer"
          onClick={() => lightbox.open(galleryItems, 0, imgRef.current || undefined)}>
          <img ref={imgRef} src={meta.coverURL} alt="" className="w-full h-full object-cover opacity-90 group-hover/cover:scale-105 transition-transform duration-300" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {meta.pageURL && (
            <a href={meta.pageURL} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()}
              className="absolute top-3 right-3 p-1.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          {(() => { const count = 1 + (meta.sampleImages?.length || 0); return count > 1 ? (
            <span className="absolute top-3 left-3 flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-amber-500/80 text-[10px] font-bold text-white">
              <Layers size={10} />{count}
            </span>
          ) : null })()}
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <div>
              <span className="text-white font-semibold text-sm">{group.number}</span>
              {meta.rating && <span className="ml-2 text-xs text-amber-400">★ {meta.rating}{meta.reviewCount > 0 && ` (${meta.reviewCount})`}</span>}
            </div>
            <StatusPill task={group.task} scrape={group.scrape.status} allReady={allReady} progress={group.taskProgress} />
          </div>
        </div>
      ) : (
        <div className="relative aspect-[16/9] overflow-hidden bg-[#111] flex items-center justify-center">
          <HelpCircle size={48} className="text-gray-800" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <span className="text-white font-semibold text-sm">{group.number}</span>
            <StatusPill task={group.task} scrape={group.scrape.status} allReady={allReady} progress={group.taskProgress} />
          </div>
        </div>
      )}

      <div className="p-4 space-y-3 flex-1 flex flex-col">
        {group.task === 'error' && group.taskErr && <div className="text-xs text-red-400">{group.taskErr}</div>}

        {group.scrape.status === 'success' && meta && (
          <>
            <div className="text-sm text-gray-300 line-clamp-2">{meta.title}</div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
              {meta.maker && <><dt className="text-gray-600">Info</dt><dd className="text-gray-400">{meta.maker} / {meta.premiered || meta.year} / {meta.runtime}min</dd></>}
              {meta.actors && meta.actors.length > 0 && <><dt className="text-gray-600">Actors</dt><dd className="text-gray-400">{meta.actors.join(', ')}</dd></>}
            </dl>
            {meta.genres && meta.genres.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {meta.genres.map((g) => <span key={g} className="text-[10px] px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded">{g}</span>)}
              </div>
            )}
          </>
        )}

        {isFailed && (
          <div>
            <button onClick={() => setErrorsOpen(!errorsOpen)} className="flex items-center gap-1.5 text-xs text-amber-400 hover:text-amber-300 transition">
              <AlertTriangle size={12} />Scrape failed
              {errorsOpen ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            </button>
            {errorsOpen && group.scrape.errors && (
              <div className="mt-2 p-2 bg-[#111] rounded-lg text-xs text-gray-500 space-y-1">
                {Object.entries(group.scrape.errors).map(([k, v]) => <div key={k}><span className="text-gray-600">{k}:</span> {v}</div>)}
              </div>
            )}
            <div className="flex items-center gap-2 mt-2">
              <input type="text" value={tagInput} onChange={(e) => setTagInput(e.target.value)} placeholder="Manual tag e.g. MDVR-336"
                className="flex-1 px-2.5 py-1.5 bg-[#111] border border-gray-700 rounded-lg text-white text-xs placeholder-gray-700 focus:border-indigo-500 focus:outline-none" />
              <button onClick={handleTag} className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition">
                <Tag size={11} />Tag
              </button>
            </div>
          </div>
        )}

        <div className="space-y-1">
          {group.items.map((item) => (
            <label key={item.path} className="relative flex items-center gap-2 overflow-hidden rounded px-2 py-1.5 text-xs transition hover:bg-[#222] cursor-pointer">
              {item.downloadPct > 0 && !item.ready && (
                <div
                  className="absolute inset-y-0 left-0 bg-amber-500/12 transition-all"
                  style={{ width: `${item.downloadPct}%` }}
                />
              )}
              {group.task === 'merging' && selected.has(item.path) && group.taskProgress > 0 && (
                <div
                  className="absolute inset-y-0 left-0 bg-indigo-500/12 transition-all"
                  style={{ width: `${group.taskProgress}%` }}
                />
              )}
              <input type="checkbox" checked={selected.has(item.path)} disabled={!item.ready}
                onChange={(e) => { const next = new Set(selected); e.target.checked ? next.add(item.path) : next.delete(item.path); setSelected(next) }}
                className="relative z-10 rounded border-gray-700 bg-transparent text-indigo-500 focus:ring-indigo-500" />
              <FileVideo size={12} className="relative z-10 text-gray-600 shrink-0" />
              <span className="relative z-10 flex-1 text-gray-400 truncate">
                {!item.ready && item.downloadPct > 0 && <span className="text-amber-400 mr-1">{item.downloadPct}%</span>}
                {item.ready && <span className="text-emerald-400 mr-1">✓</span>}
                {item.filename}
              </span>
              {item.resolution && <span className="relative z-10 px-1 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px] hidden sm:inline">{item.resolution}</span>}
              {item.videoCodec && <span className="relative z-10 px-1 py-0.5 bg-gray-800 text-gray-500 rounded text-[10px] hidden sm:inline">{item.videoCodec}</span>}
              <span className="relative z-10 text-gray-600 whitespace-nowrap">{item.sizeGB.toFixed(2)} GB</span>
            </label>
          ))}
        </div>

        <div className="flex items-center justify-between pt-2 border-t border-gray-800 mt-auto">
          <span className="text-xs text-gray-600">{group.items.length} files, {totalGB.toFixed(2)} GB</span>
          <div className="flex gap-1.5">
            {!busy && (
              <>
                <button onClick={handleRescrape} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-white hover:bg-[#222] rounded-lg transition">
                  <RefreshCw size={12} />Rescrape
                </button>
                {group.items.length > 1 && allReady && !groupHasMixedMkvMp4 && (
                  <button onClick={handleMerge} className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg transition">
                    <Merge size={12} />Merge
                  </button>
                )}
                {canAct && (
                  <button onClick={handleLink} disabled={!canLinkSelection}
                    className={`flex items-center gap-1 text-xs px-2.5 py-1.5 rounded-lg transition ${
                      canLinkSelection ? 'bg-indigo-600 hover:bg-indigo-500 text-white' : 'bg-gray-800 text-gray-500 cursor-not-allowed'
                    }`}>
                    <Link2 size={12} />Link
                  </button>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function countSelectedExtensions(items: GroupResponse['items'], selected: Set<string>) {
  const extSet = new Set<string>()
  for (const item of items) {
    if (!selected.has(item.path)) continue
    const ext = getExt(item.filename || item.path)
    if (ext) extSet.add(ext)
    if (extSet.size > 1) return extSet.size
  }
  return extSet.size
}

function hasMixedMkvAndMp4(items: GroupResponse['items']) {
  let hasMKV = false
  let hasMP4 = false
  for (const item of items) {
    const ext = getExt(item.filename || item.path)
    if (ext === '.mkv') hasMKV = true
    if (ext === '.mp4') hasMP4 = true
    if (hasMKV && hasMP4) return true
  }
  return false
}

function getExt(name: string) {
  const idx = name.lastIndexOf('.')
  if (idx < 0 || idx === name.length - 1) return ''
  return name.slice(idx).toLowerCase()
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
