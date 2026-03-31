import { useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Link2, RefreshCw, ChevronLeft, ChevronRight, ArrowUpDown } from 'lucide-react'
import { pipelines, groups, library, unknowns } from '../mock/data'
import GroupCard from '../components/GroupCard'
import LibraryCard from '../components/LibraryCard'
import UnknownCard from '../components/UnknownCard'
import PipelineInfoBar from '../components/PipelineInfoBar'

const PAGE_SIZE = 12

type SortKey = 'number' | 'year' | 'rating' | 'added'

export default function PipelineDetail() {
  const { id } = useParams<{ id: string }>()
  const pipeline = pipelines.find((p) => p.id === id)
  const [tab, setTab] = useState<'pending' | 'library'>('pending')

  // Link All
  const [linkAllProgress, setLinkAllProgress] = useState<{ running: boolean; done: number; total: number } | null>(null)

  // Library pagination & sort
  const [libPage, setLibPage] = useState(0)
  const [libSort, setLibSort] = useState<SortKey>('added')
  const [libSortDir, setLibSortDir] = useState<'asc' | 'desc'>('desc')

  if (!pipeline) return <div className="text-red-400">Pipeline not found</div>

  const pGroups = groups[pipeline.id] || []
  const pLibrary = library[pipeline.id] || []
  const pUnknowns = unknowns[pipeline.id] || []

  const linkableCount = pGroups.filter(
    (g) => g.scrape.status === 'success' && g.items.every((i) => i.ready) && !g.task
  ).length

  const handleLinkAll = () => {
    setLinkAllProgress({ running: true, done: 0, total: linkableCount })
    let done = 0
    const timer = setInterval(() => {
      done++
      if (done >= linkableCount) {
        clearInterval(timer)
        setLinkAllProgress({ running: false, done: linkableCount, total: linkableCount })
        setTimeout(() => setLinkAllProgress(null), 3000)
      } else {
        setLinkAllProgress({ running: true, done, total: linkableCount })
      }
    }, 800)
  }

  // Sorted & paginated library
  const sortedLibrary = useMemo(() => {
    const sorted = [...pLibrary].sort((a, b) => {
      let cmp = 0
      switch (libSort) {
        case 'number': cmp = a.number.localeCompare(b.number); break
        case 'year': cmp = (a.year || '').localeCompare(b.year || ''); break
        case 'rating': cmp = parseFloat(a.rating || '0') - parseFloat(b.rating || '0'); break
        case 'added': cmp = a.id - b.id; break
      }
      return libSortDir === 'desc' ? -cmp : cmp
    })
    return sorted
  }, [pLibrary, libSort, libSortDir])

  const libTotalPages = Math.ceil(sortedLibrary.length / PAGE_SIZE)
  const libPageItems = sortedLibrary.slice(libPage * PAGE_SIZE, (libPage + 1) * PAGE_SIZE)

  const toggleSort = (key: SortKey) => {
    if (libSort === key) setLibSortDir(libSortDir === 'asc' ? 'desc' : 'asc')
    else { setLibSort(key); setLibSortDir('desc') }
    setLibPage(0)
  }

  return (
    <div className="pb-24">
      {/* Breadcrumb */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2 text-sm">
          <Link to="/" className="text-gray-500 hover:text-white transition">bangou</Link>
          <span className="text-gray-700">›</span>
          <span className="text-white font-medium">{pipeline.name}</span>
        </div>
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
          Library ({pLibrary.length})
        </button>
      </div>

      {tab === 'pending' && (
        <>
          {(pGroups.length > 0 || pUnknowns.length > 0) ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {pGroups.map((g) => <GroupCard key={g.number} group={g} />)}
              {pUnknowns.map((u) => <UnknownCard key={u.path} file={u} />)}
            </div>
          ) : (
            <div className="text-center text-gray-600 py-12">No pending groups.</div>
          )}
        </>
      )}

      {tab === 'library' && (
        <>
          {/* Sort controls */}
          {pLibrary.length > 0 && (
            <div className="flex items-center gap-2 mb-4">
              <ArrowUpDown size={12} className="text-gray-600" />
              {(['added', 'number', 'year', 'rating'] as SortKey[]).map((key) => (
                <button key={key} onClick={() => toggleSort(key)}
                  className={`text-xs px-2.5 py-1 rounded-lg transition ${libSort === key ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                  {key}{libSort === key && (libSortDir === 'desc' ? ' ↓' : ' ↑')}
                </button>
              ))}
              <span className="text-xs text-gray-600 ml-auto">{sortedLibrary.length} items</span>
            </div>
          )}

          {libPageItems.length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {libPageItems.map((item) => <LibraryCard key={item.id} item={item} />)}
            </div>
          ) : (
            <div className="text-center text-gray-600 py-12">No committed outputs yet.</div>
          )}

          {/* Pagination */}
          {libTotalPages > 1 && (
            <div className="flex items-center justify-center gap-3 mt-6">
              <button onClick={() => setLibPage(Math.max(0, libPage - 1))} disabled={libPage === 0}
                className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition">
                <ChevronLeft size={16} />
              </button>
              <div className="flex gap-1">
                {Array.from({ length: libTotalPages }, (_, i) => (
                  <button key={i} onClick={() => setLibPage(i)}
                    className={`w-8 h-8 text-xs rounded-lg transition ${i === libPage ? 'bg-indigo-600 text-white' : 'bg-[#1a1a1a] text-gray-500 hover:text-white border border-gray-800'}`}>
                    {i + 1}
                  </button>
                ))}
              </div>
              <button onClick={() => setLibPage(Math.min(libTotalPages - 1, libPage + 1))} disabled={libPage === libTotalPages - 1}
                className="p-2 text-gray-500 hover:text-white disabled:opacity-20 transition">
                <ChevronRight size={16} />
              </button>
            </div>
          )}
        </>
      )}

      {/* FABs - bottom right */}
      <div className="fixed bottom-6 right-6 flex flex-col gap-3 z-40">
        {/* Link All FAB */}
        {tab === 'pending' && linkableCount > 0 && (
          linkAllProgress ? (
            <div className="bg-[#1a1a1a] border border-gray-700 rounded-2xl px-4 py-3 shadow-2xl min-w-[180px]">
              {linkAllProgress.running ? (
                <div className="space-y-2">
                  <div className="text-xs text-gray-400">Linking {linkAllProgress.done + 1}/{linkAllProgress.total}</div>
                  <div className="w-full h-1.5 bg-gray-800 rounded-full overflow-hidden">
                    <div className="h-full bg-indigo-500 transition-all duration-500 rounded-full"
                      style={{ width: `${(linkAllProgress.done / linkAllProgress.total) * 100}%` }} />
                  </div>
                </div>
              ) : (
                <div className="text-xs text-emerald-400">Linked {linkAllProgress.done} groups ✓</div>
              )}
            </div>
          ) : (
            <button onClick={handleLinkAll}
              className="flex items-center gap-2 px-5 py-3 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-2xl shadow-2xl shadow-indigo-500/20 transition">
              <Link2 size={16} />Link All ({linkableCount})
            </button>
          )
        )}
        {/* Rescan FAB */}
        <button className="flex items-center gap-2 px-5 py-3 bg-[#1a1a1a] hover:bg-[#222] border border-gray-700 text-gray-300 hover:text-white text-sm rounded-2xl shadow-2xl transition">
          <RefreshCw size={16} />Rescan
        </button>
      </div>
    </div>
  )
}
