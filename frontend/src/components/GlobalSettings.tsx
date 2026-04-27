import { useState, useEffect } from 'react'
import { CheckCircle, AlertCircle, Loader2, Download, Search, Eye, EyeOff } from 'lucide-react'
import Modal from './Modal'
import * as api from '../api/client'

interface GlobalSettingsProps {
  open: boolean
  onClose: () => void
  onConfigChanged: () => void
}

export default function GlobalSettings({ open, onClose, onConfigChanged }: GlobalSettingsProps) {
  // aria2
  const [aria2Url, setAria2Url] = useState('http://localhost:6800/jsonrpc')
  const [aria2Token, setAria2Token] = useState('')
  const [aria2Test, setAria2Test] = useState<'idle' | 'testing' | 'ok' | 'fail'>('idle')

  // DMM
  const [dmmApiId, setDmmApiId] = useState('')
  const [dmmAffId, setDmmAffId] = useState('')
  const [dmmTest, setDmmTest] = useState<'idle' | 'testing' | 'ok' | 'fail'>('idle')

  // visibility toggles
  const [showAria2Token, setShowAria2Token] = useState(false)
  const [showDmmAffId, setShowDmmAffId] = useState(false)

  useEffect(() => {
    if (!open) return
    api.listProviderConfigs().then((configs) => {
      for (const c of configs) {
        try {
          const parsed = JSON.parse(c.config)
          if (c.provider === 'aria2') {
            if (parsed.rpc_url) setAria2Url(parsed.rpc_url)
            if (parsed.token) setAria2Token(parsed.token)
          } else if (c.provider === 'dmm') {
            if (parsed.api_id) setDmmApiId(parsed.api_id)
            if (parsed.affiliate_id) setDmmAffId(parsed.affiliate_id)
          }
        } catch { /* ignore */ }
      }
    })
  }, [open])

  const testAria2 = async () => {
    setAria2Test('testing')
    try {
      await api.setProviderConfig('aria2', { rpc_url: aria2Url, token: aria2Token })
      await api.testProviderConfig('aria2')
      setAria2Test('ok')
      onConfigChanged()
    } catch { setAria2Test('fail') }
  }

  const saveAria2 = async () => {
    await api.setProviderConfig('aria2', { rpc_url: aria2Url, token: aria2Token })
    onConfigChanged()
  }

  const testDmm = async () => {
    setDmmTest('testing')
    try {
      await api.setProviderConfig('dmm', { api_id: dmmApiId, affiliate_id: dmmAffId })
      await api.testProviderConfig('dmm')
      setDmmTest('ok')
      onConfigChanged()
    } catch { setDmmTest('fail') }
  }

  const saveDmm = async () => {
    await api.setProviderConfig('dmm', { api_id: dmmApiId, affiliate_id: dmmAffId })
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
              <label className="block text-xs text-gray-500 mb-1">RPC URL</label>
              <input type="text" value={aria2Url} onChange={(e) => { setAria2Url(e.target.value); setAria2Test('idle') }}
                className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Secret Token</label>
              <div className="relative">
                <input type={showAria2Token ? 'text' : 'password'} value={aria2Token} onChange={(e) => { setAria2Token(e.target.value); setAria2Test('idle') }} placeholder="aria2 secret"
                  className="w-full px-3 py-2 pr-10 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
                <button type="button" onClick={() => setShowAria2Token(!showAria2Token)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white transition">
                  {showAria2Token ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button onClick={saveAria2} className="text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                Save
              </button>
              <button onClick={testAria2} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                {aria2Test === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
              </button>
              {aria2Test === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
              {aria2Test === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
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
              <label className="block text-xs text-gray-500 mb-1">API ID</label>
              <input type="text" value={dmmApiId} onChange={(e) => { setDmmApiId(e.target.value); setDmmTest('idle') }} placeholder="Your DMM API ID"
                className="w-full px-3 py-2 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Affiliate ID</label>
              <div className="relative">
                <input type={showDmmAffId ? 'text' : 'password'} value={dmmAffId} onChange={(e) => { setDmmAffId(e.target.value); setDmmTest('idle') }} placeholder="Your Affiliate ID"
                  className="w-full px-3 py-2 pr-10 bg-[#111] border border-gray-700 rounded-lg text-white text-sm focus:border-indigo-500 focus:outline-none" />
                <button type="button" onClick={() => setShowDmmAffId(!showDmmAffId)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white transition">
                  {showDmmAffId ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button onClick={saveDmm} className="text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                Save
              </button>
              <button onClick={testDmm} className="flex items-center gap-1.5 text-xs px-3 py-1.5 border border-gray-700 text-gray-400 hover:text-white rounded-lg transition">
                {dmmTest === 'testing' && <Loader2 size={12} className="animate-spin" />}Test
              </button>
              {dmmTest === 'ok' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CheckCircle size={12} />OK</span>}
              {dmmTest === 'fail' && <span className="flex items-center gap-1 text-xs text-red-400"><AlertCircle size={12} />Failed</span>}
            </div>
          </div>
        </div>

        <div className="pt-3 border-t border-gray-800">
          <button onClick={onClose} className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm rounded-lg transition">Done</button>
        </div>
      </div>
    </Modal>
  )
}
