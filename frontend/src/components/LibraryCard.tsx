import { useRef, useState } from 'react'
import { ExternalLink, RefreshCw, Unlink, CheckCircle, AlertCircle, Layers, FileVideo, Link2, Film } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import * as api from '../api/client'
import type { LibraryGroupResponse } from '../api/client'
import { useLightbox, type LightboxItem } from './Lightbox'
import Modal from './Modal'

interface Props {
  item: LibraryGroupResponse
  onAction: () => void
}

export default function LibraryCard({ item, onAction }: Props) {
  const lightbox = useLightbox()
  const imgRef = useRef<HTMLImageElement>(null)
  const [showUnlinkConfirm, setShowUnlinkConfirm] = useState(false)
  const [unlinking, setUnlinking] = useState(false)
  const outputs = item.outputs || []
  const allAlive = outputs.length > 0 && outputs.every((o) => o.alive)
  const unlinkTargets = buildUnlinkTargets(item.number, outputs)

  const handleRescrape = async () => {
    try { await api.libraryRescrape(item.number) } catch { /* */ }
  }
  const handleUnlink = () => {
    if (outputs.length === 0) return
    setShowUnlinkConfirm(true)
  }
  const handleUnlinkConfirm = async () => {
    if (outputs.length === 0) return
    setUnlinking(true)
    try {
      for (const output of outputs) {
        await api.unlinkOutput(output.id, item.number)
      }
      setShowUnlinkConfirm(false)
      onAction()
    } catch (e: any) { alert(e.message) }
    setUnlinking(false)
  }
  const galleryItems: LightboxItem[] = item.coverURL
    ? [
        {
          src: item.coverURL,
          width: imgRef.current?.naturalWidth || undefined,
          height: imgRef.current?.naturalHeight || undefined,
          msrc: imgRef.current?.currentSrc || undefined,
        },
        ...(item.sampleImages || []).filter(Boolean).map((src) => ({ src })),
      ]
    : []

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition flex flex-col">
      {item.coverURL && (
        <div className="relative aspect-[16/9] overflow-hidden bg-black group/cover cursor-pointer"
          onClick={() => lightbox.open(galleryItems, 0, imgRef.current || undefined)}>
          <img ref={imgRef} src={item.coverURL} alt="" className="w-full h-full object-cover opacity-90 group-hover/cover:scale-105 transition-transform duration-300" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {item.pageURL && (
            <a href={item.pageURL} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()}
              className="absolute top-3 right-3 p-1.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          {(() => {
            const count = 1 + (item.sampleImages?.length || 0); return count > 1 ? (
              <span className="absolute top-3 left-3 flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-amber-500/80 text-[10px] font-bold text-white">
                <Layers size={10} />{count}
              </span>
            ) : null
          })()}
          <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
            <div>
              <span className="text-white font-semibold text-sm">{item.number}</span>
              {item.rating && <span className="ml-2 text-xs text-amber-400">★ {item.rating}{item.reviewCount > 0 && ` (${item.reviewCount})`}</span>}
            </div>
            {allAlive
              ? <span className="text-xs px-2 py-0.5 bg-emerald-500/20 text-emerald-400 rounded flex items-center gap-1"><CheckCircle size={10} />alive</span>
              : <span className="text-xs px-2 py-0.5 bg-red-500/20 text-red-400 rounded flex items-center gap-1"><AlertCircle size={10} />missing</span>}
          </div>
        </div>
      )}
      <div className="p-4 space-y-3 flex-1 flex flex-col">
        {item.title && <div className="text-sm text-gray-300 line-clamp-2">{item.title}</div>}
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          {item.maker && <><dt className="text-gray-600">Info</dt><dd className="text-gray-400">{item.maker} / {item.premiered || item.year} / {item.runtime}min</dd></>}
          {item.actors && <><dt className="text-gray-600">Actors</dt><dd className="text-gray-400">{item.actors}</dd></>}
        </dl>
        {item.genres && item.genres.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {item.genres.map((g) => <span key={g} className="text-[10px] px-1.5 py-0.5 bg-gray-800 text-gray-500 rounded">{g}</span>)}
          </div>
        )}

        <div className="rounded-xl border border-gray-800 bg-[#111] overflow-hidden">
          {outputs.map((output, idx) => (
            <div key={output.id} className={idx > 0 ? 'border-t border-gray-800' : ''}>
              <FlowNode
                label="Source"
                filename={getFilename(output.srcPath) || 'Original source unavailable'}
                pathHint={getParentPath(output.srcPath) || 'Source path missing'}
                meta={[output.resolution, output.videoCodec, output.bitrate, formatFileSize(output.fileSize)].filter(Boolean) as string[]}
                icon={Film}
              />

              <div className="relative flex justify-center py-2">
                <div className="absolute left-3 right-3 top-1/2 h-px -translate-y-1/2 bg-gray-800" />
                <span className="relative inline-flex items-center gap-1 rounded-full border border-indigo-500/20 bg-[#171717] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.16em] text-indigo-300">
                  <Link2 size={10} />
                  {output.linkType}
                </span>
              </div>

              <FlowNode
                label="Output"
                filename={getFilename(output.linkPath) || item.number}
                pathHint={getParentPath(output.linkPath)}
                meta={['Nfo', 'CoverIMG']}
                icon={FileVideo}
              />
            </div>
          ))}
        </div>

        {/* Actions */}
        <div className="flex gap-1.5 pt-2 border-t border-gray-800 mt-auto">
          <button onClick={handleRescrape} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-white hover:bg-[#222] rounded-lg transition">
            <RefreshCw size={12} />Rescrape
          </button>
          <button onClick={handleUnlink} className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-amber-400 hover:bg-[#222] rounded-lg transition">
            <Unlink size={12} />Unlink
          </button>
        </div>
      </div>
      <Modal open={showUnlinkConfirm} onClose={() => !unlinking && setShowUnlinkConfirm(false)} title={`Unlink ${item.number}`}>
        <div className="space-y-4">
          <p className="text-sm text-gray-400">
            This will remove {outputs.length} linked file{outputs.length > 1 ? 's' : ''} and related sidecars.
          </p>
          <div className="rounded-lg border border-gray-800 bg-[#111] max-h-64 overflow-y-auto">
            {unlinkTargets.map((target) => (
              <div key={`${target.kind}-${target.path}`} className="flex items-center gap-2 border-b border-gray-800 last:border-b-0 px-3 py-2">
                <span className="rounded bg-gray-800 px-1.5 py-0.5 text-[10px] text-gray-400">{target.kind}</span>
                <code className="min-w-0 flex-1 truncate text-xs text-gray-500">{target.path}</code>
              </div>
            ))}
          </div>
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={() => setShowUnlinkConfirm(false)}
              disabled={unlinking}
              className="px-3 py-1.5 rounded-lg border border-gray-700 text-xs text-gray-400 hover:text-white hover:bg-[#222] disabled:opacity-50 transition"
            >
              Cancel
            </button>
            <button
              onClick={handleUnlinkConfirm}
              disabled={unlinking}
              className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-xs text-white disabled:opacity-50 transition"
            >
              {unlinking ? 'Unlinking...' : 'Confirm Unlink'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  )
}

function FlowNode({
  label,
  filename,
  pathHint,
  meta,
  icon: Icon,
}: {
  label: string
  filename: string
  pathHint?: string
  meta?: string[]
  icon: LucideIcon
}) {
  const labelColor = label === 'Output' ? 'text-indigo-300' : 'text-gray-600'

  return (
    <div className="flex items-center gap-3 px-3 py-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-gray-800 bg-[#171717]">
        <Icon size={14} className="text-gray-500" />
      </div>
      <div className="min-w-0 flex-1">
        <div className={`mb-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${labelColor}`}>{label}</div>
        <div className="truncate text-sm text-gray-300">{filename}</div>
        {pathHint && <div className="truncate text-[11px] text-gray-600">{pathHint}</div>}
      </div>
      {meta && meta.length > 0 && (
        <div className="hidden max-w-[48%] flex-wrap justify-end gap-1 sm:flex">
          {meta.map((value) => (
            <span key={`${label}-${value}`} className="rounded bg-gray-800 px-1.5 py-0.5 text-[10px] text-gray-500">
              {value}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function getParentPath(path: string) {
  if (!path) return ''
  const parts = path.split('/')
  parts.pop()
  return parts.join('/')
}

function getFilename(path: string) {
  if (!path) return ''
  const parts = path.split('/')
  return parts[parts.length - 1] || ''
}

function formatFileSize(fileSize: number) {
  if (fileSize <= 0) {
    return ''
  }
  return `${(fileSize / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

type UnlinkTarget = {
  kind: 'Link' | 'NFO' | 'IMG'
  path: string
}

function buildUnlinkTargets(number: string, outputs: LibraryGroupResponse['outputs']) {
  const out: UnlinkTarget[] = []
  const seen = new Set<string>()
  const dirs = new Set<string>()

  for (const output of outputs) {
    const linkPath = (output.linkPath || '').trim()
    if (!linkPath) continue

    const linkKey = `link:${linkPath}`
    if (!seen.has(linkKey)) {
      seen.add(linkKey)
      out.push({ kind: 'Link', path: linkPath })
    }

    const dir = getParentPath(linkPath)
    if (dir) dirs.add(dir)
  }

  for (const dir of dirs) {
    const nfo = joinPath(dir, `${number}.nfo`)
    const nfoKey = `nfo:${nfo}`
    if (!seen.has(nfoKey)) {
      seen.add(nfoKey)
      out.push({ kind: 'NFO', path: nfo })
    }

    const img = joinPath(dir, `${number}.(jpg|png|webp|gif)`)
    const imgKey = `img:${img}`
    if (!seen.has(imgKey)) {
      seen.add(imgKey)
      out.push({ kind: 'IMG', path: img })
    }
  }

  return out
}

function joinPath(dir: string, name: string) {
  if (!dir) return name
  if (dir.endsWith('/')) return `${dir}${name}`
  return `${dir}/${name}`
}
