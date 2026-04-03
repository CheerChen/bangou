import { useState, useCallback, useEffect, useMemo, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Link2, RefreshCw, ChevronLeft, ChevronRight, ArrowUpDown, Loader2 } from 'lucide-react'
import * as api from '../api/client'
import type { GroupResponse } from '../api/client'
import { usePolling } from '../api/usePolling'
import GroupCard from '../components/GroupCard'
import LibraryCard from '../components/LibraryCard'
import UnknownCard from '../components/UnknownCard'
import PipelineInfoBar from '../components/PipelineInfoBar'

const PAGE_SIZE = 12

type PendingSort = 'number' | 'size' | 'date' | 'status'

const STATUS_ORDER: Record<string, number> = {
  downloading: 0,
  scraping: 1,
  failed: 2,
  ready: 3,
  linking: 4,
  merging: 4,
  error: 5,
}

function getGroupStatusKey(g: GroupResponse): string {
  if (g.task === 'linking' || g.task === 'merging') return g.task
  if (g.task === 'error') return 'error'
  if (!g.allReady) return 'downloading'
  if (g.scrape.status === 'scraping') return 'scraping'
  if (g.scrape.status === 'failed') return 'failed'
  return 'ready'
}

export default function PipelineDetail() {
  const { id: idStr } = useParams<{ id: string }>()
  const pipelineId = Number(idStr)
  const [tab, setTab] = useState<'pending' | 'library'>('pending')

  // Pipeline info
  const pipesFetcher = useCallback(() => api.listPipelines(), [])
  const { data: pipes } = usePolling(pipesFetcher, 10000)
  const pipeline = pipes?.find((p) => p.id === pipelineId)

  // Groups (polling)
  const groupsFetcher = useCallback(() => api.listGroups(pipelineId), [pipelineId])
  const { data: groupsPage, loading: groupsLoading, refresh: refreshGroups } = usePolling(groupsFetcher, 3000)

  // Auto-scan when Pending tab is activated
  const lastScanRef = useRef(0)
  useEffect(() => {
    if (tab !== 'pending') return
    const now = Date.now()
    if (now - lastScanRef.current < 5000) return // debounce 5s
    lastScanRef.current = now
    api.triggerScan(pipelineId).catch(() => {})
  }, [tab, pipelineId])

  // Pending sort + pagination
  const [pendingPage, setPendingPage] = useState(0)
  const [pendingSort, setPendingSort] = useState<PendingSort>('status')
  const [pendingSortDir, setPendingSortDir] = useState<'asc' | 'desc'>('asc')

  const allPendingItems = useMemo(() => {
    const groups = groupsPage?.groups || []
    const unknowns = groupsPage?.unknowns || []
    return { groups, unknowns, total: groups.length + unknowns.length }
  }, [groupsPage])

  const sortedPendingItems = useMemo(() => {
    const groups = [...allPendingItems.groups].sort((a, b) => {
      let cmp = 0
      switch (pendingSort) {
        case 'number': cmp = a.number.localeCompare(b.number); break
        case 'size': cmp = a.totalSizeGB - b.totalSizeGB; break
        case 'date': {
          const da = a.scrape.meta?.premiered || a.scrape.meta?.year || ''
          const db = b.scrape.meta?.premiered || b.scrape.meta?.year || ''
          cmp = da.localeCompare(db)
          break
        }
        case 'status': cmp = (STATUS_ORDER[getGroupStatusKey(a)] ?? 99) - (STATUS_ORDER[getGroupStatusKey(b)] ?? 99); break
      }
      return pendingSortDir === 'asc' ? cmp : -cmp
    })
    return [
      ...groups.map((g) => ({ type: 'group' as const, data: g })),
      ...allPendingItems.unknowns.map((u) => ({ type: 'unknown' as const, data: u })),
    ]
  }, [allPendingItems, pendingSort, pendingSortDir])

  const pendingTotalPages = Math.ceil(sortedPendingItems.length / PAGE_SIZE)
  const pendingSlice = sortedPendingItems.slice(pendingPage * PAGE_SIZE, (pendingPage + 1) * PAGE_SIZE)

  const togglePendingSort = (key: PendingSort) => {
    if (pendingSort === key) setPendingSortDir(pendingSortDir === 'asc' ? 'desc' : 'asc')
    else { setPendingSort(key); setPendingSortDir(key === 'status' || key === 'number' ? 'asc' : 'desc') }
    setPendingPage(0)
  }

  // Library (paginated server-side)
  const [libPage, setLibPage] = useState(0)
  const [libData, setLibData] = useState<api.LibraryPage | null>(null)
  const [libLoading, setLibLoading] = useState(false)
  const [libSort, setLibSort] = useState('added')
  const [libSortDir, setLibSortDir] = useState<'asc' | 'desc'>('desc')

  const fetchLibrary = useCallback(async () => {
    setLibLoading(true)
    try {
      const data = await api.listLibrary(pipelineId, libPage, PAGE_SIZE, libSort, libSortDir)
      setLibData(data)
    } catch { /* ignore */ }
    setLibLoading(false)
  }, [pipelineId, libPage, libSort, libSortDir])

  useEffect(() => {
    if (tab === 'library') fetchLibrary()
  }, [tab, fetchLibrary])

  // Link All
  const [laStatus, setLaStatus] = useState<api.LinkAllStatus | null>(null)
  const linkableCount = groupsPage?.linkable || 0

  const handleLinkAll = async () => {
    try {
      const s = await api.linkAll(pipelineId)
      setLaStatus(s)
      pollLinkAll()
    } catch (e: any) { alert(e.message) }
  }

  const pollLinkAll = () => {
    const timer = setInterval(async () => {
      try {
        const s = await api.linkAllProgress(pipelineId)
        setLaStatus(s)
        if (!s.running) {
          clearInterval(timer)
          refreshGroups()
          setTimeout(() => setLaStatus(null), 3000)
        }
      } catch { clearInterval(timer) }
    }, 1000)
  }

  const toggleSort = (key: string) => {
    if (libSort === key) setLibSortDir(libSortDir === 'asc' ? 'desc' : 'asc')
    else { setLibSort(key); setLibSortDir('desc') }
    setLibPage(0)
  }

  const handleScan = async () => {
    lastScanRef.current = Date.now()
    try { await api.triggerScan(pipelineId) } catch { /* */ }
  }

  if (!pipeline) {
    return <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
  }

  return (
    <div className="pb-32">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm mb-4">
        <Link to="/" className="text-gray-500 hover:text-white transition">bangou</Link>
        <span className="text-gray-700">›</span>
        <span className="text-white font-medium">{pipeline.name}</span>
      </div>

      <PipelineInfoBar pipeline={pipeline} />

      {/* Tabs */}
      <div className="flex items-center gap-1 mb-4 border-b border-gray-800">
        <button onClick={() => setTab('pending')}
          className={`px-4 py-2 text-sm transition ${tab === 'pending' ? 'text-white border-b-2 border-indigo-500' : 'text-gray-500 hover:text-gray-300'}`}>
          Pending ({allPendingItems.total})
        </button>
        <button onClick={() => setTab('library')}
          className={`px-4 py-2 text-sm transition ${tab === 'library' ? 'text-white border-b-2 border-indigo-500' : 'text-gray-500 hover:text-gray-300'}`}>
          Library ({libData?.total ?? pipeline.libraryCount})
        </button>
      </div>

      {tab === 'pending' && (
        <>
          {allPendingItems.total > 0 && (
            <div className="flex items-center gap-2 mb-4">
              <ArrowUpDown size={12} className="text-gray-600" />
              {(['number', 'size', 'date', 'status'] as PendingSort[]).map((key) => (
                <button key={key} onClick={() => togglePendingSort(key)}
                  className={`text-xs px-2.5 py-1 rounded-lg transition-all duration-200 ${pendingSort === key ? 'bg-indigo-600 text-white shadow-sm shadow-indigo-500/20' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                  {key}{pendingSort === key && (pendingSortDir === 'desc' ? ' ↓' : ' ↑')}
                </button>
              ))}
              <span className="text-xs text-gray-600 ml-auto">
                {allPendingItems.total} groups
              </span>
            </div>
          )}

          {groupsLoading && allPendingItems.total === 0 ? (
            <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
          ) : allPendingItems.total > 0 ? (
            <>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {pendingSlice.map((item) =>
                  item.type === 'group'
                    ? <GroupCard key={item.data.number} group={item.data} pipelineId={pipelineId} onAction={refreshGroups} />
                    : <UnknownCard key={item.data.path} file={item.data} pipelineId={pipelineId} onAction={refreshGroups} />
                )}
              </div>
              {pendingTotalPages > 1 && (
                <Pagination page={pendingPage} totalPages={pendingTotalPages} onPage={setPendingPage} />
              )}
            </>
          ) : (
            <div className="text-center py-12">
              <p className="text-gray-500 mb-2">No pending groups</p>
              <p className="text-xs text-gray-700">Files added to the input directory will appear here automatically.</p>
            </div>
          )}
        </>
      )}

      {tab === 'library' && (
        <>
          {(libData?.total ?? 0) > 0 && (
            <div className="flex items-center gap-2 mb-4">
              <ArrowUpDown size={12} className="text-gray-600" />
              {['added', 'number', 'date', 'rating'].map((key) => (
                <button key={key} onClick={() => toggleSort(key)}
                  className={`text-xs px-2.5 py-1 rounded-lg transition-all duration-200 ${libSort === key ? 'bg-indigo-600 text-white shadow-sm shadow-indigo-500/20' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                  {key}{libSort === key && (libSortDir === 'desc' ? ' ↓' : ' ↑')}
                </button>
              ))}
              <span className="text-xs text-gray-600 ml-auto">{libData?.total} bangous</span>
            </div>
          )}

          {libLoading && !libData ? (
            <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
          ) : libData && libData.items.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {libData.items.map((item) => <LibraryCard key={item.number} item={item} onAction={fetchLibrary} />)}
            </div>
          ) : (
            <div className="text-center py-12">
              <p className="text-gray-500 mb-2">Library is empty</p>
              <p className="text-xs text-gray-700">Link groups from the Pending tab to build your library.</p>
            </div>
          )}

          {libData && Math.ceil(libData.total / PAGE_SIZE) > 1 && (
            <Pagination page={libPage} totalPages={Math.ceil(libData.total / PAGE_SIZE)} onPage={setLibPage} />
          )}
        </>
      )}

      {/* FABs */}
      <div className="fixed bottom-6 right-6 flex flex-col gap-3 z-40">
        {tab === 'pending' && linkableCount > 0 && (
          laStatus ? (
            <div className="bg-[#1a1a1a] border border-gray-700 rounded-2xl px-4 py-3 shadow-2xl min-w-[180px]">
              {laStatus.running ? (
                <div className="space-y-2">
                  <div className="text-xs text-gray-400">Linking {laStatus.done + 1}/{laStatus.total}</div>
                  <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                    <div className="h-full bg-indigo-500 transition-all duration-500 rounded-full"
                      style={{ width: `${(laStatus.done / laStatus.total) * 100}%` }} />
                  </div>
                </div>
              ) : (
                <div className="text-xs text-emerald-400">Linked {laStatus.done} groups ✓</div>
              )}
            </div>
          ) : (
            <button onClick={handleLinkAll}
              className="flex items-center gap-2 px-5 py-3 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-2xl shadow-2xl shadow-indigo-500/20 transition">
              <Link2 size={16} />Link All ({linkableCount})
            </button>
          )
        )}
        <button onClick={handleScan}
          className="flex items-center gap-2 px-5 py-3 bg-[#1a1a1a] hover:bg-[#222] border border-gray-700 text-gray-300 hover:text-white text-sm rounded-2xl shadow-2xl transition">
          <RefreshCw size={16} />Rescan
        </button>
      </div>
    </div>
  )
}

function Pagination({ page, totalPages, onPage }: { page: number; totalPages: number; onPage: (p: number) => void }) {
  const pages = paginationRange(page, totalPages)
  return (
    <div className="flex items-center justify-center gap-3 mt-6">
      <button onClick={() => onPage(Math.max(0, page - 1))} disabled={page === 0}
        aria-label="Previous page"
        className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronLeft size={16} /></button>
      <div className="flex gap-1">
        {pages.map((p, idx) =>
          p === -1 ? (
            <span key={`ellipsis-${idx}`} className="w-8 h-8 flex items-center justify-center text-xs text-gray-600">…</span>
          ) : (
            <button key={p} onClick={() => onPage(p)}
              className={`w-8 h-8 text-xs rounded-lg transition ${p === page ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
              {p + 1}
            </button>
          )
        )}
      </div>
      <button onClick={() => onPage(Math.min(totalPages - 1, page + 1))} disabled={page >= totalPages - 1}
        aria-label="Next page"
        className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronRight size={16} /></button>
    </div>
  )
}

function paginationRange(current: number, total: number): number[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i)
  const pages: number[] = []
  const near = new Set([0, 1, current - 1, current, current + 1, total - 2, total - 1])
  const sorted = [...near].filter((p) => p >= 0 && p < total).sort((a, b) => a - b)
  for (let i = 0; i < sorted.length; i++) {
    if (i > 0 && sorted[i] - sorted[i - 1] > 1) pages.push(-1)
    pages.push(sorted[i])
  }
  return pages
}
