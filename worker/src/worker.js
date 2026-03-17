// shareiscare — Cloudflare Worker + Durable Object
// Uses WebSocket Hibernation API to avoid duration charges when idle.
//
// Protections:
//   - Rate limit per IP (configurable via RATE_LIMIT_RPM)
//   - Hash validation (only [a-z0-9]{4,64})
//   - Request body size limit (configurable via MAX_REQUEST_BODY_MB)
//   - Active tunnel limit (configurable via MAX_TUNNELS, uses registry DO)
//   - Configurable cleanup time (via CLEANUP_HOURS)

// ── Defaults (overridable via env vars in wrangler.toml) ─────────────────────
const DEFAULTS = {
  RATE_LIMIT_RPM: 60,
  MAX_REQUEST_BODY_MB: 10,
  MAX_TUNNELS: 50,
  CLEANUP_HOURS: 24,
};

// ── Rate limiter (best-effort, in-memory per isolate) ────────────────────────
const rateLimits = new Map();

function checkRateLimit(ip, maxRpm) {
  const now = Date.now();
  let entry = rateLimits.get(ip);
  if (!entry || now > entry.resetAt) {
    entry = { count: 0, resetAt: now + 60_000 };
    rateLimits.set(ip, entry);
  }
  entry.count++;
  // Evict stale entries when the map grows too large.
  if (rateLimits.size > 10_000) {
    for (const [k, v] of rateLimits) {
      if (now > v.resetAt) rateLimits.delete(k);
    }
  }
  return entry.count <= maxRpm;
}

// ── Hash validation ──────────────────────────────────────────────────────────
const HASH_RE = /^[a-z0-9]{4,64}$/;

// ── Utility for safe base64 encoding with large bodies ───────────────────────
function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let str = "";
  const chunk = 8192;
  for (let i = 0; i < bytes.length; i += chunk) {
    str += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
  }
  return btoa(str);
}

export class TunnelDO {
  constructor(state, env) {
    this.state   = state;
    this.env     = env;
    this.pending = new Map(); // requestId → { resolve, reject, timer }
  }

  async fetch(request) {
    const url = new URL(request.url);

    // ── Registry API (internal, DO-to-DO only) ───────────────────────────
    if (url.pathname === "/__registry/check") {
      return this.#registryCheck();
    }
    if (url.pathname === "/__registry/release") {
      return this.#registryRelease();
    }

    const isConnect = url.pathname === "/__tunnel_connect";
    const isUpgrade = request.headers.get("Upgrade") === "websocket";

    if (isConnect && isUpgrade) {
      return this.#handleClientConnect(request);
    }

    return this.#handleBrowserRequest(request, url);
  }

  // ── Registry: controls active tunnel limit ─────────────────────────────
  async #registryCheck() {
    const count = (await this.state.storage.get("activeTunnels")) || 0;
    const max = parseInt(this.env.MAX_TUNNELS) || DEFAULTS.MAX_TUNNELS;
    if (count >= max) {
      return new Response(JSON.stringify({ count, max }), { status: 429 });
    }
    await this.state.storage.put("activeTunnels", count + 1);
    return new Response(JSON.stringify({ count: count + 1, max }), { status: 200 });
  }

  async #registryRelease() {
    const count = (await this.state.storage.get("activeTunnels")) || 0;
    if (count > 0) {
      await this.state.storage.put("activeTunnels", count - 1);
    }
    return new Response("ok");
  }

  // ── Local client connects ──────────────────────────────────────────────
  async #handleClientConnect(request) {
    const url = new URL(request.url);
    const secret = url.searchParams.get("secret");

    if (!secret) {
      return new Response("secret required", { status: 401 });
    }

    // Load or store secret durably.
    const stored = await this.state.storage.get("tunnelSecret");
    if (stored && stored !== secret) {
      return new Response("invalid secret", { status: 403 });
    }
    if (!stored) {
      // New tunnel — check limit via registry DO.
      const registryId = this.env.TUNNEL.idFromName("__registry__");
      const registry = this.env.TUNNEL.get(registryId);
      const res = await registry.fetch(new Request("https://internal/__registry/check"));
      if (res.status === 429) {
        return new Response("tunnel limit reached", { status: 429 });
      }
      await this.state.storage.put("tunnelSecret", secret);
    }

    // Cancel cleanup alarm if pending (client reconnected in time).
    await this.state.storage.deleteAlarm();

    // Close existing connections (allows clean reconnection from the same client).
    const existing = this.state.getWebSockets("tunnel");
    for (const ws of existing) {
      try { ws.close(1000, "replaced"); } catch (_) {}
    }

    // Clear pending requests from previous connections.
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel replaced"));
    }
    this.pending.clear();

    const [client, server] = Object.values(new WebSocketPair());
    this.state.acceptWebSocket(server, ["tunnel"]);
    return new Response(null, { status: 101, webSocket: client });
  }

  // ── Browser HTTP → proxy to client ─────────────────────────────────────
  async #handleBrowserRequest(request, url) {
    const tunnelSockets = this.state.getWebSockets("tunnel");

    if (tunnelSockets.length === 0) {
      return new Response(
        "No client connected. Run shareiscare on your local machine.",
        { status: 503, headers: { "Content-Type": "text/plain; charset=utf-8" } }
      );
    }

    const ws      = tunnelSockets[0];
    const id      = crypto.randomUUID();
    const body    = await request.arrayBuffer();

    // Request body size limit.
    const maxBodyMB = parseInt(this.env.MAX_REQUEST_BODY_MB) || DEFAULTS.MAX_REQUEST_BODY_MB;
    const maxBody = maxBodyMB * 1024 * 1024;
    if (body.byteLength > maxBody) {
      return new Response("Request body too large", { status: 413 });
    }

    const bodyB64 = body.byteLength > 0 ? arrayBufferToBase64(body) : null;

    const headers = {};
    for (const [k, v] of request.headers) headers[k] = v;

    const responsePromise = new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("timeout"));
      }, 30_000);
      this.pending.set(id, { resolve, reject, timer });
    });

    ws.send(JSON.stringify({
      id,
      method:  request.method,
      path:    url.pathname + url.search,
      headers,
      body:    bodyB64,
    }));

    let resp;
    try {
      resp = await responsePromise;
    } catch (e) {
      const isTout = e.message === "timeout";
      return new Response(isTout ? "Gateway Timeout" : "Tunnel Closed", {
        status: isTout ? 504 : 502,
      });
    }

    const respBody = resp.body
      ? Uint8Array.from(atob(resp.body), c => c.charCodeAt(0))
      : null;

    return new Response(respBody, {
      status:  resp.status,
      headers: resp.headers ?? {},
    });
  }

  // ── Hibernation handlers ───────────────────────────────────────────────
  // The runtime invokes these methods, waking the DO only as long as needed.

  webSocketMessage(ws, data) {
    try {
      const msg = JSON.parse(data);
      if (msg.type === "response") {
        const p = this.pending.get(msg.id);
        if (p) {
          clearTimeout(p.timer);
          this.pending.delete(msg.id);
          p.resolve(msg);
        }
      }
    } catch (_) {}
  }

  #scheduleCleanup() {
    const hours = parseInt(this.env?.CLEANUP_HOURS) || DEFAULTS.CLEANUP_HOURS;
    this.state.storage.setAlarm(Date.now() + hours * 60 * 60 * 1000);
  }

  webSocketClose(ws, code, reason) {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel closed"));
    }
    this.pending.clear();
    try { ws.close(code, reason); } catch (_) {}

    // Schedule cleanup if no one reconnects.
    this.#scheduleCleanup();
  }

  webSocketError(ws) {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel error"));
    }
    this.pending.clear();

    // Schedule cleanup if no one reconnects.
    this.#scheduleCleanup();
  }

  // ── Auto-cleanup of orphan DOs ─────────────────────────────────────────
  // If the alarm fires, it means no one reconnected in time.
  // Decrement the registry and delete storage for DO garbage collection.
  async alarm() {
    const tunnelSockets = this.state.getWebSockets("tunnel");
    if (tunnelSockets.length === 0) {
      // Notify the registry before cleaning up.
      try {
        const registryId = this.env.TUNNEL.idFromName("__registry__");
        const registry = this.env.TUNNEL.get(registryId);
        await registry.fetch(new Request("https://internal/__registry/release"));
      } catch (_) {}
      await this.state.storage.deleteAll();
    }
  }
}

// ── Router ───────────────────────────────────────────────────────────────────
export default {
  async fetch(request, env) {
    const host = new URL(request.url).hostname;
    const hash = host.split(".")[0].toLowerCase();

    if (!hash || hash === "shareiscare") {
      return new Response(
        "Specify a subdomain: https://<hash>.shareiscare.dev",
        { status: 400, headers: { "Content-Type": "text/plain" } }
      );
    }

    // Validate hash format.
    if (!HASH_RE.test(hash)) {
      return new Response(
        "Invalid hash: must be 4-64 lowercase alphanumeric characters",
        { status: 400, headers: { "Content-Type": "text/plain" } }
      );
    }

    // Rate limit per IP.
    const ip = request.headers.get("CF-Connecting-IP") || "unknown";
    const maxRpm = parseInt(env.RATE_LIMIT_RPM) || DEFAULTS.RATE_LIMIT_RPM;
    if (!checkRateLimit(ip, maxRpm)) {
      return new Response("Rate limit exceeded", {
        status: 429,
        headers: { "Retry-After": "60" },
      });
    }

    const doId   = env.TUNNEL.idFromName(hash);
    const tunnel = env.TUNNEL.get(doId);
    return tunnel.fetch(request);
  },
};
