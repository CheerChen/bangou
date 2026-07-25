import { useRef, useReducer } from 'react'
import { ExternalLink, RefreshCw, Unlink, CheckCircle, AlertCircle, Layers, FileVideo, Link as LinkIcon, FileText, Image, Loader2, ChevronRight, Calendar, Users, Play, RotateCcw } from 'lucide-react'
import * as api from '../api/client'
import type { BangouResponse, BangouFileResponse } from '../api/client'
import { useLightbox, type LightboxItem } from './useLightbox'
import Modal from './Modal'
import TagList from './TagList'

interface Props {
  item: BangouResponse
  onAction: () => void
}

type AsyncState = {
  showUnlinkConfirm: boolean
  showBackConfirm: boolean
  unlinking: boolean
  restoring: boolean
  backingToPending: boolean
  rescraping: boolean
  error: string | null
}

type AsyncAction =
  | { type: 'showUnlink' }
  | { type: 'hideUnlink' }
  | { type: 'showBack' }
  | { type: 'hideBack' }
  | { type: 'startUnlink' }
  | { type: 'endUnlink' }
  | { type: 'startRestore' }
  | { type: 'endRestore' }
  | { type: 'startBack' }
  | { type: 'endBack' }
  | { type: 'startRescrape' }
  | { type: 'endRescrape' }
  | { type: 'setError'; error: string | null }

const initialAsync: AsyncState = {
  showUnlinkConfirm: false,
  showBackConfirm: false,
  unlinking: false,
  restoring: false,
  backingToPending: false,
  rescraping: false,
  error: null,
}

function asyncReducer(state: AsyncState, action: AsyncAction): AsyncState {
  switch (action.type) {
    case 'showUnlink': return { ...state, showUnlinkConfirm: true }
    case 'hideUnlink': return { ...state, showUnlinkConfirm: false }
    case 'showBack': return { ...state, showBackConfirm: true }
    case 'hideBack': return { ...state, showBackConfirm: false }
    case 'startUnlink': return { ...state, unlinking: true, error: null }
    case 'endUnlink': return { ...state, unlinking: false }
    case 'startRestore': return { ...state, restoring: true, error: null }
    case 'endRestore': return { ...state, restoring: false }
    case 'startBack': return { ...state, backingToPending: true, error: null }
    case 'endBack': return { ...state, backingToPending: false }
    case 'startRescrape': return { ...state, rescraping: true, error: null }
    case 'endRescrape': return { ...state, rescraping: false }
    case 'setError': return { ...state, error: action.error }
  }
}

export default function LibraryCard({ item, onAction }: Props) {
  const lightbox = useLightbox()
  const imgRef = useRef<HTMLImageElement>(null)
  const [state, dispatch] = useReducer(asyncReducer, initialAsync)
  const outputs = item.outputs || []
  const allAlive = outputs.length > 0 && outputs.every((o) => o.alive)
  const missingOutputs = outputs.filter((o) => !o.alive)
  const hasMissing = missingOutputs.length > 0
  const canRestore = hasMissing && missingOutputs.every((o) => o.sourceAvailable)

  const handleRescrape = async () => {
    dispatch({ type: 'startRescrape' })
    try { await api.libraryRescrape(item.number) } catch (e) { dispatch({ type: 'setError', error: api.errorMessage(e) }) }
    dispatch({ type: 'endRescrape' })
  }
  const handleUnlink = () => {
    if (outputs.length === 0) return
    dispatch({ type: 'showUnlink' })
  }
  const handleUnlinkConfirm = async () => {
    dispatch({ type: 'startUnlink' })
    try {
      await api.unlinkBangou(item.id)
      dispatch({ type: 'hideUnlink' })
      onAction()
    } catch (e) { dispatch({ type: 'setError', error: api.errorMessage(e) }) }
    dispatch({ type: 'endUnlink' })
  }
  const handleRestore = async () => {
    if (!canRestore) return
    dispatch({ type: 'startRestore' })
    try {
      await api.restoreBangou(item.id)
      onAction()
    } catch (e) { dispatch({ type: 'setError', error: api.errorMessage(e) }) }
    dispatch({ type: 'endRestore' })
  }
  const handleBackConfirm = async () => {
    dispatch({ type: 'startBack' })
    try {
      await api.backToPendingBangou(item.id)
      dispatch({ type: 'hideBack' })
      onAction()
    } catch (e) { dispatch({ type: 'setError', error: api.errorMessage(e) }) }
    dispatch({ type: 'endBack' })
  }
  const hasVideo = !!item.sampleMovieURL
  // Built on click, not during render — reads the cover img ref.
  const openGallery = () => {
    const galleryItems: LightboxItem[] = []
    if (item.sampleMovieURL) {
      galleryItems.push({ src: item.sampleMovieURL, type: 'video', width: 720, height: 480 })
    }
    if (item.coverURL) {
      galleryItems.push({
        src: item.coverURL,
        width: imgRef.current?.naturalWidth || undefined,
        height: imgRef.current?.naturalHeight || undefined,
        msrc: imgRef.current?.currentSrc || undefined,
      })
    }
    if (item.sampleImages) {
      for (const src of item.sampleImages) {
        if (src) galleryItems.push({ src })
      }
    }
    lightbox.open(galleryItems, 0, imgRef.current || undefined)
  }

  const multiPart = outputs.length > 1
  const totalSize = outputs.reduce((sum, o) => sum + (o.fileSize || 0), 0)
  const unlinkTargets = buildUnlinkTargets(item, outputs)

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition flex flex-col">
      {item.coverURL && (
        <button type="button" className="relative aspect-[16/9] overflow-hidden bg-black group/cover cursor-pointer block w-full"
          aria-label={`Open gallery for ${item.number}`}
          onClick={openGallery}>
          <img ref={imgRef} src={item.coverURL} alt={`${item.number} cover`}
            className="w-full h-full object-cover opacity-90 group-hover/cover:scale-105 transition-transform duration-300 motion-reduce:transition-none motion-reduce:group-hover/cover:scale-100" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
          {item.pageURL && (
            <a href={item.pageURL} target="_blank" rel="noopener" onClick={(e) => e.stopPropagation()}
              aria-label={`Open ${item.number} page`}
              className="absolute top-2 right-2 p-2.5 bg-black/50 hover:bg-black/80 rounded-lg text-gray-400 hover:text-white transition">
              <ExternalLink size={14} />
            </a>
          )}
          {hasVideo && (
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
              <div className="w-12 h-12 rounded-full bg-black/60 flex items-center justify-center group-hover/cover:bg-black/80 transition">
                <Play size={20} className="text-white ml-0.5" fill="currentColor" />
              </div>
            </div>
          )}
          {(() => {
            const count = (item.coverURL ? 1 : 0) + (item.sampleImages?.length || 0); return count > 1 ? (
              <span className="absolute top-2 left-2 flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-amber-500/80 text-xs font-bold text-white">
                <Layers size={11} />{count}
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
              : <span className="text-xs px-2 py-0.5 bg-red-500/20 text-red-400 rounded flex items-center gap-1"><AlertCircle size={10} />link missing</span>}
          </div>
        </button>
      )}
      <div className="p-4 space-y-3 flex-1 flex flex-col">
        {item.title && <div className="text-sm text-gray-300 line-clamp-2">{item.title}</div>}
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
          {(item.premiered || item.year || item.runtime) && <><dt className="text-gray-600"><Calendar size={11} /></dt><dd className="text-gray-400">{[item.premiered || item.year, item.runtime && `${item.runtime}min`].filter(Boolean).join(' / ')}</dd></>}
          {item.actors && <><dt className="text-gray-600"><Users size={11} /></dt><dd className="text-gray-400">{item.actors}</dd></>}
        </dl>
        <TagList genres={item.genres} maker={item.maker} label={item.label} series={item.series} director={item.director} />

        {hasMissing && (
          <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 px-3 py-2 text-xs text-amber-300">
            Linked file was deleted from the library. Source file is {canRestore ? 'still available.' : 'not available for every missing link.'}
          </div>
        )}

        {/* File tree */}
        <details className="rounded-lg border border-gray-800 bg-[#111] overflow-hidden group/tree">
          {/* Summary: single file or multi-part aggregate */}
          <summary className="flex items-center gap-2 px-3 py-2.5 cursor-pointer hover:bg-[#161616] transition text-xs">
            <ChevronRight size={12} className="text-gray-500 transition-transform group-open/tree:rotate-90 shrink-0" />
            <FileVideo size={14} className="text-emerald-400 shrink-0" />
            {multiPart ? (
              <span className="text-gray-300">{outputs.length} files</span>
            ) : (
              <span className="text-gray-300 truncate">{getFilename(outputs[0]?.srcPath) || getFilename(outputs[0]?.linkPath)}</span>
            )}
            <div className="flex gap-1 ml-auto shrink-0">
              {!multiPart && outputs[0]?.resolution && (
                <span className="px-1 py-0.5 bg-gray-800 text-gray-400 rounded text-[11px]">{outputs[0].resolution}</span>
              )}
              {!multiPart && outputs[0]?.videoCodec && (
                <span className="px-1 py-0.5 bg-gray-800 text-gray-400 rounded text-[11px]">{outputs[0].videoCodec}</span>
              )}
              <span className="px-1 py-0.5 bg-gray-800 text-gray-400 rounded text-[11px]">{formatFileSize(totalSize)}</span>
            </div>
          </summary>

          <div className="border-t border-gray-800">
            {multiPart ? (
              <>
                {outputs.map((output) => (
                  <div key={output.id}>
                    {/* Video file */}
                    <div className="flex items-center gap-2 px-3 py-2 hover:bg-[#161616] transition">
                      <TreeLine />
                      <FileVideo size={14} className="text-emerald-400 shrink-0" />
                      <span className="text-xs text-gray-300 truncate">{getFilename(output.srcPath) || getFilename(output.linkPath)}</span>
                      <div className="flex gap-1 ml-auto shrink-0">
                        {output.resolution && <span className="px-1 py-0.5 bg-gray-800 text-gray-400 rounded text-[11px]">{output.resolution}</span>}
                        {output.videoCodec && <span className="px-1 py-0.5 bg-gray-800 text-gray-400 rounded text-[11px]">{output.videoCodec}</span>}
                        <span className="text-[11px] text-gray-600">{formatFileSize(output.fileSize)}</span>
                      </div>
                    </div>
                    {/* Source */}
                    <LinkRow output={output} nested />
                  </div>
                ))}
              </>
            ) : (
              /* Single part: source directly */
              <LinkRow output={outputs[0]} />
            )}

            {/* Sidecar files */}
            {(item.nfoPath || item.coverPath || item.rawPath) && (
              <div className="border-t border-gray-800/50">
                {item.nfoPath && <SidecarRow icon="file" name={getFilename(item.nfoPath)} />}
                {item.coverPath && <SidecarRow icon="image" name={getFilename(item.coverPath)} />}
                {item.rawPath && <SidecarRow icon="file" name={getFilename(item.rawPath)} />}
              </div>
            )}
          </div>
        </details>

        {state.error && (
          <div className="text-xs text-red-400 bg-red-500/10 px-3 py-2 rounded-lg">
            {state.error}
            <button type="button" onClick={() => dispatch({ type: 'setError', error: null })} className="ml-2 text-red-300 hover:text-white">✕</button>
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-1.5 pt-2 border-t border-gray-800 mt-auto">
          <button type="button" onClick={handleRescrape} disabled={state.rescraping}
            className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-white hover:bg-[#222] rounded-lg transition disabled:opacity-50">
            {state.rescraping ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
            Rescrape
          </button>
          {hasMissing ? (
            <>
              <button type="button" onClick={handleRestore} disabled={!canRestore || state.restoring}
                className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-emerald-400 hover:bg-[#222] rounded-lg transition disabled:opacity-40 disabled:hover:text-gray-500">
                {state.restoring ? <Loader2 size={12} className="animate-spin" /> : <LinkIcon size={12} />}
                Restore Link
              </button>
              <button type="button" onClick={() => dispatch({ type: 'showBack' })} disabled={state.backingToPending}
                className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-amber-400 hover:bg-[#222] rounded-lg transition disabled:opacity-50">
                <RotateCcw size={12} />Back to Pending
              </button>
            </>
          ) : (
            <button type="button" onClick={handleUnlink}
              className="flex items-center gap-1 text-xs px-2.5 py-1.5 text-gray-500 hover:text-amber-400 hover:bg-[#222] rounded-lg transition">
              <Unlink size={12} />Unlink
            </button>
          )}
        </div>
      </div>

      {/* Unlink confirmation modal */}
      <UnlinkConfirmModal
        open={state.showUnlinkConfirm}
        number={item.number}
        outputs={outputs}
        unlinkTargets={unlinkTargets}
        unlinking={state.unlinking}
        onCancel={() => dispatch({ type: 'hideUnlink' })}
        onConfirm={handleUnlinkConfirm}
      />

      <BackConfirmModal
        open={state.showBackConfirm}
        number={item.number}
        outputs={outputs}
        backing={state.backingToPending}
        onCancel={() => dispatch({ type: 'hideBack' })}
        onConfirm={handleBackConfirm}
      />
    </div>
  )
}

function TreeLine({ nested }: { nested?: boolean }) {
  if (nested) {
    return (
      <>
        <div className="w-3 ml-0.5 border-l border-gray-700/50 shrink-0" />
        <div className="w-3 ml-0.5 border-l border-b border-gray-700 h-3 rounded-bl-sm shrink-0" />
      </>
    )
  }
  return <div className="w-3 ml-0.5 border-l border-b border-gray-700 h-3 rounded-bl-sm shrink-0" />
}

function LinkRow({ output, nested }: { output: BangouFileResponse; nested?: boolean }) {
  if (!output?.linkPath) return null
  return (
    <div className="flex items-center gap-2 px-3 py-1.5 hover:bg-[#161616] transition">
      <TreeLine nested={nested} />
      {output.alive
        ? <LinkIcon size={12} className="text-gray-600 shrink-0" />
        : <AlertCircle size={12} className="text-red-400 shrink-0" />}
      <span className="text-[11px] text-gray-300 truncate">{getFilename(output.linkPath)}</span>
      <span className={`px-1 py-0.5 rounded text-[11px] ml-auto shrink-0 ${output.alive ? 'bg-indigo-500/10 text-indigo-400' : 'bg-red-500/10 text-red-400'}`}>
        {output.alive ? output.linkType : 'missing'}
      </span>
    </div>
  )
}

function SidecarRow({ icon, name }: { icon: 'file' | 'image'; name: string }) {
  return (
    <div className="flex items-center gap-2 px-3 py-1.5 hover:bg-[#161616] transition">
      <TreeLine />
      {icon === 'image'
        ? <Image size={14} className="text-gray-500 shrink-0" />
        : <FileText size={14} className="text-gray-500 shrink-0" />}
      <span className="text-xs text-gray-500 truncate">{name}</span>
    </div>
  )
}

function getFilename(path: string) {
  if (!path) return ''
  const parts = path.split('/')
  return parts[parts.length - 1] || ''
}

function formatFileSize(fileSize: number) {
  if (fileSize <= 0) return ''
  return `${(fileSize / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

type UnlinkTarget = {
  kind: 'Link' | 'NFO' | 'Cover' | 'Raw'
  path: string
}

function buildUnlinkTargets(bangou: BangouResponse, outputs: BangouResponse['outputs']) {
  const out: UnlinkTarget[] = []
  for (const output of outputs) {
    const linkPath = (output.linkPath || '').trim()
    if (linkPath) out.push({ kind: 'Link', path: linkPath })
  }
  if (bangou.nfoPath) out.push({ kind: 'NFO', path: bangou.nfoPath })
  if (bangou.coverPath) out.push({ kind: 'Cover', path: bangou.coverPath })
  if (bangou.rawPath) out.push({ kind: 'Raw', path: bangou.rawPath })
  return out
}

function UnlinkConfirmModal({ open, number, outputs, unlinkTargets, unlinking, onCancel, onConfirm }: {
  open: boolean
  number: string
  outputs: BangouResponse['outputs']
  unlinkTargets: UnlinkTarget[]
  unlinking: boolean
  onCancel: () => void
  onConfirm: () => void
}) {
  return (
    <Modal open={open} onClose={() => !unlinking && onCancel()} title={`Unlink ${number}`}>
      <div className="space-y-4">
        <p className="text-sm text-gray-400">
          This will remove {outputs.length} linked file{outputs.length > 1 ? 's' : ''} and related sidecars.
        </p>
        <div className="rounded-lg border border-gray-800 bg-[#111] max-h-64 overflow-y-auto">
          {unlinkTargets.map((target) => (
            <div key={`${target.kind}-${target.path}`} className="flex items-center gap-2 border-b border-gray-800 last:border-b-0 px-3 py-2">
              <span className="rounded bg-gray-800 px-1.5 py-0.5 text-[11px] text-gray-400">{target.kind}</span>
              <code className="min-w-0 flex-1 truncate text-xs text-gray-500">{target.path}</code>
            </div>
          ))}
        </div>
        <div className="flex items-center justify-end gap-2">
          <button type="button" onClick={onCancel} disabled={unlinking}
            className="px-3 py-1.5 rounded-lg border border-gray-700 text-xs text-gray-400 hover:text-white hover:bg-[#222] disabled:opacity-50 transition">
            Cancel
          </button>
          <button type="button" onClick={onConfirm} disabled={unlinking}
            className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-xs text-white disabled:opacity-50 transition">
            {unlinking ? 'Unlinking...' : 'Confirm Unlink'}
          </button>
        </div>
      </div>
    </Modal>
  )
}

function BackConfirmModal({ open, number, outputs, backing, onCancel, onConfirm }: {
  open: boolean
  number: string
  outputs: BangouResponse['outputs']
  backing: boolean
  onCancel: () => void
  onConfirm: () => void
}) {
  return (
    <Modal open={open} onClose={() => !backing && onCancel()} title={`Back ${number} to Pending`}>
      <div className="space-y-4">
        <p className="text-sm text-gray-400">
          This removes the library record only. Source files stay in the input directory and will appear in Pending after scan.
        </p>
        <div className="rounded-lg border border-gray-800 bg-[#111] max-h-64 overflow-y-auto">
          {outputs.map((output) => (
            <div key={output.id} className="flex items-center gap-2 border-b border-gray-800 last:border-b-0 px-3 py-2">
              <span className="rounded bg-gray-800 px-1.5 py-0.5 text-[11px] text-gray-400">{output.alive ? 'Link' : 'Missing'}</span>
              <code className="min-w-0 flex-1 truncate text-xs text-gray-500">{output.linkPath}</code>
            </div>
          ))}
        </div>
        <div className="flex items-center justify-end gap-2">
          <button type="button" onClick={onCancel} disabled={backing}
            className="px-3 py-1.5 rounded-lg border border-gray-700 text-xs text-gray-400 hover:text-white hover:bg-[#222] disabled:opacity-50 transition">
            Cancel
          </button>
          <button type="button" onClick={onConfirm} disabled={backing}
            className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-xs text-white disabled:opacity-50 transition">
            {backing ? 'Moving...' : 'Back to Pending'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
