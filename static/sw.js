// InvoBill Service Worker — offline shell + asset caching
const CACHE = 'invobill-v1';
const OFFLINE_URL = '/offline';

// Assets to pre-cache on install.
const PRECACHE = [
  '/static/css/app.css',
  '/static/manifest.json',
];

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE).then(cache => cache.addAll(PRECACHE))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    ).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', event => {
  const { request } = event;
  const url = new URL(request.url);

  // Only handle same-origin GET requests.
  if (request.method !== 'GET' || url.origin !== self.location.origin) return;

  // Static assets: cache-first.
  if (url.pathname.startsWith('/static/')) {
    event.respondWith(
      caches.match(request).then(cached => cached || fetch(request).then(res => {
        const clone = res.clone();
        caches.open(CACHE).then(c => c.put(request, clone));
        return res;
      }))
    );
    return;
  }

  // HTML navigation: network-first, fall back to a minimal offline page.
  if (request.headers.get('Accept')?.includes('text/html')) {
    event.respondWith(
      fetch(request).catch(() =>
        new Response(
          `<!doctype html><html lang="en"><head><meta charset="utf-8">
          <meta name="viewport" content="width=device-width,initial-scale=1">
          <title>Offline — InvoBill</title>
          <style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;
          justify-content:center;height:100vh;margin:0;background:#080b14;color:#e2e8f0}
          .box{text-align:center;padding:2rem}.icon{font-size:3rem}.h{font-size:1.5rem;
          font-weight:700;margin:.5rem 0}.p{color:#94a3b8;margin-bottom:1.5rem}
          a{color:#4f46e5;text-decoration:none;border:1px solid #4f46e5;padding:.5rem 1.2rem;
          border-radius:8px}</style></head>
          <body><div class="box"><div class="icon">📡</div>
          <div class="h">You're offline</div>
          <p class="p">InvoBill needs a connection to load this page.</p>
          <a href="/dashboard">Retry</a></div></body></html>`,
          { headers: { 'Content-Type': 'text/html' } }
        )
      )
    );
    return;
  }
});
