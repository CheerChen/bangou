export interface Pipeline {
  id: string
  name: string
  inputDir: string
  archiveDir: string // empty = disabled
  outputDir: string
  pathPattern: string
  providers: string[]
  autoMerge: boolean
}

export interface PipelineStats {
  pipeline: Pipeline
  pendingCount: number
  libraryCount: number
  status: 'idle' | 'scanning' | 'linking'
}

export interface MediaInfo {
  resolution: string
  videoCodec: string
  audioCodec: string
  bitrate: string
  duration: string
}

export interface StagedItem {
  path: string
  filename: string
  part: number
  sizeGB: number
  ready: boolean
  media?: MediaInfo
  downloadPct: number
  downloadStatus: string
}

export interface MovieMetadata {
  number: string
  title: string
  maker: string
  label: string
  series: string
  actors: string[]
  genres: string[]
  coverURL: string
  sampleImages: string[]
  premiered: string
  year: string
  runtime: string
  rating: string
  reviewCount: number
  pageURL: string
  provider: string
}

export interface ScrapeResult {
  meta: MovieMetadata | null
  errors: Record<string, string>
  status: '' | 'scraping' | 'success' | 'failed'
}

export interface Group {
  number: string
  items: StagedItem[]
  scrape: ScrapeResult
  task: '' | 'linking' | 'merging' | 'error'
  taskErr: string
  taskProgress: number
}

export interface LibraryItem {
  id: number
  number: string
  srcPath: string
  linkPath: string
  linkType: 'hardlink' | 'symlink'
  alive: boolean
  title: string
  actors: string
  genres: string[]
  coverURL: string
  sampleImages: string[]
  rating: string
  reviewCount: number
  pageURL: string
  maker: string
  year: string
  runtime: string
  provider: string
}

export interface UnknownFile {
  path: string
  filename: string
  sizeGB: number
}
