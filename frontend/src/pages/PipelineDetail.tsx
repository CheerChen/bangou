import { useState, useCallback, useEffect, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Link2, RefreshCw, ChevronLeft, ChevronRight, ArrowUpDown, Loader2 } from 'lucide-react'
import * as api from '../api/client'
import { usePolling } from '../api/usePolling'
import GroupCard from '../components/GroupCard'
import LibraryCard from '../components/LibraryCard'
import UnknownCard from '../components/UnknownCard'
import PipelineInfoBar from '../components/PipelineInfoBar'

const PAGE_SIZE = 12

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

  // Pending pagination (client-side, data already fully loaded)
  const [pendingPage, setPendingPage] = useState(0)

  const allPendingItems = useMemo(() => {
    const groups = groupsPage?.groups || []
    const unknowns = groupsPage?.unknowns || []
    return { groups, unknowns, total: groups.length + unknowns.length }
  }, [groupsPage])

  const pendingTotalPages = Math.ceil(allPendingItems.total / PAGE_SIZE)
  const pendingSlice = useMemo(() => {
    const all = [
      ...allPendingItems.groups.map((g) => ({ type: 'group' as const, data: g })),
      ...allPendingItems.unknowns.map((u) => ({ type: 'unknown' as const, data: u })),
    ]
    return all.slice(pendingPage * PAGE_SIZE, (pendingPage + 1) * PAGE_SIZE)
  }, [allPendingItems, pendingPage])

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
    try { await api.triggerScan(pipelineId) } catch { /* */ }
  }

  if (!pipeline) {
    return <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
  }

  return (
    <div className="pb-24">
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
              {/* Pending pagination */}
              {pendingTotalPages > 1 && (
                <Pagination page={pendingPage} totalPages={pendingTotalPages} onPage={setPendingPage} />
              )}
            </>
          ) : (
            <div className="text-center text-gray-600 py-12">No pending groups.</div>
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
                  className={`text-xs px-2.5 py-1 rounded-lg transition ${libSort === key ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                  {key}{libSort === key && (libSortDir === 'desc' ? ' ↓' : ' ↑')}
                </button>
              ))}
              <span className="text-xs text-gray-600 ml-auto">{libData?.total} items</span>
            </div>
          )}

          {libLoading && !libData ? (
            <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
          ) : libData && libData.items.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {libData.items.map((item) => <LibraryCard key={item.id} item={item} onAction={fetchLibrary} />)}
            </div>
          ) : (
            <div className="text-center text-gray-600 py-12">No committed outputs yet.</div>
          )}

          {/* Library pagination */}
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
  return (
    <div className="flex items-center justify-center gap-3 mt-6">
      <button onClick={() => onPage(Math.max(0, page - 1))} disabled={page === 0}
        className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronLeft size={16} /></button>
      <div className="flex gap-1">
        {Array.from({ length: totalPages }, (_, i) => (
          <button key={i} onClick={() => onPage(i)}
            className={`w-8 h-8 text-xs rounded-lg transition ${i === page ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
            {i + 1}
          </button>
        ))}
      </div>
      <button onClick={() => onPage(Math.min(totalPages - 1, page + 1))} disabled={page >= totalPages - 1}
        className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronRight size={16} /></button>
    </div>
  )
}
