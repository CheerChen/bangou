import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Scan, Archive, Merge, Loader2 } from 'lucide-react'
import { listPipelines, createPipeline } from '../api/client'
import type { PipelineResponse, CreatePipelineReq } from '../api/client'
import { usePolling } from '../api/usePolling'
import Modal from '../components/Modal'
import PipelineWizard from '../components/PipelineWizard'
import type { WizardData } from '../components/PipelineWizard'

const statusDot: Record<string, string> = {
  idle: 'bg-gray-500',
  scanning: 'bg-emerald-400 animate-pulse',
  linking: 'bg-indigo-400 animate-pulse',
}

export default function Home() {
  const navigate = useNavigate()
  const [showWizard, setShowWizard] = useState(false)

  const fetcher = useCallback(() => listPipelines(), [])
  const { data: pipes, loading, refresh } = usePolling(fetcher, 5000)

  const handleCreate = async (data: WizardData) => {
    const req: CreatePipelineReq = {
      name: data.name,
      inputDir: data.inputDir,
      outputDir: data.outputDir,
      pathPattern: data.pathPattern,
      archiveDir: data.archiveDir,
      enableMerge: data.enableMerge,
      downloadProvider: data.downloadProvider,
      scrapeProviders: data.scrapers,
    }
    try {
      await createPipeline(req)
      setShowWizard(false)
      refresh()
    } catch (e: any) {
      alert(e.message)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold text-white">Pipelines</h1>
        <button onClick={() => setShowWizard(true)}
          className="flex items-center gap-1.5 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">
          <Plus size={14} />New Pipeline
        </button>
      </div>

      {loading && !pipes ? (
        <div className="flex justify-center py-12"><Loader2 size={24} className="animate-spin text-gray-500" /></div>
      ) : pipes && pipes.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {pipes.map((ps: PipelineResponse) => (
            <div key={ps.id}
              onClick={() => navigate(`/pipelines/${ps.id}`)}
              className="bg-[#1a1a1a] border border-gray-800 rounded-xl p-5 hover:border-gray-600 transition cursor-pointer group relative"
            >
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-medium text-white group-hover:text-indigo-400 transition">{ps.name}</h2>
                <div className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${statusDot[ps.status] || statusDot.idle}`} />
                  <span className="text-xs text-gray-500">{ps.status}</span>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3 mb-4">
                <div className="bg-[#111] rounded-lg p-3">
                  <div className="text-2xl font-semibold text-white">{ps.pendingCount}</div>
                  <div className="text-xs text-gray-500">Pending</div>
                </div>
                <div className="bg-[#111] rounded-lg p-3">
                  <div className="text-2xl font-semibold text-white">{ps.libraryCount}</div>
                  <div className="text-xs text-gray-500">Library</div>
                </div>
              </div>

              <div className="text-xs text-gray-500 space-y-1 mb-4">
                <div className="flex items-center gap-1.5">
                  <Scan size={11} className="text-gray-600" />
                  <code className="text-gray-400">{ps.inputDir}</code>
                </div>
                {ps.archiveDir && (
                  <div className="flex items-center gap-1.5">
                    <Archive size={11} className="text-gray-600" />
                    <code className="text-gray-400">{ps.archiveDir}</code>
                  </div>
                )}
                <div className="flex items-center gap-1.5">
                  <span className="text-gray-600 text-[11px]">→</span>
                  <code className="text-gray-400">{ps.outputDir}</code>
                </div>
              </div>

              <div className="flex items-center gap-2 text-xs">
                {ps.enableMerge && (
                  <span className="px-2 py-0.5 bg-indigo-500/10 text-indigo-400 rounded flex items-center gap-1"><Merge size={10} />Merge</span>
                )}
                {ps.archiveDir && (
                  <span className="px-2 py-0.5 bg-amber-500/10 text-amber-400 rounded flex items-center gap-1"><Archive size={10} />Archive</span>
                )}
                {ps.scrapeProviders?.map((p: string) => (
                  <span key={p} className="px-2 py-0.5 bg-gray-800 text-gray-400 rounded">{p}</span>
                ))}
              </div>

            </div>
          ))}
        </div>
      ) : (
        <div className="text-center py-12">
          <p className="text-gray-500 mb-4">No pipelines configured yet.</p>
          <button onClick={() => setShowWizard(true)}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">
            Create your first pipeline
          </button>
        </div>
      )}

      <Modal open={showWizard} onClose={() => setShowWizard(false)} title="New Pipeline" wide>
        <PipelineWizard onComplete={handleCreate} onCancel={() => setShowWizard(false)} />
      </Modal>
    </div>
  )
}
