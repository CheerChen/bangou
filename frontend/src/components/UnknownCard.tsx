import { HelpCircle, Tag, EyeOff } from 'lucide-react'
import { useState } from 'react'
import type { UnknownFile } from '../types'

export default function UnknownCard({ file }: { file: UnknownFile }) {
  const [number, setNumber] = useState('')

  return (
    <div className="bg-[#1a1a1a] border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition">
      {/* Cover-like area with ? */}
      <div className="relative aspect-[16/9] overflow-hidden bg-[#111] flex items-center justify-center">
        <HelpCircle size={48} className="text-gray-800" />
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
        <div className="absolute bottom-3 left-3 right-3 flex items-end justify-between">
          <span className="text-white font-semibold text-sm truncate max-w-[70%]">{file.filename}</span>
          <span className="text-xs text-gray-500">{file.sizeGB.toFixed(2)} GB</span>
        </div>
      </div>

      <div className="p-4 space-y-3">
        <div className="text-xs text-gray-500">Unable to parse a number from this filename.</div>

        <div className="flex items-center gap-2">
          <input
            type="text"
            value={number}
            onChange={(e) => setNumber(e.target.value)}
            placeholder="e.g. SIVR-476"
            className="flex-1 px-3 py-1.5 bg-[#111] border border-gray-700 rounded-lg text-white text-sm placeholder-gray-700 focus:border-indigo-500 focus:outline-none"
          />
          <button className="flex items-center gap-1 text-xs px-2.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg transition">
            <Tag size={12} />Tag
          </button>
          <button className="p-1.5 text-gray-600 hover:text-red-400 rounded-lg transition" title="Ignore">
            <EyeOff size={14} />
          </button>
        </div>
      </div>
    </div>
  )
}
