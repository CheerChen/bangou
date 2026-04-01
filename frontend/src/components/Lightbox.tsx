import { createContext, useContext, useCallback, useRef } from 'react'
import PhotoSwipeLightbox from 'photoswipe/lightbox'
import 'photoswipe/style.css'

interface LightboxState {
  open: (images: string[], index?: number, thumbEl?: HTMLElement) => void
}

const LightboxContext = createContext<LightboxState>({ open: () => {} })

export const useLightbox = () => useContext(LightboxContext)

export function LightboxProvider({ children }: { children: React.ReactNode }) {
  const pswpRef = useRef<PhotoSwipeLightbox | null>(null)

  const open = useCallback((images: string[], index = 0, thumbEl?: HTMLElement) => {
    if (pswpRef.current) {
      pswpRef.current.destroy()
    }

    const dataSource = images.map((src) => ({
      src,
      width: 1920,
      height: 1080,
    }))

    const lightbox = new PhotoSwipeLightbox({
      dataSource,
      index,
      pswpModule: () => import('photoswipe'),
      bgOpacity: 0.9,
      showHideAnimationType: thumbEl ? 'zoom' : 'fade',
      closeOnVerticalDrag: true,
    })

    // Set the thumbnail element for zoom animation
    if (thumbEl) {
      lightbox.addFilter('thumbEl', () => thumbEl, 0)
    }

    lightbox.on('close', () => {
      setTimeout(() => lightbox.destroy(), 300)
    })

    lightbox.init()
    lightbox.loadAndOpen(index)
    pswpRef.current = lightbox
  }, [])

  return (
    <LightboxContext.Provider value={{ open }}>
      {children}
    </LightboxContext.Provider>
  )
}
