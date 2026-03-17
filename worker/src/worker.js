// shareiscare — Cloudflare Worker + Durable Object
// Usa WebSocket Hibernation API para evitar cargos de duración cuando está idle.
//
// Diferencia vs accept() normal:
//   - Normal:      server.accept() → DO vive (y se cobra) mientras el WS esté abierto
//   - Hibernation: state.acceptWebSocket() → DO duerme entre mensajes, solo se
//                  cobra CPU cuando hay actividad real

export class TunnelDO {
  constructor(state) {
    this.state   = state;
    this.pending = new Map(); // requestId → { resolve, reject, timer }
  }

  async fetch(request) {
    const url       = new URL(request.url);
    const isConnect = url.pathname === "/__tunnel_connect";
    const isUpgrade = request.headers.get("Upgrade") === "websocket";

    if (isConnect && isUpgrade) {
      return this.#handleClientConnect(request);
    }

    return this.#handleBrowserRequest(request, url);
  }

  // ── Cliente local se conecta ─────────────────────────────────────────────────
  async #handleClientConnect(request) {
    const url = new URL(request.url);
    const secret = url.searchParams.get("secret");

    if (!secret) {
      return new Response("secret required", { status: 401 });
    }

    // Cargar o almacenar secreto de forma durable.
    const stored = await this.state.storage.get("tunnelSecret");
    if (stored && stored !== secret) {
      return new Response("invalid secret", { status: 403 });
    }
    if (!stored) {
      await this.state.storage.put("tunnelSecret", secret);
    }

    // Cerrar conexiones previas (permite reconexión limpia del mismo cliente).
    const existing = this.state.getWebSockets("tunnel");
    for (const ws of existing) {
      try { ws.close(1000, "replaced"); } catch (_) {}
    }

    // Limpiar pending de conexiones anteriores.
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel replaced"));
    }
    this.pending.clear();

    const [client, server] = Object.values(new WebSocketPair());
    this.state.acceptWebSocket(server, ["tunnel"]);
    return new Response(null, { status: 101, webSocket: client });
  }

  // ── Browser HTTP → proxy al cliente ─────────────────────────────────────────
  async #handleBrowserRequest(request, url) {
    const tunnelSockets = this.state.getWebSockets("tunnel");

    if (tunnelSockets.length === 0) {
      return new Response(
        "No hay cliente conectado. Corre shareiscare en tu máquina local.",
        { status: 503, headers: { "Content-Type": "text/plain; charset=utf-8" } }
      );
    }

    const ws      = tunnelSockets[0];
    const id      = crypto.randomUUID();
    const body    = await request.arrayBuffer();
    const bodyB64 = body.byteLength > 0
      ? btoa(String.fromCharCode(...new Uint8Array(body)))
      : null;

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

  // ── Hibernation handlers ─────────────────────────────────────────────────────
  // El runtime invoca estos métodos despertando el DO solo el tiempo necesario.

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

  webSocketClose(ws, code, reason) {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel closed"));
    }
    this.pending.clear();
    try { ws.close(code, reason); } catch (_) {}
  }

  webSocketError(ws) {
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel error"));
    }
    this.pending.clear();
  }
}

// ── Router ───────────────────────────────────────────────────────────────────
export default {
  async fetch(request, env) {
    const host = new URL(request.url).hostname;
    const hash = host.split(".")[0];

    if (!hash || hash === "shareiscare") {
      return new Response(
        "Especifica un subdominio: https://<hash>.shareiscare.dev",
        { status: 400, headers: { "Content-Type": "text/plain" } }
      );
    }

    const doId   = env.TUNNEL.idFromName(hash);
    const tunnel = env.TUNNEL.get(doId);
    return tunnel.fetch(request);
  },
};
