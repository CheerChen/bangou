import { Routes, Route, Link } from 'react-router-dom'
import Home from './pages/Home'
import PipelineDetail from './pages/PipelineDetail'

export default function App() {
  return (
    <div className="min-h-screen bg-[#0f0f0f] text-gray-200">
      <nav className="border-b border-gray-800 px-6 py-3">
        <Link to="/" className="text-lg font-semibold text-white tracking-tight hover:text-indigo-400 transition">bangou</Link>
      </nav>
      <main className="max-w-6xl mx-auto px-4 py-6">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/pipelines/:id" element={<PipelineDetail />} />
        </Routes>
      </main>
    </div>
  )
}
