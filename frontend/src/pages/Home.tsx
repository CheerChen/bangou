import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Scan, Archive, Merge } from 'lucide-react'
import { pipelineStats } from '../mock/data'
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

  const handleCreate = (data: WizardData) => {
    alert(JSON.stringify(data, null, 2))
    setShowWizard(false)
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

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {pipelineStats.map((ps) => (
          <div key={ps.pipeline.id}
            onClick={() => navigate(`/pipelines/${ps.pipeline.id}`)}
            className="bg-[#1a1a1a] border border-gray-800 rounded-xl p-5 hover:border-gray-600 transition cursor-pointer group"
          >
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-medium text-white group-hover:text-indigo-400 transition">{ps.pipeline.name}</h2>
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${statusDot[ps.status]}`} />
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
                <code className="text-gray-400">{ps.pipeline.inputDir}</code>
              </div>
              {ps.pipeline.archiveDir && (
                <div className="flex items-center gap-1.5">
                  <Archive size={11} className="text-gray-600" />
                  <code className="text-gray-400">{ps.pipeline.archiveDir}</code>
                </div>
              )}
              <div className="flex items-center gap-1.5">
                <span className="text-gray-600 text-[11px]">→</span>
                <code className="text-gray-400">{ps.pipeline.outputDir}</code>
              </div>
            </div>

            <div className="flex items-center gap-2 text-xs">
              {ps.pipeline.autoMerge && (
                <span className="px-2 py-0.5 bg-indigo-500/10 text-indigo-400 rounded flex items-center gap-1"><Merge size={10} />Merge</span>
              )}
              {ps.pipeline.archiveDir && (
                <span className="px-2 py-0.5 bg-amber-500/10 text-amber-400 rounded flex items-center gap-1"><Archive size={10} />Archive</span>
              )}
              {ps.pipeline.providers.map((p) => (
                <span key={p} className="px-2 py-0.5 bg-gray-800 text-gray-400 rounded">{p}</span>
              ))}
            </div>
          </div>
        ))}
      </div>

      <Modal open={showWizard} onClose={() => setShowWizard(false)} title="New Pipeline" wide>
        <PipelineWizard onComplete={handleCreate} onCancel={() => setShowWizard(false)} />
      </Modal>
    </div>
  )
}
