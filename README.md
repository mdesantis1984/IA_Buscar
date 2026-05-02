# IA_Buscar

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-Compatible-FF6B6B?logo=robot)](https://modelcontextprotocol.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Servicio MCP de búsqueda, extracción, síntesis y citación para agentes IA locales. Usa CT-BUSCAR:5000 como motor de conectores.

---

## Resumen

- 13 conectores de búsqueda: web, GitHub, StackOverflow, npm, NuGet, PyPI, DockerHub, Academic, Reddit, YouTube, Images.
- Herramientas de extracción y síntesis de contenido.
- Caché inteligente con TTL e historial.
- Protección SSRF en fetch/extract.
- Integración con IA_Recuerdo para logging.
- 28 tools MCP registradas.

---

## Características principales

| Característica | Descripción |
|---|---|
| Conectores múltiples | 13 conectores para diferentes fuentes de información |
| Búsqueda web | SearxNG para búsqueda web descentralizada |
| APIs especializadas | GitHub, StackOverflow, npm, NuGet, PyPI, DockerHub, Semantic Scholar |
| Fetch/Extract | Extracción de contenido con protección SSRF |
| Síntesis | summarize_results, deep_research, compare_sources |
| Caché |get_cached, invalidate_cache, get_search_history |
| Integración IA | Compatible con IA_Recuerdo para memoria persistente |

---

## Conectores (16 disponibles)

| Conector | Fuente | Descripción |
|---|---|---|
| `search_web` | SearxNG (brave) | Búsqueda web general descentralizada |
| `search_github` | GitHub API | Repositorios, archivos y commits |
| `search_github_pr` | GitHub API | Pull requests |
| `search_github_issue` | GitHub API | Issues |
| `search_stackoverflow` | StackOverflow API | Q&A técnica |
| `search_npm` | npm Registry | Paquetes Node/TypeScript |
| `search_nuget` | NuGet Gallery | Paquetes .NET |
| `search_pypi` | PyPI | Paquetes Python |
| `search_docker_hub` | Docker Hub | Imágenes Docker |
| `search_academic` | SearxNG (arxiv) | Papers y referencias académicas |
| `search_reddit` | Reddit | Discusiones y experiencias reales |
| `search_youtube` | SearxNG (youtube,brave) | Tutoriales y demos |
| `search_images` | SearxNG (bing images) | Diagramas y material visual |
| `search_news` | SearxNG (qwant news) | Noticias y actualidad |

---

## Acceso

| Endpoint | URL | Descripción |
|---|---|---|
| MCP HTTP | `http://<HOST>:5000/mcp` | Protocolo MCP para agentes IA |
| Health | `http://<HOST>:5000/healthz` | Verificación de estado del servicio |

---

## Requisitos

1. CT-BUSCAR:5000 ejecutándose como servicio Go.
2. SearxNG disponible en `<HOST>:8080` para búsqueda web.
3. Acceso a APIs externas: GitHub, StackOverflow, npm, NuGet, PyPI, DockerHub, Semantic Scholar, Reddit, YouTube.
4. IA_Recuerdo (CT-RECUERDO) para logging persistente.

---

## Uso rápido (MCP)

```json
{
  "method": "tools/call",
  "params": {
    "name": "search_web",
    "arguments": {
      "query": "búsqueda de ejemplo",
      "maxResults": 10
    }
  }
}
```

---

## Configuración (ejemplo)

```jsonc
{
  "mcp_server": {
    "host": "<HOST>",
    "port": 5000,
    "transport": "http"
  },
  "searxng": {
    "url": "http://<HOST>:8080"
  },
  "cache": {
    "ttl_seconds": 3600,
    "max_entries": 1000
  },
  "ia_recuerdo": {
    "url": "http://<RECUERDO_HOST>:7438/mcp"
  }
}
```

---

## Arquitectura

```
CT-BUSCAR (Go Service :5000)
  │
  ├─ MCP Handler
  │   └─ 28 Tools registradas
  │
  ├─ Conectores
  │   ├─ search_web ──> SearxNG :8080
  │   ├─ search_github ──> GitHub API
  │   ├─ search_stackoverflow ──> StackOverflow API
  │   ├─ search_npm ──> npm Registry
  │   ├─ search_nuget ──> NuGet Gallery
  │   ├─ search_pypi ──> PyPI
  │   ├─ search_docker_hub ──> Docker Hub
  │   ├─ search_academic ──> Semantic Scholar API
  │   ├─ search_reddit ──> Reddit API
  │   ├─ search_youtube ──> Invidious/YouTube
  │   └─ search_images ──> DuckDuckGo Images
  │
  ├─ Fetch/Extract
  │   ├─ fetch_url
  │   ├─ fetch_and_extract
  │   └─ extract_structured
  │
  ├─ Síntesis
  │   ├─ summarize_results
  │   ├─ deep_research
  │   └─ compare_sources
  │
  └─ Cache Layer
      ├─ get_cached
      ├─ invalidate_cache
      └─ get_search_history
```

---

## Changelog

### 1.1.0 — 2026-05-02
- **SearxNG Migration**: Images, News, YouTube, Academic ahora usan SearxNG en LXC 201 (10.0.0.201:8080).
- Conectores migrados: search_images, search_news, search_youtube, search_academic.
- Se eliminaron APIs deprecated (Invidious, Semantic Scholar).
- Proceso duplicado identificado y resuelto (usuario 100997).
- Deploy: `/opt/ia-buscar/bin/ia-buscar` en LXC 15.

### 1.0.0 — 2026-04-30
- Servicio MCP de búsqueda inicial con 13 conectores.
- 28 tools MCP registradas.
- conectores: search_web, search_github, search_github_pr, search_github_issue, search_stackoverflow, search_npm, search_nuget, search_pypi, search_docker_hub, search_academic, search_reddit, search_youtube, search_images.
- Tools adicionales: fetch_url, fetch_and_extract, extract_structured, validate_url, check_link_status, summarize_results, deep_research, compare_sources, get_cached, invalidate_cache, get_search_history, get_current_date.
- Integración con IA_Recuerdo para logging.
- Protección SSRF en operaciones de fetch.

---

## Seguridad

- Protección SSRF en fetch/extract de URLs.
- Validación de URLs antes de realizar solicitudes.
- Sin telemetría ni envío de datos a terceros fuera de las APIs especificadas.

---

## Licencia

MIT © ThisCloud Services