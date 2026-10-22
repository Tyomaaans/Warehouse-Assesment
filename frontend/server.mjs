import process from 'node:process';
import { URL } from 'node:url';
import { setTimeout } from 'node:timers';
import http from 'node:http';
import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import path from 'node:path';

const root = path.resolve(process.env.ASSET_DIR ?? '/app/dist');
const port = Number(process.env.PORT ?? 8080);
const upstream = new URL(process.env.API_PROXY_TARGET ?? 'http://api:8080');
if (upstream.protocol !== 'http:' || upstream.pathname !== '/') throw new Error('Invalid API upstream');
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.woff2': 'font/woff2' };
const server = http.createServer(async (request, response) => {
  response.setHeader('X-Content-Type-Options', 'nosniff');
  response.setHeader('Referrer-Policy', 'same-origin');
  response.setHeader('X-Frame-Options', 'DENY');
  response.setHeader('Content-Security-Policy', "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
  const pathname = new URL(request.url, 'http://localhost').pathname;
  if (pathname === '/health') { response.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); response.end('{"status":"ok"}'); return; }
  // Same-origin local/container use. Host Nginx routes production /api/v1/ directly to API.
  // Never relay client-supplied forwarding headers through this additional hop.
  if (pathname === '/api/v1' || pathname.startsWith('/api/v1/')) {
    const headers = { ...request.headers, host: upstream.host };
    for (const key of Object.keys(headers)) if (key.startsWith('x-forwarded-') || ['forwarded', 'x-real-ip', 'connection', 'upgrade', 'proxy-authorization'].includes(key)) delete headers[key];
    const proxy = http.request(upstream.origin + request.url, { method: request.method, headers, timeout: 20_000 }, result => {
      response.writeHead(result.statusCode, result.headers); result.pipe(response);
    });
    proxy.on('timeout', () => proxy.destroy());
    proxy.on('error', () => { if (!response.headersSent) response.writeHead(503, { 'Content-Type': 'application/json' }); response.end('{"success":false,"error":{"code":"unavailable","message":"Service temporarily unavailable"}}'); });
    request.on('aborted', () => proxy.destroy()); request.pipe(proxy); return;
  }
  if (!['GET', 'HEAD'].includes(request.method)) { response.writeHead(405, { Allow: 'GET, HEAD' }); response.end(); return; }
  try {
    const decoded = decodeURIComponent(pathname);
    if (decoded.includes('\0') || decoded.split('/').some(part => part.startsWith('.'))) throw new Error('Invalid asset');
    let filename = path.resolve(root, '.' + decoded);
    if (!filename.startsWith(root + '/') && filename !== root) throw new Error('Invalid asset');
    let info = await stat(filename).catch(() => null);
    if (!info?.isFile()) {
      if (path.extname(decoded) || decoded.startsWith('/assets/')) { response.writeHead(404); response.end(); return; }
      filename = path.join(root, 'index.html'); info = await stat(filename);
    }
    response.writeHead(200, { 'Content-Type': types[path.extname(filename)] ?? 'application/octet-stream', 'Content-Length': info.size, 'Cache-Control': filename.endsWith('index.html') ? 'no-store' : decoded.startsWith('/assets/') ? 'public, max-age=31536000, immutable' : 'no-cache' });
    if (request.method === 'HEAD') response.end(); else createReadStream(filename).on('error', () => response.destroy()).pipe(response);
  } catch { response.writeHead(404); response.end(); }
});
server.requestTimeout = 30_000;
server.headersTimeout = 10_000;
server.keepAliveTimeout = 5_000;
server.listen(port, '0.0.0.0');
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => { server.close(() => process.exit(0)); setTimeout(() => process.exit(1), 10_000).unref(); });
