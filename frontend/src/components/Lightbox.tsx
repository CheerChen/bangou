import { createContext, useCallback, use, useMemo, useRef } from 'react'
import PhotoSwipeLightbox from 'photoswipe/lightbox'
import 'photoswipe/style.css'

export interface LightboxItem {
  src: string
  width?: number
  height?: number
  msrc?: string
  type?: 'image' | 'video'
}

type LightboxSource = string | LightboxItem

interface LightboxState {
  open: (images: LightboxSource[], index?: number, thumbEl?: HTMLElement) => void
}

const LightboxContext = createContext<LightboxState>({ open: () => {} })

export const useLightbox = () => use(LightboxContext)

interface ImageSize {
  width: number
  height: number
}

interface NormalizedLightboxItem {
  src: string
  type: 'image' | 'video'
  width: number
  height: number
  msrc?: string
}

function getThumbSize(thumbEl?: HTMLElement): ImageSize | null {
  if (!(thumbEl instanceof HTMLImageElement)) {
    return null
  }
  if (thumbEl.naturalWidth <= 1 || thumbEl.naturalHeight <= 1) {
    return null
  }
  return { width: thumbEl.naturalWidth, height: thumbEl.naturalHeight }
}

function normalizeItems(images: LightboxSource[], sizeCache: Map<string, ImageSize>): NormalizedLightboxItem[] {
  const out: NormalizedLightboxItem[] = []
  for (const image of images) {
    const item = typeof image === 'string' ? { src: image } : image
    if (!item.src) continue
    const isVideo = item.type === 'video'
    const cached = sizeCache.get(item.src)
    out.push({
      src: item.src,
      type: isVideo ? 'video' : 'image',
      width: item.width || cached?.width || (isVideo ? 720 : 1),
      height: item.height || cached?.height || (isVideo ? 480 : 1),
      msrc: item.msrc,
    })
  }
  return out
}

export function LightboxProvider({ children }: { children: React.ReactNode }) {
  const pswpRef = useRef<PhotoSwipeLightbox | null>(null)
  // Lazy-init refs so the Maps are not rebuilt on every render.
  const sizeCacheRef = useRef<Map<string, ImageSize> | null>(null)
  if (sizeCacheRef.current === null) sizeCacheRef.current = new Map()
  const probeRef = useRef<Map<string, Promise<ImageSize>> | null>(null)
  if (probeRef.current === null) probeRef.current = new Map()
  const sizeCache = sizeCacheRef.current
  const probeCache = probeRef.current

  const probeImageSize = useCallback((src: string) => {
    const cached = sizeCache.get(src)
    if (cached) {
      return Promise.resolve(cached)
    }

    const pending = probeCache.get(src)
    if (pending) {
      return pending
    }

    const probe = new Promise<ImageSize>((resolve, reject) => {
      const img = new Image()
      img.decoding = 'async'
      img.onload = () => {
        const size = { width: img.naturalWidth, height: img.naturalHeight }
        if (size.width > 1 && size.height > 1) {
          sizeCache.set(src, size)
          resolve(size)
        } else {
          reject(new Error(`Invalid image size for ${src}`))
        }
        probeCache.delete(src)
      }
      img.onerror = () => {
        probeCache.delete(src)
        reject(new Error(`Failed to load ${src}`))
      }
      img.src = src
    })

    probeCache.set(src, probe)
    return probe
  }, [sizeCache, probeCache])

  const open = useCallback((images: LightboxSource[], index = 0, thumbEl?: HTMLElement) => {
    if (pswpRef.current) {
      pswpRef.current.destroy()
      pswpRef.current = null
    }

    const dataSource = normalizeItems(images, sizeCache)
    if (dataSource.length === 0) {
      return
    }

    const thumbSize = getThumbSize(thumbEl)
    const currentItem = dataSource[index]
    if (thumbSize && currentItem && currentItem.type !== 'video' && (currentItem.width <= 1 || currentItem.height <= 1)) {
      currentItem.width = thumbSize.width
      currentItem.height = thumbSize.height
      sizeCache.set(currentItem.src, thumbSize)
    }
    if (thumbEl instanceof HTMLImageElement && currentItem && !currentItem.msrc) {
      currentItem.msrc = thumbEl.currentSrc || thumbEl.src
    }

    const lightbox = new PhotoSwipeLightbox({
      dataSource: dataSource as any,
      index,
      pswpModule: () => import('photoswipe'),
      bgOpacity: 0.9,
      showHideAnimationType: thumbEl ? 'zoom' : 'fade',
      closeOnVerticalDrag: true,
    })

    if (thumbEl) {
      lightbox.addFilter('thumbEl', () => thumbEl, 0)
    }

    // Video content type support
    lightbox.addFilter('isContentLoading', (isLoading, content) => {
      if ((content.data as any).type === 'video') {
        return false
      }
      return isLoading
    })

    lightbox.addFilter('useContentPlaceholder', (usePlaceholder, content) => {
      if ((content.data as any).type === 'video') {
        return false
      }
      return usePlaceholder
    })

    lightbox.on('contentLoad', (e: any) => {
      const { content } = e
      if (content.data.type !== 'video') return

      e.preventDefault()

      const container = document.createElement('div')
      container.style.cssText = 'display:flex;align-items:center;justify-content:center;width:100%;height:100%;'

      const iframe = document.createElement('iframe')
      iframe.src = content.data.src
      iframe.style.cssText = 'width:720px;height:480px;max-width:100%;max-height:100%;border:none;border-radius:8px;'
      iframe.setAttribute('allowfullscreen', '')
      iframe.setAttribute('allow', 'autoplay')

      container.appendChild(iframe)
      content.element = container
    })

    lightbox.on('contentActivate', (e: any) => {
      if (e.content.data.type === 'video') {
        const iframe = e.content.element?.querySelector('iframe')
        if (iframe) {
          // Reload to trigger autoplay
          iframe.src = iframe.src
        }
      }
    })

    lightbox.on('contentDeactivate', (e: any) => {
      if (e.content.data.type === 'video') {
        const iframe = e.content.element?.querySelector('iframe')
        if (iframe) {
          iframe.src = '' // Stop playback
        }
      }
    })

    const syncSlideSize = (targetIndex: number) => {
      const pswp = (lightbox as any).pswp
      const item = dataSource[targetIndex]
      const holders = pswp?.mainScroll?.itemHolders as Array<{ slide?: any }> | undefined
      if (!item?.width || !item?.height || item.width <= 1 || item.height <= 1 || !holders) {
        return
      }

      holders.forEach((holder) => {
        const slide = holder.slide
        if (!slide || slide.index !== targetIndex) {
          return
        }
        slide.width = item.width
        slide.height = item.height
        slide.content.width = item.width
        slide.content.height = item.height
        slide.updateContentSize(true)
      })

      pswp?.updateSize?.(true)
    }

    const ensureNearbySizes = (targetIndex: number) => {
      ;[targetIndex - 1, targetIndex, targetIndex + 1].forEach((candidateIndex) => {
        const item = dataSource[candidateIndex]
        if (!item || item.type === 'video' || (item.width > 1 && item.height > 1)) {
          return
        }
        void probeImageSize(item.src)
          .then((size) => {
            item.width = size.width
            item.height = size.height
            syncSlideSize(candidateIndex)
          })
          .catch(() => {})
      })
    }

    lightbox.on('loadComplete', (e: any) => {
      const { content, slide } = e
      const el = content.element
      if (el instanceof HTMLImageElement && el.naturalWidth > 1 && el.naturalHeight > 1) {
        const item = dataSource[slide.index]
        if (item) {
          item.width = el.naturalWidth
          item.height = el.naturalHeight
          sizeCache.set(item.src, { width: el.naturalWidth, height: el.naturalHeight })
        }
        slide.width = el.naturalWidth
        slide.height = el.naturalHeight
        content.width = el.naturalWidth
        content.height = el.naturalHeight
        slide.updateContentSize(true)
      }
    })

    lightbox.on('afterInit', () => {
      ensureNearbySizes(index)
    })

    lightbox.on('change', () => {
      const pswp = (lightbox as any).pswp
      if (!pswp) {
        return
      }
      ensureNearbySizes(pswp.currIndex)
    })

    lightbox.on('close', () => {
      setTimeout(() => lightbox.destroy(), 300)
    })

    lightbox.init()
    lightbox.loadAndOpen(index)
    pswpRef.current = lightbox
  }, [probeImageSize, sizeCache])

  const contextValue = useMemo(() => ({ open }), [open])

  return (
    <LightboxContext.Provider value={contextValue}>
      {children}
    </LightboxContext.Provider>
  )
}
