import { createContext, use } from 'react'

export interface LightboxItem {
  src: string
  width?: number
  height?: number
  msrc?: string
  type?: 'image' | 'video'
}

export type LightboxSource = string | LightboxItem

interface LightboxState {
  open: (images: LightboxSource[], index?: number, thumbEl?: HTMLElement) => void
}

export const LightboxContext = createContext<LightboxState>({ open: () => {} })

export const useLightbox = () => use(LightboxContext)
