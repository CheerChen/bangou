import { useEffect, useRef, useEffectEvent } from 'react'
import { X } from 'lucide-react'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  wide?: boolean
  children: React.ReactNode
}

export default function Modal({ open, onClose, title, wide, children }: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  // Effect Event: always sees the latest onClose but is not a dependency,
  // so the close listener does not re-subscribe on every parent render.
  const onCloseEvent = useEffectEvent(onClose)

  // Controlling a native <dialog> from an `open` prop requires an effect
  // because showModal()/close() are imperative APIs.
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (open && !dialog.open) {
      dialog.showModal()
    // react-doctor-disable-next-line react-doctor/no-event-handler -- imperative dialog API requires effect-driven open/close
    } else if (!open && dialog.open) {
      dialog.close()
    }
  }, [open])

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const handleClose = () => onCloseEvent()
    const handleCancel = (e: Event) => {
      e.preventDefault()
      onCloseEvent()
    }
    dialog.addEventListener('close', handleClose)
    dialog.addEventListener('cancel', handleCancel)
    return () => {
      dialog.removeEventListener('close', handleClose)
      dialog.removeEventListener('cancel', handleCancel)
    }
  }, [])

  return (
    // react-doctor-disable-next-line react-doctor/no-noninteractive-element-interactions -- native <dialog> is interactive; onClick handles backdrop click-to-close
    <dialog
      ref={dialogRef}
      aria-label={title}
      onClick={(e) => { if (e.target === dialogRef.current) onClose() }}
      onKeyDown={(e) => { if (e.key === 'Escape') onClose() }}
      className={`backdrop:bg-black/60 backdrop:backdrop-blur-sm bg-transparent p-0 border-0 max-h-none w-full m-auto ${wide ? 'sm:max-w-2xl' : 'sm:max-w-lg'}`}
    >
      <div className="bg-[#1a1a1a] border border-gray-800 rounded-2xl shadow-2xl max-h-[90vh] overflow-y-auto mx-4">
        {title && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
            <h2 className="text-lg font-medium text-white">{title}</h2>
            <button type="button" onClick={onClose} aria-label="Close"
              className="text-gray-500 hover:text-white transition">
              <X size={18} />
            </button>
          </div>
        )}
        <div className="px-6 py-5">
          {children}
        </div>
      </div>
    </dialog>
  )
}
