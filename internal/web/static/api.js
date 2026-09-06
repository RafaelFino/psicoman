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
