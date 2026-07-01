interface Props {
  genres?: string[]
  maker?: string
  label?: string
  series?: string
  director?: string
}

export default function TagList({ genres, maker, label, series, director }: Props) {
  const tags: { text: string; color: string; truncate?: boolean }[] = []
  if (maker) tags.push({ text: maker, color: 'bg-indigo-500/10 text-indigo-300' })
  if (label) tags.push({ text: label, color: 'bg-violet-500/10 text-violet-400' })
  if (series) tags.push({ text: series, color: 'bg-amber-500/10 text-amber-400', truncate: true })
  if (director) tags.push({ text: director, color: 'bg-cyan-500/10 text-cyan-400' })
  if (genres) {
    for (const g of genres) {
      tags.push({ text: g, color: 'bg-gray-800 text-gray-300' })
    }
  }
  if (tags.length === 0) return null
  return (
    <div className="flex flex-wrap gap-1">
      {tags.map((t) => (
        <span key={t.text} title={t.truncate ? t.text : undefined}
          className={`text-[11px] px-1.5 py-0.5 rounded ${t.color} ${t.truncate ? 'max-w-[10rem] truncate' : ''}`}>
          {t.text}
        </span>
      ))}
    </div>
  )
}
