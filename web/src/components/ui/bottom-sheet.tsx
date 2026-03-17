import { useEffect, useRef, type ReactNode } from 'react';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

interface BottomSheetProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  /** Sticky footer pinned at bottom — ideal for action buttons */
  stickyFooter?: ReactNode;
  /** Full-screen on mobile (for complex forms) */
  fullScreen?: boolean;
  /** Max width on desktop */
  maxWidth?: string;
}

export function BottomSheet({
  open,
  onClose,
  title,
  children,
  stickyFooter,
  fullScreen = false,
  maxWidth = 'max-w-lg',
}: BottomSheetProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handler);
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', handler);
      document.body.style.overflow = '';
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-end md:items-center justify-center"
      onClick={(e) => {
        if (e.target === overlayRef.current) onClose();
      }}
    >
      {/* Mobile: bottom sheet. Desktop: centered modal */}
      <div
        className={cn(
          'bg-card border-border flex flex-col overflow-hidden',
          'w-full border-t md:border md:rounded-xl md:shadow-2xl',
          // Mobile
          fullScreen
            ? 'h-full rounded-none'
            : 'max-h-[92vh] rounded-t-2xl',
          // Desktop
          fullScreen
            ? 'md:max-h-[90vh] md:max-w-6xl md:rounded-xl'
            : `md:max-h-[85vh] ${maxWidth} md:rounded-xl`,
        )}
      >
        {/* Header */}
        {title && (
          <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
            {/* Drag handle on mobile */}
            <div className="absolute top-2 left-1/2 -translate-x-1/2 w-10 h-1 rounded-full bg-border md:hidden" />
            <h2 className="font-semibold text-base">{title}</h2>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}

        {/* Scrollable content */}
        <div className="flex-1 overflow-y-auto overscroll-contain">
          {children}
        </div>

        {/* Sticky footer */}
        {stickyFooter && (
          <div className="shrink-0 border-t border-border bg-card px-5 py-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))]">
            {stickyFooter}
          </div>
        )}
      </div>
    </div>
  );
}
