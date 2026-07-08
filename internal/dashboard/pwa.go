package dashboard

// PWA assets served by `scanner serve` (public, no auth) so the dashboard is an
// installable, standalone web app.

// Manifest is the web app manifest (served at /manifest.webmanifest).
const Manifest = `{
  "name": "Cetus Scanner",
  "short_name": "Cetus",
  "description": "Market-data scanner: breadth, signal studies, and a live dashboard.",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "background_color": "#0b0f16",
  "theme_color": "#0b0f16",
  "icons": [
    { "src": "/icon.svg", "sizes": "any", "type": "image/svg+xml", "purpose": "any maskable" }
  ]
}`

// IconSVG is the app icon (served at /icon.svg) — a radar/scanner glyph.
const IconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
<rect width="64" height="64" rx="14" fill="#2563eb"/>
<g fill="none" stroke="#ffffff" stroke-width="3.5" stroke-linecap="round">
<circle cx="32" cy="32" r="26" opacity="0.28"/>
<circle cx="32" cy="32" r="17" opacity="0.5"/>
<circle cx="32" cy="32" r="8"/>
<path d="M32 32 L51 19"/>
</g></svg>`

// ServiceWorker is a minimal SW (served at /sw.js) — present so the app is
// installable; it does not cache (the scanner needs live data).
const ServiceWorker = `self.addEventListener('install', function(){ self.skipWaiting(); });
self.addEventListener('activate', function(e){ e.waitUntil(self.clients.claim()); });
self.addEventListener('fetch', function(){});`
