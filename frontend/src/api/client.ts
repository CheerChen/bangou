const BASE = '/api'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) {
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(`${BASE}${path}`, opts)
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data as T
}

// ── Pipelines ──

export interface PipelineResponse {
  id: number
  name: string
  inputDir: string
  outputDir: string
  pathPattern: string
  archiveDir: string
  enableMerge: boolean
  downloadProvider: string
  scrapeProviders: string[]
  pendingCount: number
  libraryCount: number
  status: string
}

export interface CreatePipelineReq {
  name: string
  inputDir: string
  outputDir: string
  pathPattern: string
  archiveDir: string
  enableMerge: boolean
  downloadProvider: string
  scrapeProviders: string[]
}

export const listPipelines = () => request<PipelineResponse[]>('GET', '/pipelines')
export const createPipeline = (data: CreatePipelineReq) => request<{ id: number }>('POST', '/pipelines', data)
export const deletePipeline = (id: number) => request<unknown>('DELETE', `/pipelines/${id}`)

// ── Groups ──

export interface GroupsPage {
  groups: GroupResponse[]
  unknowns: UnknownResponse[]
  linkable: number
}

export interface GroupResponse {
  number: string
  items: ItemResponse[]
  totalSizeGB: number
  scrape: ScrapeResponse
  task: string
  taskErr?: string
  taskProgress: number
  allReady: boolean
}

export interface ItemResponse {
  path: string
  filename: string
  part: number
  sizeGB: number
  ready: boolean
  resolution?: string
  videoCodec?: string
  audioCodec?: string
  bitrate?: string
  duration?: string
  downloadPct: number
  downloadStatus?: string
}

export interface ScrapeResponse {
  meta: MetaResponse | null
  errors?: Record<string, string>
  status: string
}

export interface MetaResponse {
  number: string
  title: string
  maker?: string
  label?: string
  series?: string
  actors?: string[]
  genres?: string[]
  coverURL?: string
  sampleImages?: string[]
  premiered?: string
  year?: string
  runtime?: string
  rating?: string
  reviewCount: number
  pageURL?: string
  provider?: string
}

export interface UnknownResponse {
  path: string
  filename: string
  sizeGB: number
}

export const listGroups = (pipelineId: number) =>
  request<GroupsPage>('GET', `/pipelines/${pipelineId}/groups`)

// ── Group Actions ──

export const groupLink = (pipelineId: number, number: string, paths: string[]) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/groups/${number}/link`, { paths })

export const groupMerge = (pipelineId: number, number: string, paths: string[]) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/groups/${number}/merge`, { paths })

export const groupIgnore = (pipelineId: number, number: string) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/groups/${number}/ignore`, {})

export const groupRescrape = (pipelineId: number, number: string) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/groups/${number}/rescrape`, {})

export const groupTag = (pipelineId: number, number: string, path: string) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/groups/${number}/tag`, { path })

// ── Unknown Actions ──

export const unknownTag = (pipelineId: number, path: string, number: string) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/unknowns/tag`, { path, number })

export const unknownIgnore = (pipelineId: number, path: string) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/unknowns/ignore`, { path })

// ── Scan / Link All ──

export const triggerScan = (pipelineId: number) =>
  request<unknown>('POST', `/pipelines/${pipelineId}/scan`, {})

export interface LinkAllStatus {
  total: number
  done: number
  current: string
  errors?: string[]
  running: boolean
}

export const linkAll = (pipelineId: number) =>
  request<LinkAllStatus>('POST', `/pipelines/${pipelineId}/link-all`, {})

export const linkAllProgress = (pipelineId: number) =>
  request<LinkAllStatus>('GET', `/pipelines/${pipelineId}/link-all/progress`)

// ── Library ──

export interface LibraryPage {
  items: LibraryItemResponse[]
  total: number
  page: number
  size: number
}

export interface LibraryItemResponse {
  id: number
  number: string
  srcPath: string
  linkPath: string
  linkType: string
  fileSize: number
  resolution?: string
  videoCodec?: string
  audioCodec?: string
  duration?: string
  bitrate?: string
  alive: boolean
  title?: string
  actors?: string
  genres?: string[]
  coverURL?: string
  sampleImages?: string[]
  rating?: string
  reviewCount: number
  pageURL?: string
  maker?: string
  premiered?: string
  year?: string
  runtime?: string
  provider?: string
}

export const listLibrary = (pipelineId: number, page = 0, size = 12, sort = 'added', order = 'desc') =>
  request<LibraryPage>('GET', `/pipelines/${pipelineId}/library?page=${page}&size=${size}&sort=${sort}&order=${order}`)

// ── Library Actions ──

export const libraryRescrape = (number: string) =>
  request<unknown>('POST', `/library/${number}/rescrape`, {})

export const libraryApply = (number: string) =>
  request<unknown>('POST', `/library/${number}/apply`, {})

export const libraryDismiss = (number: string) =>
  request<unknown>('POST', `/library/${number}/dismiss`, {})

export const unlinkOutput = (id: number, number: string) =>
  request<unknown>('POST', `/outputs/${id}/unlink`, { number })

export const deleteOutput = (id: number) =>
  request<unknown>('DELETE', `/outputs/${id}`)

// ── Provider Configs ──

export interface ProviderConfig {
  provider: string
  config: string
}

export const listProviderConfigs = () => request<ProviderConfig[]>('GET', '/provider-configs')

export const setProviderConfig = (provider: string, config: Record<string, string>) =>
  request<unknown>('PUT', `/provider-configs/${provider}`, config)

export const testProviderConfig = (provider: string) =>
  request<{ status: string }>('POST', `/provider-configs/${provider}/test`, {})
