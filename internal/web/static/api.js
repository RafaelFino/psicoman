// Psicoman — cliente JS da API JSON.
// A API responde sempre com o envelope { message, elapsed_ms, data, error, request_id }.
// Este helper centraliza fetch, envio de credenciais (cookie de sessão do portal)
// e o desembrulho do envelope, lançando ApiError em falhas.

const Api = (() => {
  class ApiError extends Error {
    constructor(message, status, code, detail) {
      super(message || "Erro inesperado.");
      this.name = "ApiError";
      this.status = status;
      this.code = code;
      this.detail = detail;
    }
  }

  async function request(method, path, body) {
    const opts = {
      method,
      // same-origin envia o cookie HttpOnly de sessão do portal.
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    };
    if (body !== undefined && body !== null) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }

    let resp;
    try {
      resp = await fetch(path, opts);
    } catch (netErr) {
      throw new ApiError("Falha de rede. Verifique sua conexão.", 0, "network", String(netErr));
    }

    let env = {};
    const text = await resp.text();
    if (text) {
      try {
        env = JSON.parse(text);
      } catch {
        // Resposta não-JSON (ex: erro de proxy): trata como erro genérico.
        if (!resp.ok) {
          throw new ApiError("Resposta inválida do servidor.", resp.status, "bad_response", text);
        }
      }
    }

    if (!resp.ok) {
      const err = env.error || {};
      throw new ApiError(env.message || "Erro na requisição.", resp.status, err.code, err.detail);
    }

    return { message: env.message, data: env.data };
  }

  return {
    ApiError,
    get: (path) => request("GET", path),
    post: (path, body) => request("POST", path, body),
    put: (path, body) => request("PUT", path, body),
    del: (path) => request("DELETE", path),
  };
})();

// Componente Alpine reutilizável para blocos com estado de carregamento/feedback.
// Uso: x-data="panel()" e chame this.run(fn) dentro de handlers.
function panel() {
  return {
    loading: false,
    feedback: null, // { kind: 'ok'|'attention', text: string }
    ok(text) {
      this.feedback = { kind: "ok", text };
    },
    warn(text) {
      this.feedback = { kind: "attention", text };
    },
    clearFeedback() {
      this.feedback = null;
    },
    // run executa uma ação assíncrona controlando loading e traduzindo ApiError.
    async run(fn) {
      this.loading = true;
      this.clearFeedback();
      try {
        return await fn();
      } catch (e) {
        if (e instanceof Api.ApiError) {
          this.warn(e.message + (e.detail ? " (" + e.detail + ")" : ""));
        } else {
          this.warn("Erro inesperado: " + (e && e.message ? e.message : String(e)));
        }
        return undefined;
      } finally {
        this.loading = false;
      }
    },
  };
}

// Helpers de formatação.
const Fmt = {
  // Centavos (int) -> BRL. A API usa amount em centavos (int64).
  brl(cents) {
    if (cents === null || cents === undefined) return "—";
    return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
  },
  // ISO-8601 -> data/hora local pt-BR.
  datetime(iso) {
    if (!iso) return "—";
    const d = new Date(iso);
    if (isNaN(d.getTime())) return String(iso);
    return d.toLocaleString("pt-BR");
  },
  // Escapa texto para inserção segura (usado só quando necessário; Alpine já escapa via x-text).
  esc(s) {
    const div = document.createElement("div");
    div.textContent = s == null ? "" : String(s);
    return div.innerHTML;
  },
};

// Renderizador Markdown client-side.
// Espelha o subconjunto do servidor (internal/platform/markdown): cabeçalhos,
// listas não ordenadas, **negrito**, *itálico*, parágrafos. Acrescenta código
// inline (`code`), links [texto](url) e blocos de código (``` … ```), sempre
// escapando HTML antes de aplicar a formatação (segurança contra XSS).
const Md = (() => {
  function escapeHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  // Formatação inline aplicada após o escape de HTML.
  function inline(s) {
    let out = escapeHtml(s);
    // Código inline primeiro (protege o conteúdo de outras regras).
    out = out.replace(/`([^`]+)`/g, (_, c) => `<code>${c}</code>`);
    out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    out = out.replace(/\*([^*]+)\*/g, "<em>$1</em>");
    // Links [texto](http…): só http/https/mailto para evitar javascript:.
    out = out.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+|mailto:[^\s)]+)\)/g,
      (_, txt, url) => `<a href="${url}" target="_blank" rel="noopener">${txt}</a>`);
    return out;
  }

  function toHtml(md) {
    if (!md) return "";
    const lines = String(md).replace(/\r\n/g, "\n").split("\n");
    const out = [];
    let inList = false;
    let inCode = false;
    let para = [];
    let code = [];

    const flushPara = () => {
      if (para.length) { out.push(`<p>${inline(para.join(" "))}</p>`); para = []; }
    };
    const closeList = () => { if (inList) { out.push("</ul>"); inList = false; } };

    for (const raw of lines) {
      const line = raw.replace(/\s+$/, "");
      const trimmed = line.trim();

      // Bloco de código cercado por ```
      if (trimmed.startsWith("```")) {
        if (inCode) { out.push(`<pre class="code">${escapeHtml(code.join("\n"))}</pre>`); code = []; inCode = false; }
        else { flushPara(); closeList(); inCode = true; }
        continue;
      }
      if (inCode) { code.push(raw); continue; }

      if (trimmed === "") { flushPara(); closeList(); continue; }

      if (trimmed.startsWith("#")) {
        flushPara(); closeList();
        let level = 0;
        while (level < trimmed.length && trimmed[level] === "#") level++;
        if (level > 6) level = 6;
        const text = trimmed.slice(level).trim();
        out.push(`<h${level}>${inline(text)}</h${level}>`);
        continue;
      }

      if (trimmed.startsWith("- ") || trimmed.startsWith("* ")) {
        flushPara();
        if (!inList) { out.push("<ul>"); inList = true; }
        out.push(`<li>${inline(trimmed.slice(2).trim())}</li>`);
        continue;
      }

      closeList();
      para.push(trimmed);
    }
    if (inCode) out.push(`<pre class="code">${escapeHtml(code.join("\n"))}</pre>`);
    flushPara();
    closeList();
    return out.join("\n");
  }

  return { toHtml };
})();

// Componente Alpine reutilizável: editor de texto com Markdown.
// Uso:  x-data="mdField('conteúdo inicial')"  e faça x-model no textarea via
// o próprio state.value; ou bind manual. Fornece o toggle escrever/visualizar.
function mdField(initial) {
  return {
    value: initial || "",
    preview: false,
    get html() { return Md.toHtml(this.value); },
    toggle() { this.preview = !this.preview; },
  };
}
