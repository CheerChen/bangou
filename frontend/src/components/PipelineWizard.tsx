import { useState } from 'react'
import { ChevronLeft, ChevronRight, FolderOpen, Check, Settings2, CheckCircle, AlertCircle, Loader2, Download, Search, Archive, Merge, Link2 } from 'lucide-react'
import Modal from './Modal'
import * as api from '../api/client'

const availableScrapers = ['avwiki', 'dmm']

interface WizardProps {
  onComplete: (data: WizardData) => void
  onCancel: () => void
}

export interface WizardData {
  name: string
  inputDir: string
  outputDir: string
  pathPattern: string
  archiveDir: string
  enableMerge: boolean
  scrapers: string[]
  downloadProvider: string
}

const STEPS = ['Sources', 'Processing', 'Output', 'Confirm']

export default function PipelineWizard({ onComplete, onCancel }: WizardProps) {
  const [step, setStep] = useState(0)

  // Step 0: Sources
  const [name, setName] = useState('')
  const [inputDir, setInputDir] = useState('')
  const [downloadProvider, setDownloadProvider] = useState('none')
  const [showAria2Settings, setShowAria2Settings] = useState(false)
  const [aria2Url, setAria2Url] = useState('http://localhost:6800/jsonrpc')
  const [aria2Token, setAria2Token] = useState('')
  const [aria2Test, setAria2Test] = useState<'idle' | 'testing' | 'ok' | 'fail'>('idle')
  const [scrapers, setScrapers] = useState<string[]>(['avwiki', 'dmm'])
  const [showDmmSettings, setShowDmmSettings] = useState(false)
  const [dmmApiId, setDmmApiId] = useState('')
  const [dmmAffId, setDmmAffId] = useState('')
  const [dmmTest, setDmmTest] = useState<'idle' | 'testing' | 'ok' | 'fail'>('idle')

  // Step 1: Processing
  const [enableArchive, setEnableArchive] = useState(false)
  const [archiveDir, setArchiveDir] = useState('')
  const [enableMerge, setEnableMerge] = useState(false)
  const mkvmergeInstalled = true // mock

  // Step 2: Output
  const [outputDir, setOutputDir] = useState('')
  const [pathPattern, setPathPattern] = useState('{Year}/{Number}')

  const canNext = () => {
    if (step === 0) return name.trim() !== '' && inputDir.trim() !== ''
    if (step === 2) return outputDir.trim() !== ''
    return true
  }

  const moveScraper = (idx: number, dir: -1 | 1) => {
    const next = [...scrapers]
    const target = idx + dir
    if (target < 0 || target >= next.length) return
    ;[next[idx], next[target]] = [next[target], next[idx]]
    setScrapers(next)
  }

  const toggleScraper = (s: string) => {
    if (scrapers.includes(s)) {
      if (scrapers.length > 1) setScrapers(scrapers.filter((x) => x !== s))
    } else {
      setScrapers([...scrapers, s])
    }
  }

  const testAria2 = async () => {
    setAria2Test('testing')
    try {
      await api.setProviderConfig('aria2', { rpc_url: aria2Url, token: aria2Token })
      await api.testProviderConfig('aria2')
      setAria2Test('ok')
    } catch { setAria2Test('fail') }
  }
  const testDmm = async () => {
    setDmmTest('testing')
    try {
      await api.setProviderConfig('dmm', { api_id: dmmApiId, affiliate_id: dmmAffId })
      await api.testProviderConfig('dmm')
      setDmmTest('ok')
    } catch { setDmmTest('fail') }
  }

  const handleFinish = () => {
    onComplete({ name, inputDir, outputDir, pathPattern, archiveDir: enableArchive ? archiveDir : '', enableMerge, scrapers, downloadProvider })
  }

  return (
    <div>
      {/* Step indicator */}
      <div className="flex items-center gap-2 mb-6">
        {STEPS.map((s, i) => (
          <div key={s} className="flex items-center gap-2">
            {i > 0 && <div className={`w-8 h-px ${i <= step ? 'bg-indigo-500' : 'bg-gray-700'}`} />}
            <div className={`flex items-center gap-1.5 text-xs ${i === step ? 'text-indigo-400' : i < step ? 'text-emerald-400' : 'text-gray-600'}`}>
              <div className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-medium ${
                i === step ? 'bg-indigo-600 text-white' : i < step ? 'bg-emerald-600 text-white' : 'bg-gray-800 text-gray-600'
              }`}>
                {i < step ? <Check size={10} /> : i + 1}
              </div>
              {s}
            </div>
          </div>
        ))}
      </div>

      {/* Step 0: Sources */}
      {step === 0 && (
        <div className="space-y-5">
          <div>
            <label className="block text-xs text-gray-500 mb-1">Pipeline Name</label>
            <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="VR Downloads"
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          </div>

          <div>
            <label className="block text-xs text-gray-500 mb-1">Scan Directory</label>
            <div className="flex gap-2">
              <input type="text" value={inputDir} onChange={(e) => setInputDir(e.target.value)} placeholder="/download/VR"
                className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
              <button type="button" className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                <FolderOpen size={16} />
              </button>
            </div>
          </div>

          {/* Download Provider */}
          <div>
            <label className="block text-xs text-gray-500 mb-2 flex items-center gap-1"><Download size={11} />Download Provider</label>
            <div className="space-y-2">
              <label className="flex items-center gap-3 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5 cursor-pointer hover:border-gray-700 transition">
                <input type="radio" name="dl" value="none" checked={downloadProvider === 'none'} onChange={() => setDownloadProvider('none')}
                  className="text-indigo-500 focus:ring-indigo-500" />
                <div className="flex-1">
                  <span className="text-sm text-white">None</span>
                  <span className="text-xs text-gray-600 ml-2">Files are placed in scan directory manually</span>
                </div>
              </label>
              <label className="flex items-center gap-3 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5 cursor-pointer hover:border-gray-700 transition">
                <input type="radio" name="dl" value="aria2" checked={downloadProvider === 'aria2'} onChange={() => setDownloadProvider('aria2')}
                  className="text-indigo-500 focus:ring-indigo-500" />
                <div className="flex-1">
                  <span className="text-sm text-white">aria2</span>
                  <span className="text-xs text-gray-600 ml-2">Monitor download progress via RPC</span>
                </div>
                {downloadProvider === 'aria2' && (
                  <button type="button" onClick={(e) => { e.preventDefault(); setShowAria2Settings(true) }}
                    className="p-1.5 text-gray-500 hover:text-white rounded-lg transition">
                    <Settings2 size={14} />
                  </button>
                )}
              </label>
            </div>
          </div>

          {/* Scrape Providers */}
          <div>
            <label className="block text-xs text-gray-500 mb-2 flex items-center gap-1"><Search size={11} />Scrape Providers</label>
            <div className="space-y-2">
              {scrapers.map((s, i) => (
                <div key={s} className="flex items-center gap-2 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5">
                  <span className="text-xs text-gray-600 w-4">{i + 1}.</span>
                  <span className="flex-1 text-sm text-white">{s}</span>
                  {s === 'dmm' && (
                    <button type="button" onClick={() => setShowDmmSettings(true)}
                      className="p-1.5 text-gray-500 hover:text-white rounded-lg transition">
                      <Settings2 size={14} />
                    </button>
                  )}
                  <button type="button" onClick={() => moveScraper(i, -1)} disabled={i === 0}
                    className="text-gray-600 hover:text-white disabled:opacity-20 text-xs p-1">↑</button>
                  <button type="button" onClick={() => moveScraper(i, 1)} disabled={i === scrapers.length - 1}
                    className="text-gray-600 hover:text-white disabled:opacity-20 text-xs p-1">↓</button>
                  <button type="button" onClick={() => toggleScraper(s)}
                    className="text-gray-600 hover:text-red-400 text-xs p-1">×</button>
                </div>
              ))}
              {availableScrapers.filter((s) => !scrapers.includes(s)).map((s) => (
                <button key={s} type="button" onClick={() => toggleScraper(s)}
                  className="text-xs px-3 py-1.5 border border-dashed border-gray-700 text-gray-500 hover:text-white rounded-lg transition">
                  + {s}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Step 1: Processing */}
      {step === 1 && (
        <div className="space-y-3">
          <div className="bg-[#111] border border-gray-800 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Archive size={14} className="text-amber-400" />
                <div>
                  <div className="text-sm text-white">Move to Archive</div>
                  <div className="text-xs text-gray-500">Move source files to a separate directory before linking</div>
                </div>
              </div>
              <button onClick={() => setEnableArchive(!enableArchive)}
                className={`w-10 h-6 rounded-full transition-colors relative ${enableArchive ? 'bg-indigo-600' : 'bg-gray-700'}`}>
                <div className={`absolute top-0.5 w-5 h-5 bg-white rounded-full transition-transform ${enableArchive ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
              </button>
            </div>
            {enableArchive && (
              <div className="mt-3 flex gap-2">
                <input type="text" value={archiveDir} onChange={(e) => setArchiveDir(e.target.value)} placeholder="/archive/VR"
                  className="flex-1 px-3 py-2 bg-[#0f0f0f] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
                <button type="button" className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                  <FolderOpen size={16} />
                </button>
              </div>
            )}
          </div>

          <div className="bg-[#111] border border-gray-800 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Merge size={14} className="text-indigo-400" />
                <div>
                  <div className="text-sm text-white">Merge</div>
                  <div className="text-xs text-gray-500">Enable merging multi-part files (requires manual confirmation)</div>
                </div>
              </div>
              <button onClick={() => setEnableMerge(!enableMerge)}
                className={`w-10 h-6 rounded-full transition-colors relative ${enableMerge ? 'bg-indigo-600' : 'bg-gray-700'}`}>
                <div className={`absolute top-0.5 w-5 h-5 bg-white rounded-full transition-transform ${enableMerge ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
              </button>
            </div>
            {enableMerge && (
              <div className="mt-3 flex items-center gap-2 text-xs">
                {mkvmergeInstalled
                  ? <span className="flex items-center gap-1 text-emerald-400"><CheckCircle size={12} />mkvmerge installed</span>
                  : <span className="flex items-center gap-1 text-red-400"><AlertCircle size={12} />mkvmerge not found — install MKVToolNix</span>
                }
              </div>
            )}
          </div>
        </div>
      )}

      {/* Step 2: Output */}
      {step === 2 && (
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-gray-500 mb-1 flex items-center gap-1"><Link2 size={11} />Link To</label>
            <div className="flex gap-2">
              <input type="text" value={outputDir} onChange={(e) => setOutputDir(e.target.value)} placeholder="/media/VR"
                className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
              <button type="button" className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                <FolderOpen size={16} />
              </button>
            </div>
            <p className="text-xs text-gray-600 mt-1">Media library directory where links will be created</p>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Path Pattern</label>
            <input type="text" value={pathPattern} onChange={(e) => setPathPattern(e.target.value)}
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            <p className="text-xs text-gray-600 mt-1">Variables: {'{Year}'}, {'{Actor}'}, {'{Number}'}, {'{Series}'}</p>
          </div>
        </div>
      )}

      {/* Step 3: Confirm */}
      {step === 3 && (
        <div className="bg-[#111] border border-gray-800 rounded-xl p-4 space-y-2.5 text-sm">
          <div className="flex justify-between"><span className="text-gray-500">Name</span><span className="text-white">{name}</span></div>
          <div className="flex justify-between"><span className="text-gray-500">Scan</span><code className="text-gray-300">{inputDir}</code></div>
          <div className="flex justify-between"><span className="text-gray-500">Download</span><span className="text-gray-300">{downloadProvider === 'none' ? 'Manual' : 'aria2'}</span></div>
          <div className="flex justify-between"><span className="text-gray-500">Scrapers</span><span className="text-gray-300">{scrapers.join(' → ')}</span></div>
          <div className="border-t border-gray-800 my-1" />
          <div className="flex justify-between"><span className="text-gray-500">Archive</span><span className={enableArchive ? 'text-gray-300' : 'text-gray-600'}>{enableArchive ? archiveDir : 'Off'}</span></div>
          <div className="flex justify-between"><span className="text-gray-500">Merge</span><span className={enableMerge ? 'text-emerald-400' : 'text-gray-600'}>{enableMerge ? 'Enabled' : 'Off'}</span></div>
          <div className="border-t border-gray-800 my-1" />
          <div className="flex justify-between"><span className="text-gray-500">Link To</span><code className="text-gray-300">{outputDir}</code></div>
          <div className="flex justify-between"><span className="text-gray-500">Pattern</span><code className="text-gray-300">{pathPattern}</code></div>
        </div>
      )}

      {/* Navigation */}
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-gray-800">
        <button onClick={step === 0 ? onCancel : () => setStep(step - 1)}
          className="flex items-center gap-1 text-sm text-gray-400 hover:text-white transition">
          <ChevronLeft size={16} />{step === 0 ? 'Cancel' : 'Back'}
        </button>
        {step < STEPS.length - 1 ? (
          <button onClick={() => setStep(step + 1)} disabled={!canNext()}
            className="flex items-center gap-1 text-sm px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-lg transition">
            Next<ChevronRight size={16} />
          </button>
        ) : (
          <button onClick={handleFinish}
            className="flex items-center gap-1 text-sm px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg transition">
            <Check size={16} />Create Pipeline
          </button>
        )}
      </div>

      {/* aria2 Settings Modal */}
      <Modal open={showAria2Settings} onClose={() => setShowAria2Settings(false)} title="aria2 Settings">
        <div className="space-y-3">
          <div>
            <label className="block text-xs text-gray-500 mb-1">RPC URL</label>
            <input type="text" value={aria2Url} onChange={(e) => setAria2Url(e.target.value)}
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Secret Token</label>
            <input type="password" value={aria2Token} onChange={(e) => setAria2Token(e.target.value)} placeholder="aria2 secret"
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          </div>
          <div className="flex items-center gap-2">
            <button onClick={testAria2} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
              {aria2Test === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
            </button>
            {aria2Test === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
            {aria2Test === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
          </div>
          <div className="pt-3 border-t border-gray-800">
            <button onClick={() => setShowAria2Settings(false)} className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">Done</button>
          </div>
        </div>
      </Modal>

      {/* DMM Settings Modal */}
      <Modal open={showDmmSettings} onClose={() => setShowDmmSettings(false)} title="DMM API Settings">
        <div className="space-y-3">
          <div>
            <label className="block text-xs text-gray-500 mb-1">API ID</label>
            <input type="text" value={dmmApiId} onChange={(e) => setDmmApiId(e.target.value)} placeholder="Your DMM API ID"
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Affiliate ID</label>
            <input type="password" value={dmmAffId} onChange={(e) => setDmmAffId(e.target.value)} placeholder="Your Affiliate ID"
              className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          </div>
          <div className="flex items-center gap-2">
            <button onClick={testDmm} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
              {dmmTest === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
            </button>
            {dmmTest === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
            {dmmTest === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
          </div>
          <div className="pt-3 border-t border-gray-800">
            <button onClick={() => setShowDmmSettings(false)} className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">Done</button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
