import { useReducer } from 'react'
import { ChevronLeft, ChevronRight, FolderOpen, Check, CheckCircle, AlertCircle, Download, Search, Archive, Merge, Link2 } from 'lucide-react'
import DirectoryBrowser from './DirectoryBrowser'

const availableScrapers = ['avwiki', 'dmm']

interface WizardProps {
  onComplete: (data: WizardData) => void
  onCancel: () => void
  dmmConfigured: boolean
  aria2Configured: boolean
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

type State = {
  step: number
  name: string
  inputDir: string
  downloadProvider: string
  scrapers: string[]
  enableArchive: boolean
  archiveDir: string
  enableMerge: boolean
  outputDir: string
  pathPattern: string
  browseTarget: 'input' | 'archive' | 'output' | null
}

type Action =
  | { type: 'setStep'; value: number }
  | { type: 'setName'; value: string }
  | { type: 'setInputDir'; value: string }
  | { type: 'setDownloadProvider'; value: string }
  | { type: 'setScrapers'; value: string[] }
  | { type: 'toggleArchive' }
  | { type: 'setArchiveDir'; value: string }
  | { type: 'toggleMerge' }
  | { type: 'setOutputDir'; value: string }
  | { type: 'setPathPattern'; value: string }
  | { type: 'setBrowseTarget'; value: 'input' | 'archive' | 'output' | null }
  | { type: 'browseSelect'; path: string }

function initState(dmmConfigured: boolean): State {
  return {
    step: 0,
    name: '',
    inputDir: '',
    downloadProvider: 'none',
    scrapers: dmmConfigured ? ['avwiki', 'dmm'] : ['avwiki'],
    enableArchive: false,
    archiveDir: '',
    enableMerge: false,
    outputDir: '',
    pathPattern: '{Year}/{Number}',
    browseTarget: null,
  }
}

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'setStep': return { ...state, step: action.value }
    case 'setName': return { ...state, name: action.value }
    case 'setInputDir': return { ...state, inputDir: action.value }
    case 'setDownloadProvider': return { ...state, downloadProvider: action.value }
    case 'setScrapers': return { ...state, scrapers: action.value }
    case 'toggleArchive': return { ...state, enableArchive: !state.enableArchive }
    case 'setArchiveDir': return { ...state, archiveDir: action.value }
    case 'toggleMerge': return { ...state, enableMerge: !state.enableMerge }
    case 'setOutputDir': return { ...state, outputDir: action.value }
    case 'setPathPattern': return { ...state, pathPattern: action.value }
    case 'setBrowseTarget': return { ...state, browseTarget: action.value }
    case 'browseSelect': {
      const next = { ...state, browseTarget: null }
      if (state.browseTarget === 'input') next.inputDir = action.path
      else if (state.browseTarget === 'archive') next.archiveDir = action.path
      else if (state.browseTarget === 'output') next.outputDir = action.path
      return next
    }
  }
}

export default function PipelineWizard({ onComplete, onCancel, dmmConfigured, aria2Configured }: WizardProps) {
  const [state, dispatch] = useReducer(reducer, dmmConfigured, initState)
  const mkvmergeInstalled = true // mock

  const canNext = () => {
    if (state.step === 0) return state.name.trim() !== '' && state.inputDir.trim() !== ''
    if (state.step === 2) return state.outputDir.trim() !== ''
    return true
  }

  const moveScraper = (idx: number, dir: -1 | 1) => {
    const next = [...state.scrapers]
    const target = idx + dir
    if (target < 0 || target >= next.length) return
    ;[next[idx], next[target]] = [next[target], next[idx]]
    dispatch({ type: 'setScrapers', value: next })
  }

  const toggleScraper = (s: string) => {
    if (s === 'dmm' && !dmmConfigured) return
    if (state.scrapers.includes(s)) {
      if (state.scrapers.length > 1) dispatch({ type: 'setScrapers', value: state.scrapers.filter((x) => x !== s) })
    } else {
      dispatch({ type: 'setScrapers', value: [...state.scrapers, s] })
    }
  }

  const handleFinish = () => {
    onComplete({ name: state.name, inputDir: state.inputDir, outputDir: state.outputDir, pathPattern: state.pathPattern, archiveDir: state.enableArchive ? state.archiveDir : '', enableMerge: state.enableMerge, scrapers: state.scrapers, downloadProvider: state.downloadProvider })
  }

  const getBrowseInitialPath = () => {
    if (state.browseTarget === 'input' && state.inputDir) return state.inputDir
    if (state.browseTarget === 'archive' && state.archiveDir) return state.archiveDir
    if (state.browseTarget === 'output' && state.outputDir) return state.outputDir
    return '/'
  }

  return (
    <div>
      <StepIndicator step={state.step} />

      {state.step === 0 && (
        <SourcesStep
          state={state}
          dispatch={dispatch}
          dmmConfigured={dmmConfigured}
          aria2Configured={aria2Configured}
          moveScraper={moveScraper}
          toggleScraper={toggleScraper}
        />
      )}

      {state.step === 1 && (
        <ProcessingStep
          state={state}
          dispatch={dispatch}
          mkvmergeInstalled={mkvmergeInstalled}
        />
      )}

      {state.step === 2 && <OutputStep state={state} dispatch={dispatch} />}

      {state.step === 3 && <ConfirmStep state={state} />}

      {/* Navigation */}
      <div className="flex items-center justify-between mt-6 pt-4 border-t border-gray-800">
        <button type="button" onClick={state.step === 0 ? onCancel : () => dispatch({ type: 'setStep', value: state.step - 1 })}
          className="flex items-center gap-1 text-sm text-gray-400 hover:text-white transition">
          <ChevronLeft size={16} />{state.step === 0 ? 'Cancel' : 'Back'}
        </button>
        {state.step < STEPS.length - 1 ? (
          <button type="button" onClick={() => dispatch({ type: 'setStep', value: state.step + 1 })} disabled={!canNext()}
            className="flex items-center gap-1 text-sm px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-lg transition">
            Next<ChevronRight size={16} />
          </button>
        ) : (
          <button type="button" onClick={handleFinish}
            className="flex items-center gap-1 text-sm px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg transition">
            <Check size={16} />Create Pipeline
          </button>
        )}
      </div>

      {/* Directory Browser */}
      <DirectoryBrowser
        open={state.browseTarget !== null}
        onClose={() => dispatch({ type: 'setBrowseTarget', value: null })}
        onSelect={(path) => dispatch({ type: 'browseSelect', path })}
        initialPath={getBrowseInitialPath()}
      />
    </div>
  )
}

type Dispatch = (action: Action) => void

function StepIndicator({ step }: { step: number }) {
  return (
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
  )
}

function SourcesStep({ state, dispatch, dmmConfigured, aria2Configured, moveScraper, toggleScraper }: {
  state: State
  dispatch: Dispatch
  dmmConfigured: boolean
  aria2Configured: boolean
  moveScraper: (idx: number, dir: -1 | 1) => void
  toggleScraper: (s: string) => void
}) {
  return (
    <div className="space-y-5">
      <div>
        <label htmlFor="pw-name" className="block text-xs text-gray-500 mb-1">Pipeline Name</label>
        <input id="pw-name" type="text" value={state.name} onChange={(e) => dispatch({ type: 'setName', value: e.target.value })} placeholder="VR Downloads"
          className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
      </div>

      <div>
        <label htmlFor="pw-input" className="block text-xs text-gray-500 mb-1">Scan Directory</label>
        <div className="flex gap-2">
          <input id="pw-input" type="text" value={state.inputDir} onChange={(e) => dispatch({ type: 'setInputDir', value: e.target.value })} placeholder="/download/VR"
            className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          <button type="button" onClick={() => dispatch({ type: 'setBrowseTarget', value: 'input' })} aria-label="Browse scan directory"
            className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
            <FolderOpen size={16} />
          </button>
        </div>
      </div>

      {/* Download Provider */}
      <div>
        <span id="pw-dl-label" className="block text-xs text-gray-500 mb-2 flex items-center gap-1"><Download size={11} />Download Provider</span>
        <div className="space-y-2">
          <label className="flex items-center gap-3 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5 cursor-pointer hover:border-gray-700 transition">
            <input type="radio" name="dl" value="none" checked={state.downloadProvider === 'none'} onChange={() => dispatch({ type: 'setDownloadProvider', value: 'none' })}
              className="text-indigo-500 focus:ring-indigo-500" />
            <div className="flex-1">
              <span className="text-sm text-white">None</span>
              <span className="text-xs text-gray-600 ml-2">Files are placed in scan directory manually</span>
            </div>
          </label>
          <label className={`flex items-center gap-3 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5 transition ${
            aria2Configured ? 'cursor-pointer hover:border-gray-700' : 'opacity-40 cursor-not-allowed'
          }`}>
            <input type="radio" name="dl" value="aria2"
              checked={state.downloadProvider === 'aria2'}
              onChange={() => aria2Configured && dispatch({ type: 'setDownloadProvider', value: 'aria2' })}
              disabled={!aria2Configured}
              className="text-indigo-500 focus:ring-indigo-500" />
            <div className="flex-1">
              <span className="text-sm text-white">aria2</span>
              <span className="text-xs text-gray-600 ml-2">Monitor download progress via RPC</span>
            </div>
            {aria2Configured
              ? <span title="Configured" className="text-emerald-400"><CheckCircle size={14} /></span>
              : <span title="Not configured" className="text-amber-500"><AlertCircle size={14} /></span>
            }
          </label>
        </div>
      </div>

      {/* Scrape Providers */}
      <div>
        <span id="pw-scrape-label" className="block text-xs text-gray-500 mb-2 flex items-center gap-1"><Search size={11} />Scrape Providers</span>
        <div className="space-y-2">
          {state.scrapers.map((s, i) => (
            <div key={s} className="flex items-center gap-2 bg-[#111] border border-gray-800 rounded-lg px-3 py-2.5">
              <span className="text-xs text-gray-600 w-4">{i + 1}.</span>
              <span className="flex-1 text-sm text-white">{s}</span>
              {s === 'dmm' && (
                dmmConfigured
                  ? <span title="Configured" className="text-emerald-400"><CheckCircle size={14} /></span>
                  : <span title="Not configured" className="text-amber-500"><AlertCircle size={14} /></span>
              )}
              <button type="button" onClick={() => moveScraper(i, -1)} disabled={i === 0} aria-label="Move scraper up"
                className="text-gray-600 hover:text-white disabled:opacity-20 text-xs p-1">&#x2191;</button>
              <button type="button" onClick={() => moveScraper(i, 1)} disabled={i === state.scrapers.length - 1} aria-label="Move scraper down"
                className="text-gray-600 hover:text-white disabled:opacity-20 text-xs p-1">&#x2193;</button>
              <button type="button" onClick={() => toggleScraper(s)} aria-label={`Remove ${s} scraper`}
                className="text-gray-600 hover:text-red-400 text-xs p-1">&#x00D7;</button>
            </div>
          ))}
          {availableScrapers.reduce<React.ReactNode[]>((acc, s) => {
            if (!state.scrapers.includes(s)) {
              const disabled = s === 'dmm' && !dmmConfigured
              acc.push(
                <button key={s} type="button" onClick={() => toggleScraper(s)} disabled={disabled}
                  className={`text-xs px-3 py-1.5 border border-dashed rounded-lg transition ${
                    disabled
                      ? 'border-gray-800 text-gray-700 cursor-not-allowed'
                      : 'border-gray-700 text-gray-500 hover:text-white'
                  }`}>
                  + {s}{disabled && ' (not configured)'}
                </button>
              )
            }
            return acc
          }, [])}
        </div>
      </div>
    </div>
  )
}

function ProcessingStep({ state, dispatch, mkvmergeInstalled }: {
  state: State
  dispatch: Dispatch
  mkvmergeInstalled: boolean
}) {
  return (
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
          <button type="button" onClick={() => dispatch({ type: 'toggleArchive' })} aria-label="Toggle archive" aria-pressed={state.enableArchive}
            className={`w-10 h-6 rounded-full transition-colors relative ${state.enableArchive ? 'bg-indigo-600' : 'bg-gray-700'}`}>
            <div className={`absolute top-0.5 w-5 h-5 bg-white rounded-full transition-transform ${state.enableArchive ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
          </button>
        </div>
        {state.enableArchive && (
          <div className="mt-3 flex gap-2">
            <input type="text" value={state.archiveDir} onChange={(e) => dispatch({ type: 'setArchiveDir', value: e.target.value })} placeholder="/archive/VR" aria-label="Archive directory"
              className="flex-1 px-3 py-2 bg-[#0f0f0f] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            <button type="button" onClick={() => dispatch({ type: 'setBrowseTarget', value: 'archive' })} aria-label="Browse archive directory"
              className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
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
          <button type="button" onClick={() => dispatch({ type: 'toggleMerge' })} aria-label="Toggle merge" aria-pressed={state.enableMerge}
            className={`w-10 h-6 rounded-full transition-colors relative ${state.enableMerge ? 'bg-indigo-600' : 'bg-gray-700'}`}>
            <div className={`absolute top-0.5 w-5 h-5 bg-white rounded-full transition-transform ${state.enableMerge ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
          </button>
        </div>
        {state.enableMerge && (
          <div className="mt-3 flex items-center gap-2 text-xs">
            {mkvmergeInstalled
              ? <span className="flex items-center gap-1 text-emerald-400"><CheckCircle size={12} />mkvmerge installed</span>
              : <span className="flex items-center gap-1 text-red-400"><AlertCircle size={12} />mkvmerge not found — install MKVToolNix</span>
            }
          </div>
        )}
      </div>
    </div>
  )
}

function OutputStep({ state, dispatch }: { state: State; dispatch: Dispatch }) {
  return (
    <div className="space-y-4">
      <div>
        <label htmlFor="pw-output" className="block text-xs text-gray-500 mb-1 flex items-center gap-1"><Link2 size={11} />Link To</label>
        <div className="flex gap-2">
          <input id="pw-output" type="text" value={state.outputDir} onChange={(e) => dispatch({ type: 'setOutputDir', value: e.target.value })} placeholder="/media/VR"
            className="flex-1 px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
          <button type="button" onClick={() => dispatch({ type: 'setBrowseTarget', value: 'output' })} aria-label="Browse link target directory"
            className="px-3 py-2 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
            <FolderOpen size={16} />
          </button>
        </div>
        <p className="text-xs text-gray-600 mt-1">Media library directory where links will be created</p>
      </div>
      <div>
        <label htmlFor="pw-pattern" className="block text-xs text-gray-500 mb-1">Path Pattern</label>
        <input id="pw-pattern" type="text" value={state.pathPattern} onChange={(e) => dispatch({ type: 'setPathPattern', value: e.target.value })}
          className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
        <p className="text-xs text-gray-600 mt-1">Variables: {'{Year}'}, {'{Actor}'}, {'{Number}'}, {'{Series}'}</p>
      </div>
    </div>
  )
}

function ConfirmStep({ state }: { state: State }) {
  return (
    <div className="bg-[#111] border border-gray-800 rounded-xl p-4 space-y-2.5 text-sm">
      <div className="flex justify-between"><span className="text-gray-500">Name</span><span className="text-white">{state.name}</span></div>
      <div className="flex justify-between"><span className="text-gray-500">Scan</span><code className="text-gray-300">{state.inputDir}</code></div>
      <div className="flex justify-between"><span className="text-gray-500">Download</span><span className="text-gray-300">{state.downloadProvider === 'none' ? 'Manual' : 'aria2'}</span></div>
      <div className="flex justify-between"><span className="text-gray-500">Scrapers</span><span className="text-gray-300">{state.scrapers.join(' \u2192 ')}</span></div>
      <div className="border-t border-gray-800 my-1" />
      <div className="flex justify-between"><span className="text-gray-500">Archive</span><span className={state.enableArchive ? 'text-gray-300' : 'text-gray-600'}>{state.enableArchive ? state.archiveDir : 'Off'}</span></div>
      <div className="flex justify-between"><span className="text-gray-500">Merge</span><span className={state.enableMerge ? 'text-emerald-400' : 'text-gray-600'}>{state.enableMerge ? 'Enabled' : 'Off'}</span></div>
      <div className="border-t border-gray-800 my-1" />
      <div className="flex justify-between"><span className="text-gray-500">Link To</span><code className="text-gray-300">{state.outputDir}</code></div>
      <div className="flex justify-between"><span className="text-gray-500">Pattern</span><code className="text-gray-300">{state.pathPattern}</code></div>
    </div>
  )
}
