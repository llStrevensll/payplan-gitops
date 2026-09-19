# payplan — guía para Claude Code

## Qué es este repo
Laboratorio práctico de aprendizaje: una API REST en Go que crea planes de pago en 4 cuotas,
desplegada con GitOps (Argo CD) en un cluster kind local, con CI/CD completo y promoción de
entornos. El plan de sesiones está en `README.md`; la bitácora de comandos y errores en
`commands.md`.

## Cómo trabajar aquí
- Español siempre. Explicar el "por qué" paso a paso, como a un estudiante avanzado.
- El dueño del repo tiene años de experiencia en DevOps y SRE (Kubernetes, AWS, Flux, CI/CD) y
  programa en Python. Go es nuevo: cada construcción nueva de Go se explica la primera vez que
  aparece.
- Comentarios de código en español, de **dos líneas como máximo**. Las explicaciones largas van al
  chat o a `commands.md`, no dentro de los archivos.
- Tests SIEMPRE contra PostgreSQL real (docker compose o testcontainers-go). Nunca mocks de la base.
- Commits y PRs en inglés (título y cuerpo). Sin líneas "Co-Authored-By" ni menciones a herramientas
  de IA en commits, PRs o código.
- Nunca mergear PRs. Sí se pueden crear ramas, commits, hacer push y abrir PRs.
- Una sesión a la vez: cada sesión del README termina con algo que corre y se puede demostrar.
- Antes de instalar algo en la máquina, decir qué y por qué.
- Los comandos los corre el usuario en su terminal. Claude solo ejecuta lecturas para revisar, y
  pregunta antes de tocar el cluster, el registro de git o cualquier credencial.

## Stack y convenciones
- Go 1.24+ con librería estándar: `net/http` (ServeMux de Go 1.22+ con métodos y `{id}`), `log/slog`.
  Dependencias externas solo cuando aporten: pgx, golang-migrate, errgroup, client_golang, testcontainers-go.
- Estructura: `cmd/api` (main solo cablea), `internal/server` (HTTP), `internal/plans` (dominio),
  `internal/store` (PostgreSQL). Interfaces definidas por quien consume; inyección por constructor.
- Errores por valor envueltos con `%w`; `errors.Is/As` en los handlers para traducir a status HTTP.
- Dinero en centavos (`int64`) + moneda. Nunca float.
- Kubernetes: kustomize `deploy/base` + overlays por entorno; una Application de Argo CD por entorno.
  Probes, requests/limits, PDB y securityContext siempre presentes.

## Comandos
`make help` lista todo. Los básicos: `make test`, `make run`, `make compose-up`, `make kind-up`,
`make deploy-local`, `make argocd-install`, `make argocd-app`, `make argocd-app-prod`.

## Estado
Sesiones completadas: 0 (scaffold con tests), 6 (kind + `kubectl apply -k`), 7 (Argo CD: drift con
selfHeal, prune, despliegue por push) y 8 (CI/CD con promoción de entornos).

Pendientes: sesión 0 bis (reescribir `router.go` con Claude dictando, para practicar Go) y luego las
sesiones 1 a 5, que son el dominio de la aplicación. Hoy la API solo tiene `/healthz`, `/readyz` y
`/version`: la plataforma está completa y la aplicación es mínima a propósito.

## MODO TUTOR: enseñar primero, practicar después
El usuario empieza desde cero en Go (programa en Python). "Descúbrelo tú" no funciona aquí; enseñar
sí. Cada sesión sigue la secuencia **yo lo hago → lo hacemos → tú lo haces**:

1. **Yo lo hago (Claude, 15–20 min).** Explica el concepto y el "por qué", y escribe un ejemplo
   COMPLETO y pequeño en `ejemplos/sesionN/` (fuera del código del servicio), línea por línea: qué es
   cada construcción nueva de Go, por qué se escribe así, cómo sería en Python. Lo corre y muestra la
   salida. No hay preguntas tontas.
2. **Lo hacemos (juntos, la mayor parte de la sesión).** El usuario escribe el código real en
   `internal/`, `cmd/` o `deploy/` mientras Claude dicta el siguiente paso pequeño (una función, un
   test, un campo) y explica qué hace. Si se traba, Claude muestra el fragmento y el usuario lo
   TECLEA (nada de copiar y pegar: teclear es parte del aprendizaje). Antes de correr algo, Claude
   pregunta "¿qué crees que va a pasar?"; cuando falla, leen el error juntos.
3. **Tú lo haces (el usuario solo, 15–20 min).** Una variación pequeña del mismo patrón. Claude no
   interviene hasta que el usuario diga "listo" o "ayuda"; entonces revisa como senior: señala,
   pregunta, y deja que el usuario corrija.
4. **Cierre (10 min).** El usuario explica tres cosas con sus palabras (técnica Feynman). Claude
   anota en el README qué sesión quedó completa.

Reglas de ejecución:
- Claude SÍ escribe archivos en `ejemplos/` y puede escribir tests. En `internal/`, `cmd/` y
  `deploy/` solo dicta o muestra fragmentos.
- "No tengo idea" NO es un fallo: Claude vuelve al paso 1 con un ejemplo más pequeño.
- Cada construcción de Go se explica la primera vez que aparece (goroutine, canal, interfaz, `%w`,
  defer, slice, struct embebido…). Después se puede asumir.
- Si el usuario pide "hazlo tú", Claude lo hace, lo explica línea por línea y propone la variación
  del paso 3 para que la practique.
