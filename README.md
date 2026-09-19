# payplan

API REST en Go para planes de pago en 4 cuotas (estilo BNPL), desplegada con **GitOps** sobre un
cluster **kind** local, y con un pipeline de CI/CD completo que promociona entornos.

Es un laboratorio de aprendizaje: cada pieza corresponde a un tema que aparece en entrevistas de
Senior SRE, y cada sesión termina con algo que corre y se puede demostrar.

## Alcance

**La plataforma está completa; la aplicación es mínima a propósito.**

Un push a `main` corre tests contra un PostgreSQL real, publica una imagen multi-arquitectura en el
registry, escribe el nuevo tag en el overlay de desarrollo, y Argo CD despliega sin que nadie toque
`kubectl`. Producción espera una aprobación manual.

La API en sí solo expone `/healthz`, `/readyz` y `/version`. El dominio de planes de pago (sesiones
1 a 5 de este README: reparto de centavos, PostgreSQL, idempotencia, outbox transaccional,
observabilidad) está diseñado aquí pero todavía no implementado.

Todos los comandos usados, con el porqué de cada uno y una tabla de **17 errores reales con su causa
y su arreglo**, están en [`commands.md`](commands.md).

## Qué demuestra

- **Go de librería estándar**: `net/http` con timeouts, apagado ordenado ante SIGTERM, logging
  estructurado con `log/slog`, middleware, tests de tabla con `httptest`.
- **Imagen mínima y segura**: multi-stage sobre distroless, binario estático, usuario no-root con
  UID numérico, `.dockerignore`.
- **Kubernetes con criterio**: probes, requests y limits, `GOMEMLIMIT`, `preStop`,
  `terminationGracePeriodSeconds` mayor que el timeout de apagado, PodDisruptionBudget,
  `securityContext` restrictivo.
- **GitOps con Argo CD**: `selfHeal` revirtiendo cambios hechos a mano, `prune` borrando lo que
  desaparece de git, una `Application` por entorno.
- **CI/CD con promoción de entornos**: el mismo pipeline escrito dos veces, en
  [`.gitlab-ci.yml`](.gitlab-ci.yml) y en [`.github/workflows/ci.yml`](.github/workflows/ci.yml),
  para comparar las dos plataformas lado a lado.
- **Build multi-arquitectura**: `--platform=$BUILDPLATFORM` y `GOARCH=$TARGETARCH`, así un runner
  Intel emite binarios arm64 a velocidad nativa, sin emulación QEMU.

## Empezar

```bash
git clone <este-repo> && cd payplan
./scripts/bootstrap-mac.sh      # go, kind, kubectl, argocd, k6, golang-migrate con brew
make test                        # go vet + tests con -race
make run                         # API en :8080
curl -s localhost:8080/healthz && curl -s localhost:8080/version
```

## Estructura

```
cmd/api/             main: configuración, servidor HTTP, apagado ordenado (SIGTERM)
internal/server/     rutas, middlewares, tests de tabla con httptest
ejemplos/            ejemplos autocontenidos para aprender Go (go vet, detector de carreras)
deploy/base/         Deployment (probes, recursos, securityContext), Service, PDB
deploy/overlays/dev  kustomize: namespace payplan, tag de imagen, NodePort para kind
deploy/overlays/prod kustomize: namespace payplan-prod, 3 réplicas, ClusterIP
deploy/kind/         cluster kind con puerto 18080 → API
argocd/              una Application por entorno (sync automático, prune, selfHeal)
.gitlab-ci.yml       CI/CD en GitLab: test → build multi-arch → deploy-dev auto → deploy-prod manual
.github/workflows/   el mismo pipeline en GitHub Actions
scripts/             bootstrap-mac.sh
commands.md          bitácora de comandos y errores encontrados
```

## Plan de sesiones (1–2 horas cada una)

### Sesión 0 · Scaffold — hecha
Servidor `net/http` con timeouts, `/healthz`, `/readyz`, `/version`, middleware de logging con
`slog` JSON, apagado ordenado, test de tabla, Dockerfile multi-stage distroless, compose con
PostgreSQL 16, kustomize, Application de Argo CD, CI.
**Listo cuando:** `make test` pasa y `curl localhost:8080/version` responde.

### Sesión 1 · Dominio: planes en memoria
- `internal/plans`: `Plan{ID, OrderID, AmountCents, Currency, Installments}` e `Installment{Seq, DueDate, AmountCents, Status}`.
- `Service.Create` divide el monto en 4 cuotas cada 14 días. Ojo con la división de centavos: el
  residuo va a la primera cuota (4999 → 1252 + 1249 + 1249 + 1249).
- Interfaz `Repository` definida en `plans` (quien la consume) + implementación en memoria con `sync.RWMutex`.
- `POST /plans` (valida: monto > 0, moneda ISO de 3 letras) y `GET /plans/{id}`. Errores → 400/404 con `errors.Is/As`.
- Tests de tabla para el reparto de centavos y para los handlers.
**Listo cuando:** creas un plan con `curl` y lo consultas; `go test -race` verde.

### Sesión 2 · PostgreSQL real
- `pgx` + pool (`MaxConns`, `MaxConnLifetime` 5 min: importa con failover de Aurora).
- Migraciones con `golang-migrate` embebidas (`embed.FS`): tablas `plans` e `installments`, índices `(order_id)` y `(due_date, status)`.
- `internal/store/postgres.go` implementa `plans.Repository`. `/readyz` hace ping con timeout de 1 s.
- Tests contra la base de `docker compose` (`DATABASE_URL`); cada test corre en una transacción que se revierte.
**Listo cuando:** `make compose-up`, `DATABASE_URL=... go test ./...` verde y la CI verde.

### Sesión 3 · Idempotencia
- Header `Idempotency-Key` en `POST /plans`: tabla `idempotency_keys(key PK, status, body, created_at)` escrita en la MISMA transacción que el plan.
- Repetir la petición devuelve exactamente la misma respuesta (mismo `id`).
- `POST /plans/{id}/installments/{seq}/pay`: máquina de estados `pending → paid`; pagar dos veces no duplica.
**Listo cuando:** un test repite el POST 3 veces y hay un solo plan en la base.

### Sesión 4 · Outbox transaccional
- Tabla `outbox(id, aggregate_id, type, payload, created_at, published_at)`; insert en la misma transacción que el evento de negocio.
- Relay en goroutine: `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 100`, publica, marca `published_at`.
- Consumidor con `processed_events(event_id PK)` para deduplicar.
- Demostración: mata el relay a mitad (`kill -9`) y verifica que no se pierde ni se duplica.
**Listo cuando:** explicas doble escritura → outbox → at-least-once → idempotencia con tu propio código.

### Sesión 5 · Observabilidad
- `prometheus/client_golang`: contador de peticiones y histograma de duración por ruta y status (RED). Endpoint `/metrics`.
- `slog` con request id; `load/k6.js` con umbrales p99 < 300 ms y errores < 0.1 %.
- `docs/slo.md`: SLI, SLO (99.9 %) y las dos alertas de burn rate (1h/5m a 14.4x, 6h/30m a 6x).
**Listo cuando:** corres k6 contra la API, ves las métricas y encuentras el primer cuello de botella.

### Sesión 6 · Kubernetes local — hecha
```bash
make kind-up          # cluster con el puerto 18080 mapeado
make deploy-local     # build → kind load → kubectl apply -k → rollout
curl localhost:18080/healthz
```
- Observa probes y `preStop`; borra un pod y mira el rolling; baja `limits.memory` a 20Mi para ver un OOMKilled (exit 137) de cerca.
- `kubectl drain` con y sin PDB.
**Listo cuando:** explicas requests/limits, QoS, probes y GOMEMLIMIT con lo que viste.

### Sesión 7 · Argo CD — hecha
```bash
make argocd-install
make argocd-password          # usuario admin
make argocd-ui                # https://localhost:8443
argocd login localhost:8443 --username admin --insecure
# repo privado: Argo CD necesita una credencial de solo lectura, no tu llave personal
ssh-keygen -t ed25519 -f ~/.ssh/payplan_argocd -N "" -C "argocd-kind-payplan"
# registra la .pub como deploy key del repo, luego:
argocd repo add <url-ssh-del-repo> --ssh-private-key-path ~/.ssh/payplan_argocd
make argocd-app        # dev
make argocd-app-prod   # prod
```
- Edita `replicas` a mano con `kubectl` → `selfHeal` lo revierte en segundos. Borra `pdb.yaml` en git → `prune` lo elimina.

| Concepto | Flux | Argo CD |
|---|---|---|
| Fuente git | `GitRepository` | `Application.spec.source` |
| Qué aplicar | `Kustomization` / `HelmRelease` | `Application` (detecta kustomize o Helm por la ruta) |
| Varias apps | Kustomization que apunta a otras | App of Apps o `ApplicationSet` |
| Drift | reconcile por intervalo + prune | `selfHeal` + `prune`; la UI muestra el diff |
| Sync manual | `flux reconcile kustomization x` | `argocd app sync x` |
| Imagen nueva | image-reflector + image-automation | la CI hace commit del tag (o Argo CD Image Updater) |
| Arquitectura | controladores por CRD, sin UI propia | api-server + repo-server + application-controller + redis + UI |

Diferencia clave: Argo CD detecta drift por eventos del cluster y lo revierte en segundos; Flux lo
detecta en su siguiente reconciliación periódica.

**Listo cuando:** demuestras drift, prune y un despliegue por push, y explicas las diferencias con Flux.

### Sesión 8 · CI/CD con promoción de entornos — hecha

Cuatro fases: `test` → `build` → `deploy-dev` → `deploy-prod`.

- **test** corre `go vet` y `go test -race -cover` contra un PostgreSQL real levantado como service.
- **build** solo en la rama por defecto. Publica una imagen multi-arquitectura (amd64 y arm64)
  etiquetada con el SHA corto del commit.
- **deploy-dev** es automático: reescribe `newTag` en el overlay de dev con ese SHA, commitea con
  `[skip ci]` y hace push. Argo CD lo ve y despliega.
- **deploy-prod** es `when: manual`: un botón en la interfaz que hace lo mismo sobre el overlay de
  prod. En GitHub la puerta no va en el YAML, se configura en Settings → Environments → production
  con "Required reviewers".

Lo que hay que preparar una vez, porque el token del job NO puede escribir en el repo:

1. Un token de proyecto con scope `write_repository` y **rol Maintainer** (si `main` está protegida,
   un token de rol Developer será rechazado), guardado como variable de CI enmascarada.
2. Un deploy token de solo lectura del registry, como `imagePullSecret` en cada namespace.

**Listo cuando:** un push a `main` termina con la nueva versión corriendo en dev sin tocar
`kubectl`, y prod sigue en la anterior hasta que alguien aprieta el botón.

### Sesión 9 · Prometheus y alertas (opcional)
- `kube-prometheus-stack` instalado como `Application` Helm de Argo CD.
- `ServiceMonitor` para payplan; reglas de burn rate; dashboard RED en Grafana.

## Convenciones
- Comentarios de código en español, escritos para quien aprende Go, de dos líneas como máximo.
- Tests siempre contra PostgreSQL real, nunca mocks de la base.
- Commits y PRs en inglés.
- Dinero en centavos (`int64`), nunca float.

## Licencia

[PolyForm Noncommercial 1.0.0](LICENSE). Puedes usar, modificar y redistribuir este trabajo
libremente **para cualquier fin no comercial**: aprender, enseñar, investigar, proyectos personales
y organizaciones sin ánimo de lucro.

Cualquier uso comercial requiere una licencia aparte. Para pedirla, abre un issue titulado
"Solicitud de licencia comercial" indicando quién eres y cómo piensas usar el trabajo.

PolyForm es un conjunto de licencias redactadas por abogados especializados en software, en lenguaje
llano. No son licencias de código abierto: reservan el uso comercial al autor.
