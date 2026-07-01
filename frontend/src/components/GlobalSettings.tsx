import { useReducer, useEffect } from 'react'
import { CheckCircle, AlertCircle, Loader2, Download, Search, Eye, EyeOff } from 'lucide-react'
import Modal from './Modal'
import * as api from '../api/client'

interface GlobalSettingsProps {
  open: boolean
  onClose: () => void
  onConfigChanged: () => void
}

type TestState = 'idle' | 'testing' | 'ok' | 'fail'

type State = {
  aria2Url: string
  aria2Token: string
  aria2Test: TestState
  dmmApiId: string
  dmmAffId: string
  dmmTest: TestState
  showAria2Token: boolean
  showDmmAffId: boolean
}

type Action =
  | { type: 'setAria2Url'; value: string }
  | { type: 'setAria2Token'; value: string }
  | { type: 'setAria2Test'; value: TestState }
  | { type: 'setDmmApiId'; value: string }
  | { type: 'setDmmAffId'; value: string }
  | { type: 'setDmmTest'; value: TestState }
  | { type: 'toggleShowAria2Token' }
  | { type: 'toggleShowDmmAffId' }
  | { type: 'load'; aria2Url?: string; aria2Token?: string; dmmApiId?: string; dmmAffId?: string }

const initialState: State = {
  aria2Url: 'http://localhost:6800/jsonrpc',
  aria2Token: '',
  aria2Test: 'idle',
  dmmApiId: '',
  dmmAffId: '',
  dmmTest: 'idle',
  showAria2Token: false,
  showDmmAffId: false,
}

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'setAria2Url': return { ...state, aria2Url: action.value, aria2Test: 'idle' }
    case 'setAria2Token': return { ...state, aria2Token: action.value, aria2Test: 'idle' }
    case 'setAria2Test': return { ...state, aria2Test: action.value }
    case 'setDmmApiId': return { ...state, dmmApiId: action.value, dmmTest: 'idle' }
    case 'setDmmAffId': return { ...state, dmmAffId: action.value, dmmTest: 'idle' }
    case 'setDmmTest': return { ...state, dmmTest: action.value }
    case 'toggleShowAria2Token': return { ...state, showAria2Token: !state.showAria2Token }
    case 'toggleShowDmmAffId': return { ...state, showDmmAffId: !state.showDmmAffId }
    case 'load': return {
      ...state,
      aria2Url: action.aria2Url || state.aria2Url,
      aria2Token: action.aria2Token || state.aria2Token,
      dmmApiId: action.dmmApiId || state.dmmApiId,
      dmmAffId: action.dmmAffId || state.dmmAffId,
    }
  }
}

export default function GlobalSettings({ open, onClose, onConfigChanged }: GlobalSettingsProps) {
  const [state, dispatch] = useReducer(reducer, initialState)

  useEffect(() => {
    if (!open) return
    api.listProviderConfigs().then((configs) => {
      let aria2Url: string | undefined
      let aria2Token: string | undefined
      let dmmApiId: string | undefined
      let dmmAffId: string | undefined
      for (const c of configs) {
        try {
          const parsed = JSON.parse(c.config)
          if (c.provider === 'aria2') {
            if (parsed.rpc_url) aria2Url = parsed.rpc_url
            if (parsed.token) aria2Token = parsed.token
          } else if (c.provider === 'dmm') {
            if (parsed.api_id) dmmApiId = parsed.api_id
            if (parsed.affiliate_id) dmmAffId = parsed.affiliate_id
          }
        } catch { /* ignore */ }
      }
      dispatch({ type: 'load', aria2Url, aria2Token, dmmApiId, dmmAffId })
    })
  }, [open])

  const testAria2 = async () => {
    dispatch({ type: 'setAria2Test', value: 'testing' })
    try {
      await api.setProviderConfig('aria2', { rpc_url: state.aria2Url, token: state.aria2Token })
      await api.testProviderConfig('aria2')
      dispatch({ type: 'setAria2Test', value: 'ok' })
      onConfigChanged()
    } catch { dispatch({ type: 'setAria2Test', value: 'fail' }) }
  }

  const saveAria2 = async () => {
    await api.setProviderConfig('aria2', { rpc_url: state.aria2Url, token: state.aria2Token })
    onConfigChanged()
  }

  const testDmm = async () => {
    dispatch({ type: 'setDmmTest', value: 'testing' })
    try {
      await api.setProviderConfig('dmm', { api_id: state.dmmApiId, affiliate_id: state.dmmAffId })
      await api.testProviderConfig('dmm')
      dispatch({ type: 'setDmmTest', value: 'ok' })
      onConfigChanged()
    } catch { dispatch({ type: 'setDmmTest', value: 'fail' }) }
  }

  const saveDmm = async () => {
    await api.setProviderConfig('dmm', { api_id: state.dmmApiId, affiliate_id: state.dmmAffId })
    onConfigChanged()
  }

  return (
    <Modal open={open} onClose={onClose} title="Global Settings">
      <div className="space-y-6">
        {/* aria2 Settings */}
        <div>
          <h3 className="text-sm font-medium text-white flex items-center gap-1.5 mb-3">
            <Download size={13} className="text-indigo-400" />aria2 Settings
          </h3>
          <div className="space-y-3 pl-1">
            <div>
              <label htmlFor="gs-aria2-url" className="block text-xs text-gray-500 mb-1">RPC URL</label>
              <input id="gs-aria2-url" type="text" value={state.aria2Url} onChange={(e) => dispatch({ type: 'setAria2Url', value: e.target.value })}
                className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            </div>
            <div>
              <label htmlFor="gs-aria2-token" className="block text-xs text-gray-500 mb-1">Secret Token</label>
              <div className="relative">
                <input id="gs-aria2-token" type={state.showAria2Token ? 'text' : 'password'} value={state.aria2Token} onChange={(e) => dispatch({ type: 'setAria2Token', value: e.target.value })} placeholder="aria2 secret"
                  className="w-full px-3 py-2 pr-10 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
                <button type="button" onClick={() => dispatch({ type: 'toggleShowAria2Token' })} aria-label={state.showAria2Token ? 'Hide token' : 'Show token'}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white transition">
                  {state.showAria2Token ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button type="button" onClick={saveAria2} className="text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                Save
              </button>
              <button type="button" onClick={testAria2} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                {state.aria2Test === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
              </button>
              {state.aria2Test === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
              {state.aria2Test === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
            </div>
          </div>
        </div>

        <div className="border-t border-gray-800" />

        {/* DMM Settings */}
        <div>
          <h3 className="text-sm font-medium text-white flex items-center gap-1.5 mb-3">
            <Search size={13} className="text-indigo-400" />DMM API Settings
          </h3>
          <div className="space-y-3 pl-1">
            <div>
              <label htmlFor="gs-dmm-apiid" className="block text-xs text-gray-500 mb-1">API ID</label>
              <input id="gs-dmm-apiid" type="text" value={state.dmmApiId} onChange={(e) => dispatch({ type: 'setDmmApiId', value: e.target.value })} placeholder="Your DMM API ID"
                className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            </div>
            <div>
              <label htmlFor="gs-dmm-affid" className="block text-xs text-gray-500 mb-1">Affiliate ID</label>
              <div className="relative">
                <input id="gs-dmm-affid" type={state.showDmmAffId ? 'text' : 'password'} value={state.dmmAffId} onChange={(e) => dispatch({ type: 'setDmmAffId', value: e.target.value })} placeholder="Your Affiliate ID"
                  className="w-full px-3 py-2 pr-10 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
                <button type="button" onClick={() => dispatch({ type: 'toggleShowDmmAffId' })} aria-label={state.showDmmAffId ? 'Hide affiliate ID' : 'Show affiliate ID'}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white transition">
                  {state.showDmmAffId ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button type="button" onClick={saveDmm} className="text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                Save
              </button>
              <button type="button" onClick={testDmm} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                {state.dmmTest === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
              </button>
              {state.dmmTest === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
              {state.dmmTest === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
            </div>
          </div>
        </div>

        <div className="pt-3 border-t border-gray-800">
          <button type="button" onClick={onClose} className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">Done</button>
        </div>
      </div>
    </Modal>
  )
}
