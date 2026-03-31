import { Scan, Archive, Merge, Link2, Search, Download, ChevronRight, FolderOpen, CheckCircle, XCircle } from 'lucide-react'
import type { PipelineResponse } from '../api/client'

interface Props {
  pipeline: PipelineResponse
}

function Chip({ icon, label, ok }: { icon: React.ReactNode; label: string; ok?: boolean }) {
  return (
    <span className="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 bg-gray-800/60 rounded text-gray-400">
      {icon}{label}
      {ok !== undefined && (ok ? <CheckCircle size={9} className="text-emerald-400" /> : <XCircle size={9} className="text-gray-600" />)}
    </span>
  )
}

function Arrow() {
  return <ChevronRight size={16} className="text-gray-700 shrink-0 mx-1" />
}

export default function PipelineInfoBar({ pipeline }: Props) {
  const hasArchive = !!pipeline.archiveDir
  const hasAria2 = pipeline.downloadProvider === 'aria2'

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl mb-4">
      <div className="flex items-stretch">
        <div className="flex-1 p-4 space-y-2.5">
          <div className="flex items-center gap-2">
            <Scan size={14} className="text-emerald-400" />
            <span className="text-sm font-medium text-gray-200">Scan</span>
          </div>
          <code className="block text-xs text-gray-500 truncate">{pipeline.inputDir}</code>
          <div className="flex flex-wrap gap-1.5">
            <Chip icon={<Download size={9} />} label={hasAria2 ? 'aria2' : 'Manual'} ok={hasAria2 ? true : undefined} />
            {pipeline.scrapeProviders?.map((p) => {
              const needsConfig = p === 'dmm'
              return <Chip key={p} icon={<Search size={9} />} label={p === 'dmm' ? 'DMM API' : p} ok={needsConfig ? true : undefined} />
            })}
          </div>
        </div>
        <Arrow />
        <div className="flex-1 p-4 space-y-2.5 border-x border-gray-800">
          <div className="flex items-center gap-2">
            <Archive size={14} className="text-amber-400" />
            <span className="text-sm font-medium text-gray-200">Process</span>
          </div>
          <div className="space-y-1.5 text-xs">
            {hasArchive ? (
              <div className="flex items-center gap-1.5 text-gray-400"><CheckCircle size={10} className="text-emerald-400 shrink-0" />Move to <code className="text-gray-500 truncate">{pipeline.archiveDir}</code></div>
            ) : (
              <div className="flex items-center gap-1.5 text-gray-600"><XCircle size={10} className="shrink-0" />Archive off</div>
            )}
            {pipeline.enableMerge ? (
              <div className="flex items-center gap-1.5 text-gray-400"><CheckCircle size={10} className="text-emerald-400 shrink-0" />Merge</div>
            ) : (
              <div className="flex items-center gap-1.5 text-gray-600"><XCircle size={10} className="shrink-0" />Merge off</div>
            )}
          </div>
        </div>
        <Arrow />
        <div className="flex-1 p-4 space-y-2.5">
          <div className="flex items-center gap-2">
            <Link2 size={14} className="text-rose-400" />
            <span className="text-sm font-medium text-gray-200">Link To</span>
          </div>
          <div className="flex items-center gap-1.5">
            <FolderOpen size={11} className="text-gray-600 shrink-0" />
            <code className="text-xs text-gray-500 truncate">{pipeline.outputDir}</code>
          </div>
          <div className="text-[11px] text-gray-600">{pipeline.pathPattern}</div>
        </div>
      </div>
    </div>
  )
}
