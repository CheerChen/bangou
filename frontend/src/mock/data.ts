import type { Pipeline, PipelineStats, Group, LibraryItem, UnknownFile } from '../types'

export const pipelines: Pipeline[] = [
  {
    id: 'vr',
    name: 'VR',
    inputDir: '/download/VR',
    archiveDir: '/archive/VR',
    outputDir: '/media/VR',
    pathPattern: '{Year}/{Number}',
    providers: ['dmm', 'avwiki'],
    autoMerge: true,
  },
  {
    id: 'jav',
    name: 'JAV',
    inputDir: '/download/JAV',
    archiveDir: '',
    outputDir: '/media/JAV',
    pathPattern: '{Year}/{Actor}/{Number}',
    providers: ['avwiki', 'dmm'],
    autoMerge: false,
  },
  {
    id: 'anime',
    name: 'Anime',
    inputDir: '/download/Anime',
    archiveDir: '',
    outputDir: '/media/Anime',
    pathPattern: '{Series}/{Number}',
    providers: ['avwiki'],
    autoMerge: false,
  },
]

export const pipelineStats: PipelineStats[] = [
  { pipeline: pipelines[0], pendingCount: 6, libraryCount: 60, status: 'scanning' },
  { pipeline: pipelines[1], pendingCount: 3, libraryCount: 89, status: 'idle' },
  { pipeline: pipelines[2], pendingCount: 0, libraryCount: 34, status: 'idle' },
]

const cover = (id: string) => `https://pics.dmm.co.jp/digital/video/${id}/${id}pl.jpg`

export const groups: Record<string, Group[]> = {
  vr: [
    {
      number: 'DSVR-1737',
      items: [
        { path: '/download/VR/13dsvr01737.part1.mp4', filename: '13dsvr01737.part1.mp4', part: 1, sizeGB: 14.2, ready: true, downloadPct: 0, downloadStatus: '', media: { resolution: '4320x2160', videoCodec: 'HEVC', audioCodec: 'AAC', bitrate: '52.3 Mbps', duration: '1:42:15' } },
        { path: '/download/VR/13dsvr01737.part2.mp4', filename: '13dsvr01737.part2.mp4', part: 2, sizeGB: 14.3, ready: true, downloadPct: 0, downloadStatus: '', media: { resolution: '4320x2160', videoCodec: 'HEVC', audioCodec: 'AAC', bitrate: '52.1 Mbps', duration: '1:41:50' } },
      ],
      scrape: {
        meta: { number: 'DSVR-1737', title: '【VR】フェス帰り×夜行バス ライブで意気投合した小悪魔美少女にゼロ距離密着', maker: 'SOD Create', label: '3DSVR', series: '', actors: ['MINAMO'], genres: ['VR専用', 'ハイクオリティVR', '痴女'], coverURL: cover('13dsvr01737'), sampleImages: [], premiered: '2025-06-15', year: '2025', runtime: '103', rating: '4.25', reviewCount: 8, pageURL: 'https://www.dmm.co.jp/digital/videoa/-/detail/=/cid=13dsvr01737/', provider: 'dmm' },
        errors: {},
        status: 'success',
      },
      task: '',
      taskErr: '',
      taskProgress: 0,
    },
    {
      number: 'SIVR-414',
      items: [
        { path: '/download/VR/sivr00414_1_8k.mp4', filename: '4k2.com@sivr00414_1_8k.mp4', part: 1, sizeGB: 22.1, ready: true, downloadPct: 0, downloadStatus: '', media: { resolution: '7680x3840', videoCodec: 'HEVC', audioCodec: 'AAC', bitrate: '78.2 Mbps', duration: '2:01:30' } },
      ],
      scrape: {
        meta: { number: 'SIVR-414', title: '【VR】完全主観で楽しめるノーモザイク 超高画質8K', maker: 'S1 NO.1 STYLE', label: 'SIVR', series: '', actors: ['七ツ森りり'], genres: ['VR専用', '8K', '単体作品'], coverURL: cover('sivr00414'), sampleImages: [], premiered: '2025-03-20', year: '2025', runtime: '121', rating: '4.50', reviewCount: 12, pageURL: '', provider: 'dmm' },
        errors: {},
        status: 'success',
      },
      task: '',
      taskErr: '',
      taskProgress: 0,
    },
    {
      number: 'SAVR-676',
      items: [
        { path: '/download/VR/savr00676.part1.mp4', filename: 'savr00676.part1.mp4', part: 1, sizeGB: 14.0, ready: true, downloadPct: 0, downloadStatus: '' },
        { path: '/download/VR/savr00676.part2.mp4', filename: 'savr00676.part2.mp4', part: 2, sizeGB: 14.1, ready: true, downloadPct: 0, downloadStatus: '' },
        { path: '/download/VR/savr00676.part3.mp4', filename: 'savr00676.part3.mp4', part: 3, sizeGB: 14.0, ready: false, downloadPct: 67, downloadStatus: 'active' },
      ],
      scrape: {
        meta: { number: 'SAVR-676', title: '【VR】全身舐め 濃厚なキス いいなり なんでもOK 全裸変態メイド', maker: 'KMP', label: 'SAVR', series: '', actors: ['藤田こずえ'], genres: ['VR専用', 'メイド'], coverURL: cover('savr00676'), sampleImages: [], premiered: '2025-05-01', year: '2025', runtime: '96', rating: '', reviewCount: 0, pageURL: '', provider: 'dmm' },
        errors: {},
        status: 'success',
      },
      task: '',
      taskErr: '',
      taskProgress: 0,
    },
    {
      number: 'MDVR-336',
      items: [
        { path: '/download/VR/mdvr00336_1.mp4', filename: 'mdvr00336_1.mp4', part: 1, sizeGB: 12.0, ready: true, downloadPct: 0, downloadStatus: '' },
      ],
      scrape: {
        meta: null,
        errors: { avwiki: 'avwiki no result for MDVR-336', dmm: 'dmm no results for MDVR-336' },
        status: 'failed',
      },
      task: '',
      taskErr: '',
      taskProgress: 0,
    },
    {
      number: 'HNVR-141',
      items: [
        { path: '/download/VR/hnvr00141.part1.mp4', filename: 'hnvr00141.part1.mp4', part: 1, sizeGB: 8.5, ready: true, downloadPct: 0, downloadStatus: '' },
        { path: '/download/VR/hnvr00141.part2.mp4', filename: 'hnvr00141.part2.mp4', part: 2, sizeGB: 8.4, ready: true, downloadPct: 0, downloadStatus: '' },
      ],
      scrape: {
        meta: { number: 'HNVR-141', title: '【VR】退院までの5日間いたずら痴女ナースの凄テク', maker: 'SOD', label: 'HNVR', series: '', actors: ['松本いちか'], genres: ['VR専用', 'ナース'], coverURL: cover('hnvr00141'), sampleImages: [], premiered: '2025-04-10', year: '2025', runtime: '88', rating: '3.75', reviewCount: 4, pageURL: '', provider: 'dmm' },
        errors: {},
        status: 'success',
      },
      task: 'merging',
      taskErr: '',
      taskProgress: 45,
    },
  ],
  jav: [
    {
      number: 'ACHJ-057',
      items: [
        { path: '/download/JAV/ACHJ-057.mp4', filename: 'ACHJ-057.mp4', part: 0, sizeGB: 4.8, ready: true, downloadPct: 0, downloadStatus: '' },
      ],
      scrape: {
        meta: { number: 'ACHJ-057', title: '都合のイイ カラダ 社長秘書の裏の顔', maker: 'Attackers', label: 'ACHJ', series: '', actors: ['石川澪'], genres: ['単体作品', 'OL'], coverURL: cover('achj00057'), sampleImages: [], premiered: '2025-01-15', year: '2025', runtime: '120', rating: '4.00', reviewCount: 6, pageURL: '', provider: 'dmm' },
        errors: {},
        status: 'success',
      },
      task: '',
      taskErr: '',
      taskProgress: 0,
    },
  ],
  anime: [],
}

const vrLabels = ['SIVR', 'DSVR', 'KAVR', 'SAVR', 'MDVR', 'HNVR', 'FCVR', 'JUVR', 'IPVR', 'WAVR']
const makers = ['S1 NO.1 STYLE', 'SOD Create', 'KMP', 'Fitch', 'MOODYZ', 'Prestige', 'Attackers', 'Madonna', 'IDEA POCKET', 'kawaii*']
const actresses = ['MINAMO', '七ツ森りり', '桃園怜奈', '美乃すずめ', '松本いちか', '石川澪', '椎名もも', '藤田こずえ', '天使もえ', '明日花キララ', '三上悠亜', '橋本ありな']
const vrGenres = ['VR専用', 'ハイクオリティVR', '8K', '単体作品', '巨乳', '痴女', 'NTR', 'メイド', 'ナース', 'OL']

function generateVRLibrary(count: number): LibraryItem[] {
  const items: LibraryItem[] = []
  for (let i = 0; i < count; i++) {
    const label = vrLabels[i % vrLabels.length]
    const num = 100 + i * 7
    const number = `${label}-${num}`
    const cid = `${label.toLowerCase()}00${num}`
    const maker = makers[i % makers.length]
    const actress = actresses[i % actresses.length]
    const year = i < 40 ? '2025' : '2024'
    const alive = i !== 5 && i !== 22 && i !== 41
    items.push({
      id: i + 1,
      number,
      srcPath: alive ? `/download/VR/${cid}.mp4` : '',
      linkPath: `/media/VR/${year}/${number}/${number}.mp4`,
      linkType: 'symlink',
      alive,
      title: `【VR】Mock title for ${number} featuring ${actress}`,
      actors: actress,
      genres: [vrGenres[i % vrGenres.length], vrGenres[(i + 3) % vrGenres.length]],
      coverURL: cover(cid),
      sampleImages: [],
      rating: alive ? ((3 + (i % 8) * 0.25).toFixed(2)) : '',
      reviewCount: alive ? (i % 20) : 0,
      pageURL: '',
      maker,
      year,
      runtime: String(80 + (i % 60)),
      provider: i % 3 === 0 ? 'avwiki' : 'dmm',
    })
  }
  return items
}

export const library: Record<string, LibraryItem[]> = {
  vr: generateVRLibrary(60),
  jav: [
    { id: 100, number: 'HMN-690', srcPath: '/download/JAV/hmn690.mp4', linkPath: '/media/JAV/2025/HMN-690/HMN-690.mp4', linkType: 'symlink', alive: true, title: '新人 某有名お嬢様大学に通う', actors: '椎名もも', genres: ['単体作品', '美少女'], coverURL: cover('hmn00690'), sampleImages: [], rating: '4.25', reviewCount: 10, pageURL: '', maker: '本中', year: '2025', runtime: '150', provider: 'dmm' },
  ],
  anime: [],
}

export const unknowns: Record<string, UnknownFile[]> = {
  vr: [
    { path: '/download/VR/random_clip_2025.mp4', filename: 'random_clip_2025.mp4', sizeGB: 1.2 },
  ],
  jav: [],
  anime: [],
}
