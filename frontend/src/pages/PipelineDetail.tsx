import { useState, useCallback, useEffect } from 'react'
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

  // Library (paginated, not polling)
  const [libPage, setLibPage] = useState(0)
  const [libData, setLibData] = useState<api.LibraryPage | null>(null)
  const [libLoading, setLibLoading] = useState(false)
  const [libSort, setLibSort] = useState('added')
  const [libSortDir, setLibSortDir] = useState<'asc' | 'desc'>('desc')

  const fetchLibrary = useCallback(async () => {
    setLibLoading(true)
    try {
      const data = await api.listLibrary(pipelineId, libPage, PAGE_SIZE)
      setLibData(data)
    } catch { /* ignore */ }
    setLibLoading(false)
  }, [pipelineId, libPage])

  useEffect(() => {
    if (tab === 'library') fetchLibrary()
  }, [tab, fetchLibrary])

  // Link All
  const [laStatus, setLaStatus] = useState<api.LinkAllStatus | null>(null)

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

  const pGroups = groupsPage?.groups || []
  const pUnknowns = groupsPage?.unknowns || []
  const linkableCount = groupsPage?.linkable || 0

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
          Pending ({pGroups.length + pUnknowns.length})
        </button>
        <button onClick={() => setTab('library')}
          className={`px-4 py-2 text-sm transition ${tab === 'library' ? 'text-white border-b-2 border-indigo-500' : 'text-gray-500 hover:text-gray-300'}`}>
          Library ({libData?.total ?? pipeline.libraryCount})
        </button>
      </div>

      {tab === 'pending' && (
        <>
          {groupsLoading && pGroups.length === 0 ? (
            <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
          ) : (pGroups.length > 0 || pUnknowns.length > 0) ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {pGroups.map((g) => <GroupCard key={g.number} group={g} pipelineId={pipelineId} onAction={refreshGroups} />)}
              {pUnknowns.map((u) => <UnknownCard key={u.path} file={u} pipelineId={pipelineId} onAction={refreshGroups} />)}
            </div>
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
              {['added', 'number', 'year', 'rating'].map((key) => (
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
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {libData.items.map((item) => <LibraryCard key={item.id} item={item} onAction={fetchLibrary} />)}
            </div>
          ) : (
            <div className="text-center text-gray-600 py-12">No committed outputs yet.</div>
          )}

          {/* Pagination */}
          {libData && Math.ceil(libData.total / PAGE_SIZE) > 1 && (
            <div className="flex items-center justify-center gap-3 mt-6">
              <button onClick={() => setLibPage(Math.max(0, libPage - 1))} disabled={libPage === 0}
                className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronLeft size={16} /></button>
              <div className="flex gap-1">
                {Array.from({ length: Math.ceil(libData.total / PAGE_SIZE) }, (_, i) => (
                  <button key={i} onClick={() => setLibPage(i)}
                    className={`w-8 h-8 text-xs rounded-lg transition ${i === libPage ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                    {i + 1}
                  </button>
                ))}
              </div>
              <button onClick={() => setLibPage(Math.min(Math.ceil(libData.total / PAGE_SIZE) - 1, libPage + 1))} disabled={libPage >= Math.ceil(libData.total / PAGE_SIZE) - 1}
                className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition"><ChevronRight size={16} /></button>
            </div>
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
