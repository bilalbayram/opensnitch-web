import { useState, useSyncExternalStore } from 'react';

function subscribe(query: string) {
  return (callback: () => void) => {
    const mql = window.matchMedia(query);
    mql.addEventListener('change', callback);
    return () => mql.removeEventListener('change', callback);
  };
}

function getSnapshot(query: string) {
  return () => window.matchMedia(query).matches;
}

function getServerSnapshot() {
  return false;
}

export function useMediaQuery(query: string): boolean {
  const sub = useState(() => subscribe(query))[0];
  const snap = useState(() => getSnapshot(query))[0];
  return useSyncExternalStore(sub, snap, getServerSnapshot);
}

/** Below 768px */
export function useIsMobile() {
  return useMediaQuery('(max-width: 767px)');
}

/** 768px – 1023px */
export function useIsTablet() {
  return useMediaQuery('(min-width: 768px) and (max-width: 1023px)');
}

/** 1024px+ */
export function useIsDesktop() {
  return useMediaQuery('(min-width: 1024px)');
}
