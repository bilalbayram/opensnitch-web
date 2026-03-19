import { useEffect, useRef } from 'react';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import { cn } from '@/lib/utils';

export type GeoPoint = {
  ip: string;
  country: string;
  country_code: string;
  city: string;
  lat: number;
  lon: number;
  hits: number;
};

interface GeoMapProps {
  geoData: GeoPoint[];
  className?: string;
}

const DARK_STYLE = 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json';
const SOURCE_ID = 'geo-points';
const DOT_LAYER = 'geo-dots';
const GLOW_LAYER = 'geo-glow';

function toFeatureCollection(points: GeoPoint[]): GeoJSON.FeatureCollection {
  return {
    type: 'FeatureCollection',
    features: points
      .filter((p) => p.lat !== 0 || p.lon !== 0)
      .map((p) => ({
        type: 'Feature' as const,
        geometry: { type: 'Point' as const, coordinates: [p.lon, p.lat] },
        properties: {
          ip: p.ip,
          city: p.city,
          country: p.country,
          country_code: p.country_code,
          hits: p.hits,
        },
      })),
  };
}

export function GeoMap({ geoData, className }: GeoMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const popupRef = useRef<maplibregl.Popup | null>(null);

  // Initialize map
  useEffect(() => {
    if (!containerRef.current) return;

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: DARK_STYLE,
      center: [20, 20],
      zoom: 1.3,
      minZoom: 0.8,
      maxZoom: 6,
      attributionControl: false,
      scrollZoom: false,
    });

    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'bottom-right');

    map.on('load', () => {
      map.addSource(SOURCE_ID, {
        type: 'geojson',
        data: toFeatureCollection(geoData),
      });

      // Glow layer (large, semi-transparent circles)
      map.addLayer({
        id: GLOW_LAYER,
        type: 'circle',
        source: SOURCE_ID,
        paint: {
          'circle-radius': [
            'interpolate', ['linear'], ['get', 'hits'],
            1, 12,
            50, 24,
            500, 36,
          ],
          'circle-color': '#6366f1',
          'circle-opacity': 0.12,
          'circle-blur': 1,
        },
      });

      // Dot layer (solid, sized by hits)
      map.addLayer({
        id: DOT_LAYER,
        type: 'circle',
        source: SOURCE_ID,
        paint: {
          'circle-radius': [
            'interpolate', ['linear'], ['get', 'hits'],
            1, 3,
            50, 7,
            500, 11,
          ],
          'circle-color': '#6366f1',
          'circle-opacity': 0.85,
          'circle-stroke-color': '#ffffff',
          'circle-stroke-width': 0.5,
          'circle-stroke-opacity': 0.3,
        },
      });

      // Hover popup
      map.on('mouseenter', DOT_LAYER, (e) => {
        map.getCanvas().style.cursor = 'pointer';
        const f = e.features?.[0];
        if (!f || f.geometry.type !== 'Point') return;

        const coords = f.geometry.coordinates.slice() as [number, number];
        const { city, country, ip, hits } = f.properties as Record<string, string>;

        popupRef.current?.remove();
        popupRef.current = new maplibregl.Popup({ closeButton: false, offset: 10 })
          .setLngLat(coords)
          .setHTML(
            `<div class="geo-popup">` +
            `<strong>${city ? city + ', ' : ''}${country}</strong>` +
            `<div class="geo-popup-ip">${ip}</div>` +
            `<div class="geo-popup-hits">${Number(hits).toLocaleString()} connections</div>` +
            `</div>`,
          )
          .addTo(map);
      });

      map.on('mouseleave', DOT_LAYER, () => {
        map.getCanvas().style.cursor = '';
        popupRef.current?.remove();
        popupRef.current = null;
      });
    });

    mapRef.current = map;

    // Resize observer
    const observer = new ResizeObserver(() => map.resize());
    observer.observe(containerRef.current);

    return () => {
      observer.disconnect();
      map.remove();
      mapRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Update data when geoData changes
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const update = () => {
      const source = map.getSource(SOURCE_ID) as maplibregl.GeoJSONSource | undefined;
      if (source) {
        source.setData(toFeatureCollection(geoData));
      }
    };

    if (map.isStyleLoaded()) {
      update();
    } else {
      map.once('load', update);
    }
  }, [geoData]);

  const uniqueCountries = new Set(geoData.map((g) => g.country_code)).size;

  return (
    <div className={cn('relative rounded-xl border border-border overflow-hidden', className)} style={{ height: '40vh', minHeight: 200 }}>
      <div ref={containerRef} style={{ width: '100%', height: '100%' }} />

      {/* Overlay labels */}
      <div className="absolute top-2 left-3 flex items-center gap-1.5 text-[10px] text-white/40 pointer-events-none z-10">
        <svg className="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="10" />
          <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
        </svg>
        <span>Destinations (24h)</span>
      </div>
      {geoData.length > 0 && (
        <div className="absolute bottom-2 left-3 text-[10px] text-white/40 pointer-events-none z-10">
          {geoData.length} IPs &middot; {uniqueCountries} countries
        </div>
      )}
    </div>
  );
}
