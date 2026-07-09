const STATIC_CACHE = "bws-static-v1";
const RUNTIME_CACHE = "bws-runtime-v1";
const QR_CACHE = "bws-qr-v1";
const APP_SHELL = ["/", "/favicon.svg", "/manifest.webmanifest"];

self.addEventListener("install", (event) => {
  event.waitUntil(precacheAppShell());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  if (request.mode === "navigate") {
    event.respondWith(fetch(request).catch(() => caches.match("/") || Response.error()));
    return;
  }

  if (url.pathname === "/api/v1/user/qr") {
    event.respondWith(cacheFirst(request, QR_CACHE));
    return;
  }

  if (url.pathname.startsWith("/api/")) return;

  event.respondWith(cacheFirst(request, RUNTIME_CACHE));
});

async function cacheFirst(request, cacheName) {
  const cached = await caches.match(request);
  if (cached) return cached;
  const response = await fetch(request);
  if (response.ok) {
    const cache = await caches.open(cacheName);
    await cache.put(request, response.clone());
  }
  return response;
}

async function precacheAppShell() {
  const cache = await caches.open(STATIC_CACHE);
  let assetURLs = [];
  try {
    const response = await fetch("/", { cache: "reload" });
    if (response.ok) {
      assetURLs = collectIndexAssetURLs(await response.text());
    }
  } catch {
    assetURLs = [];
  }
  await cache.addAll([...APP_SHELL, ...assetURLs]);
  await self.skipWaiting();
}

function collectIndexAssetURLs(html) {
  const urls = new Set();
  const patterns = [
    /<script\b[^>]*\bsrc=["']([^"']+)["'][^>]*>/gi,
    /<link\b[^>]*\bhref=["']([^"']+)["'][^>]*>/gi
  ];
  for (const pattern of patterns) {
    for (const match of html.matchAll(pattern)) {
      const pathname = sameOriginPath(match[1]);
      if (pathname && !pathname.startsWith("/api/")) {
        urls.add(pathname);
      }
    }
  }
  return Array.from(urls);
}

function sameOriginPath(value) {
  try {
    const url = new URL(value, self.location.origin);
    if (url.origin !== self.location.origin) return "";
    return url.pathname + url.search;
  } catch {
    return "";
  }
}
