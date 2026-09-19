# Comandos del laboratorio

Bitácora de los comandos que usamos sesión a sesión, con el "por qué" al lado. Sirve para repetir
el laboratorio desde cero en otra máquina y para repasar antes de la entrevista.

Datos del proyecto que aparecen en varios comandos:

| Dato | Valor |
|---|---|
| Repo GitLab | `git@gitlab.com:<usuario>/payplan.git` |
| ID del proyecto GitLab | `<project-id>` |
| Container registry | `registry.gitlab.com/<usuario>/payplan` (siempre en minúsculas) |
| Repo GitHub (respaldo) | remoto `github`, `git@github-personal:<usuario>/payplan.git` |
| Cluster kind | `payplan`, API expuesta en `localhost:18080` |

---

## 1. Entorno en la Mac

```bash
./scripts/bootstrap-mac.sh          # go, kind, kubectl, argocd, k6, golang-migrate por brew
```

El script es idempotente: salta lo ya instalado. Usa `set -euo pipefail`, así que si una instalación
falla el script entero aborta (fallar rápido y ruidoso) y basta con volver a correrlo.

### Elegir la versión de kubectl con asdf

Docker Desktop instala su propio `kubectl` en `/usr/local/bin`, que va antes de `/opt/homebrew/bin`
en el PATH y por eso gana. asdf pone sus shims en la primera posición del PATH, así que gana sobre
ambos y permite fijar la versión exacta.

```bash
which -a kubectl                    # ver todas las copias y cuál gana
asdf plugin add kubectl
asdf list all kubectl | tail -8
asdf install kubectl 1.37.0
asdf set -u kubectl 1.37.0          # -u escribe en ~/.tool-versions (global)
```

Sin `-u` escribe un `.tool-versions` en la carpeta actual, que es como se fija una versión por
proyecto. Un `.tool-versions` local siempre gana sobre el global.

### Identidad de git solo en este repo

```bash
git config user.name "<tu nombre>"
git config user.email "<correo personal>"
```

Sin `--global`, para que los commits de este repo no salgan con la cuenta empresarial.

---

## 2. Go y tests

```bash
make help                           # lista todos los targets
make run                            # API en :8080
make test                           # go vet + go test -race -cover
make build                          # binario en bin/api con la versión de git inyectada
```

### Qué hace cada pieza de `make test`

```bash
go vet ./...                        # análisis estático: código que compila pero huele a error
go test -race -cover ./...          # tests + detector de carreras + porcentaje de cobertura
```

- **`go vet`** atrapa lo que el compilador deja pasar: un `%d` con un string, código inalcanzable.
  Devuelve exit 1 si encuentra algo, por eso va antes de los tests en el Makefile.
- **`-race`** recompila con instrumentación que vigila cada acceso a memoria y reporta
  `WARNING: DATA RACE` con la línea exacta. Hace el programa 5 a 10 veces más lento: solo en
  tests y CI, nunca en producción. Sale con código 66 si encuentra carreras.
- **`-cover`** imprime el porcentaje de *sentencias* ejecutadas por los tests. Mide qué se ejecutó,
  no qué se verificó: un test que llama a todo sin comprobar nada da 100 %.

### Ver la cobertura en detalle

```bash
go test -coverprofile=cover.out ./internal/server
go tool cover -func=cover.out       # porcentaje por función
go tool cover -html=cover.out       # código pintado en el navegador: verde ejecutado, rojo no
```

`cover.out` es un archivo de datos crudos, se regenera en un segundo y cambia en cada corrida:
va al `.gitignore`, nunca al repo.

### Ejemplos de la sesión 0 bis

```bash
go vet ./ejemplos/sesion0bis/vet/main.go      # dos hallazgos a propósito
go run ./ejemplos/sesion0bis/vet/main.go      # imprime basura sin explotar
go run ./ejemplos/sesion0bis/carrera          # da un número distinto cada vez
go run -race ./ejemplos/sesion0bis/carrera    # el detector señala la línea del contador++
```

Nota de sintaxis: prefija siempre con `./` cuando te refieras a un archivo o carpeta del disco.
Sin `./`, Go lo interpreta como ruta de import y lo busca en la librería estándar
(`package ... is not in std`).

### Ver el apagado ordenado

```bash
make run                            # en una terminal
curl -s localhost:8080/version      # en otra
# Ctrl+C en la primera
echo $?
```

Con el binario directo (`./bin/api`) el código de salida es 0 porque el proceso atrapa SIGTERM y
llama a `Shutdown`. Con `make run` sale 1 porque `go run` se interrumpe y `make` reporta el fallo
de su hijo. Un proceso que muere por una señal sin atraparla deja `128 + número de señal`: 130 para
SIGINT, 137 para SIGKILL (el OOMKilled o el fin del grace period en Kubernetes).

---

## 3. Docker y compose

```bash
make docker-build                   # imagen multi-stage sobre distroless
make compose-up                     # Postgres 16 + API
make compose-down                   # apaga y borra el volumen
```

---

## 4. kind y Kubernetes (sesión 6)

```bash
make kind-up                        # cluster de un nodo, puerto 30080 del nodo → 18080 del host
make kind-load                      # build + carga la imagen dentro de kind, sin registry
make deploy-local                   # kind-load + kubectl apply -k + rollout status
make kind-down                      # borra el cluster
```

```bash
curl -s localhost:18080/version
kubectl -n payplan get pods,svc,pdb
kubectl kustomize deploy/overlays/dev          # renderiza sin aplicar
```

`make kind-load TAG=v2` construye y carga la imagen con otro tag. La versión que queda dentro del
binario sale de `git describe --tags --always --dirty`: si el árbol de trabajo tiene cambios sin
commitear, el tag lleva sufijo `-dirty` y esa imagen no es reproducible por nadie. Por eso las
imágenes de producción se construyen en CI desde un checkout limpio.

---

## 5. Argo CD (sesión 7)

```bash
make argocd-install                 # namespace argocd + manifiesto estable + espera el rollout
make argocd-password                # contraseña inicial del usuario admin
make argocd-ui                      # port-forward a https://localhost:8443 (bloquea la terminal)
make argocd-app                     # aplica argocd/application.yaml
```

```bash
argocd login localhost:8443 --username admin --insecure
argocd repo list
argocd app get payplan-dev
argocd app get payplan-dev --refresh        # fuerza leer git sin esperar el intervalo (3 min)
argocd app history payplan-dev
argocd app sync payplan-dev                 # sincronización manual
```

El token de sesión de la CLI **expira a las 24 horas**: cuando veas
`invalid session: token has invalid claims: token is expired`, repite el `argocd login`.

### Credencial de solo lectura para el repo

Una deploy key está atada a un solo repo, es de solo lectura y se revoca sin tocar tu llave
personal. Es lo correcto para un repo privado.

```bash
ssh-keygen -t ed25519 -f ~/.ssh/payplan_argocd -N "" -C "argocd-kind-payplan"
gh repo deploy-key add ~/.ssh/payplan_argocd.pub -R <usuario>/payplan -t "argocd read-only"
argocd repo add git@github.com:<usuario>/payplan.git --ssh-private-key-path ~/.ssh/payplan_argocd
```

La URL debe ser idéntica byte por byte a `spec.source.repoURL` de la Application: así empareja Argo
la credencial con el repo. La llave queda en un Secret del namespace `argocd` con la etiqueta
`argocd.argoproj.io/secret-type=repository`, en base64 y no cifrada:

```bash
kubectl -n argocd get secret -l argocd.argoproj.io/secret-type=repository
kubectl -n argocd describe secret -l argocd.argoproj.io/secret-type=repository   # claves, no valores
```

### Las tres demostraciones

```bash
# 1. Drift: selfHeal revierte un cambio hecho a mano, en segundos
kubectl -n payplan get deploy payplan-api -w      # dejar corriendo ANTES
kubectl -n payplan scale deploy/payplan-api --replicas=5
kubectl -n payplan describe deploy payplan-api | grep -A8 '^Events'

# 2. Prune: borrar de git borra del cluster
git rm deploy/base/pdb.yaml                       # y quitar la línea del kustomization de base
git commit -am "Remove PDB" && git push
argocd app get payplan-dev --refresh && kubectl -n payplan get pdb

# 3. Despliegue por push: cambiar newTag en el overlay, commit, push. Sin kubectl.
```

### Piezas de Argo CD y su equivalente en Flux

```bash
kubectl -n argocd get deploy,sts
```

| Argo CD | Hace | Flux |
|---|---|---|
| `argocd-repo-server` | clona git, corre kustomize o helm | `source-controller` |
| `argocd-application-controller` | compara, aplica, detecta drift | `kustomize-controller` |
| `argocd-server` | API y UI | no existe |
| `argocd-redis` | caché de manifiestos | no existe |
| `argocd-applicationset-controller` | genera Applications | Kustomization que apunta a otras |
| `argocd-notifications-controller` | avisos | `notification-controller` |

Diferencia clave: Argo detecta drift por eventos del cluster (informers) y lo revierte en segundos.
Flux lo detecta en su siguiente reconciliación periódica.

---

## 6. GitLab (sesión 8)

### CLI y autenticación

```bash
brew install glab gitlab-runner
glab auth login --hostname gitlab.com        # el --hostname evita que detecte mal los remotos
```

Sin `--hostname`, `glab` escanea los remotos de git y puede confundir un alias SSH
(`github-personal`) con una instancia de GitLab autoalojada, y termina pidiendo un Application ID
de OAuth que no existe.

### Crear el proyecto

```bash
glab api --method POST projects --field name=payplan --field visibility=private
```

Uso la API en vez de `glab repo create` porque ese comando toca los remotos locales.
**La respuesta trae `runners_token`**: filtra la salida o rótalo después.

```bash
glab api --method POST projects/<project-id>/runners/reset_registration_token
```

### SSH hacia GitLab

```bash
glab ssh-key add ~/.ssh/id_ed25519_personal.pub --title "MacBook Air personal"
ssh -T git@gitlab.com                        # debe saludar con @<usuario>
```

`id_ed25519_personal` no es un nombre que ssh pruebe por defecto, así que hace falta un bloque en
`~/.ssh/config`:

```
Host gitlab.com
  HostName gitlab.com
  User git
  IdentityFile ~/.ssh/id_ed25519_personal
  IdentitiesOnly yes
```

La huella ED25519 de gitlab.com es `SHA256:eUXGGm1YGsMAS7vkcx6JOJdOGHPem5gQp4taiCfCLB8`.
Verifícala contra la documentación oficial antes de aceptarla: decir `yes` a ciegas es el hueco
por donde entra un ataque de intermediario.

### Mover los remotos

GitLab pasa a ser `origin` y GitHub queda como `github`, para que un `git push` a secas dispare el
pipeline de GitLab.

```bash
git remote rename origin github
git remote add origin git@gitlab.com:<usuario>/payplan.git
git push -u origin main
git remote -v
```

### Elegir runner: gestionados o propio

**Decisión del laboratorio: los runners gestionados de gitlab.com.** Un runner propio enseña el
registro y los executors, pero en la vida real los administra un equipo de plataforma o los da el
proveedor, y tú solo eliges por tags.

```bash
glab api --method PUT projects/<project-id> --field shared_runners_enabled=true
glab api "projects/<project-id>/runners?type=instance_type&per_page=100" \
  | python3 -c 'import sys,json;[print("%-16s %s" % (r["status"], r["description"])) for r in json.load(sys.stdin)]'
```

Familias disponibles en gitlab.com:

| Familia | Arquitectura | Para qué |
|---|---|---|
| `saas-linux-small-amd64` | Intel, 2 vCPU | el que corre por defecto si no pones tags; el más barato |
| `saas-linux-medium-amd64` … `2xlarge` | Intel, más grandes | compilaciones pesadas, multiplican el consumo de minutos |
| `saas-linux-medium-amd64-gpu-standard` | Intel + GPU | entrenar modelos |
| `saas-linux-medium-arm64` | ARM | builds arm64 nativos |
| `saas-macos-medium-m1` | Apple Silicon | apps iOS y macOS; el más caro |
| `saas-windows-medium-amd64` | Windows | .NET |

Los que aparecen como `shared-gitlab-org` en la lista son los runners internos del propio proyecto
GitLab, no son para tu uso. El tier gratuito da 400 minutos al mes y cada familia consume con un
multiplicador: el `small` es la base.

Se elige runner con `tags` en el job:

```yaml
test:
  tags:
    - saas-linux-small-amd64
```

**Por qué el `small` de Intel sirve incluso para imágenes arm64:** Go cross-compila sin emulación.
Con `FROM --platform=$BUILDPLATFORM` y `GOARCH=$TARGETARCH` en el Dockerfile, la compilación corre
nativa en el runner y emite el binario para la arquitectura destino. La etapa final sobre distroless
solo copia el binario, sin ningún `RUN`, así que tampoco necesita QEMU. Solo elegirías
`saas-linux-medium-arm64` si el build tuviera que EJECUTAR código de la arquitectura destino, por
ejemplo con dependencias CGO que no cross-compilan.

### Runner propio en Docker (referencia)

Desde GitLab 16.6 ya no hay token compartido de registro: primero se crea un objeto runner, que
devuelve un token de autenticación propio, y con ese se registra el agente. Cada runner tiene
identidad y es auditable.

```bash
RUNNER_TOKEN=$(glab api --method POST user/runners \
  --field runner_type=project_type --field project_id=<project-id> \
  --field description=macbook-local --field run_untagged=true \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
echo "token capturado: ${#RUNNER_TOKEN} caracteres"      # ~78

gitlab-runner register --non-interactive \
  --url https://gitlab.com --token "$RUNNER_TOKEN" \
  --executor docker --docker-image golang:1.24 \
  --description macbook-local

cat ~/.gitlab-runner/config.toml
gitlab-runner run                            # primer plano, bloquea la terminal
gitlab-runner unregister --all-runners       # lo retira y lo borra del config.toml
```

El token solo se muestra al crear el runner: si lo pierdes, borra el runner y crea otro.

**En macOS hay que decirle dónde está el socket de Docker.** En Linux el demonio escucha en
`/var/run/docker.sock`; Docker Desktop lo pone en `~/.docker/run/docker.sock`. El `docker` de
terminal funciona porque usa los *contexts*, pero el runner va directo a la ruta de Linux y falla con
`dial unix /var/run/docker.sock: no such file or directory`. En `[runners.docker]` del
`config.toml`:

```toml
  [runners.docker]
    host = "unix://$HOME/.docker/run/docker.sock"
    volumes = ["$HOME/.docker/run/docker.sock:/var/run/docker.sock", "/cache"]
```

`host` es dónde busca el runner el demonio para crear contenedores; `volumes` es qué socket se monta
*dentro* del job para que pueda construir imágenes. Verifica la ruta con:

```bash
docker context inspect --format '{{.Endpoints.docker.Host}}'
```

Montar el socket le da al job control total del demonio del anfitrión, lo que equivale a root en la
máquina. Aceptable en un laboratorio propio; en un runner compartido se usa Docker-in-Docker con
`--docker-privileged`, o herramientas sin demonio como Kaniko o Buildah.

Sobre los executors: `docker` da un contenedor nuevo y desechable por job; `shell` corre directo en
la máquina sin aislamiento; `kubernetes` crea un pod por job y es lo típico en empresas grandes.

### Listar y limpiar runners de proyecto

```bash
glab api "projects/<project-id>/runners?type=project_type&per_page=100" \
  | python3 -c 'import sys,json;[print("id=%s %-16s %s" % (r["id"], r["status"], r["description"])) for r in json.load(sys.stdin)]'
glab api --method DELETE runners/<id>
```

Un runner desregistrado puede seguir apareciendo `Online` un rato: GitLab mantiene ese estado
durante una ventana de gracia tras el último latido. El `config.toml` local es la fuente de verdad
de si el agente existe.

### Seguir los pipelines

```bash
glab ci status                               # interactivo, permite ver logs y reintentar
glab ci run --branch main                    # lanza un pipeline sin hacer commit
glab ci list
glab ci trace                                # logs del último job
```

Consultas no interactivas, útiles para scripts:

```bash
glab api "projects/<project-id>/pipelines?per_page=5" \
  | python3 -c 'import sys,json;[print(p["id"], p["status"], p["sha"][:8]) for p in json.load(sys.stdin)]'
glab api "projects/<project-id>/pipelines/<id>/jobs" \
  | python3 -c 'import sys,json
for j in json.load(sys.stdin):
    r = j.get("runner") or {}
    print(j["name"], j["status"], j.get("duration"), r.get("description"))'
glab api --method POST projects/<project-id>/pipelines/<id>/cancel     # matar un job zombi
```

Un job puede quedar colgado en `running` si el runner muere a mitad: GitLab no se entera hasta que
expira el timeout. Se cancela a mano.

### Comparación de tiempos

| Dónde | Duración del job de tests |
|---|---|
| GitHub Actions, runner hospedado | 2 min 20 s |
| gitlab.com, `saas-linux-small-amd64` | 2 min aprox |
| Runner propio en la Mac | 20 s |

El runner local gana porque ya tiene las imágenes en caché y no arranca una máquina virtual nueva.

## 7. Promoción de entornos (CI/CD completo)

El pipeline vive dos veces, en `.gitlab-ci.yml` y en `.github/workflows/ci.yml`, para comparar las
dos plataformas. Cuatro fases: `test` → `build` → `deploy-dev` (automático) → `deploy-prod` (manual).

### Dockerfile multi-arquitectura

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.24 AS build
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build ...
```

`BUILDPLATFORM` es la arquitectura del runner; `TARGETARCH` la del destino. La compilación corre
nativa y Go cross-compila: un runner Intel barato produce arm64 sin emulación QEMU. La etapa final
sobre distroless no lleva ningún `RUN`, así que tampoco necesita emular nada.

`USER 65532:65532` en lugar de `USER nonroot`: el UID numérico evita el `CreateContainerConfigError`
cuando el pod pide `runAsNonRoot`.

### buildx dentro de un job

```yaml
services:
  - docker:28-dind
variables:
  DOCKER_HOST: tcp://docker:2376
  DOCKER_TLS_CERTDIR: "/certs"
  DOCKER_TLS_VERIFY: 1
  DOCKER_CERT_PATH: "/certs/client"
script:
  - docker login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" "$CI_REGISTRY"
  - docker context create dind          # buildx no acepta TLS por variables de entorno
  - docker buildx create --use --driver docker-container --name multi dind
  - docker buildx build --platform linux/amd64,linux/arm64 --push .
```

`CI_REGISTRY_USER` y `CI_REGISTRY_PASSWORD` son el token del job: sirven para el registry del propio
proyecto y no hay que crear ningún secreto. `--platform` con dos valores publica un *manifest list*:
una sola referencia que sirve a un cluster arm64 y a uno Intel.

### El job que escribe el tag en git

El token del job NO puede hacer push al repo. Hace falta un token de proyecto con
`write_repository`, guardado como variable enmascarada:

```bash
# Crea el token en Settings → Access Tokens (scope write_repository, rol Maintainer), luego:
read -rs "TOK?Pega el token: " && glab api --method POST projects/<project-id>/variables \
  --raw-field key=GITLAB_PUSH_TOKEN --raw-field value="$TOK" \
  --raw-field masked=true --raw-field protected=false && unset TOK
glab api "projects/<project-id>/variables" | python3 -c 'import sys,json;[print(v["key"], "masked:", v["masked"]) for v in json.load(sys.stdin)]'
```

El rol del token importa: `main` suele estar protegida y admitir push solo de Maintainer, mientras
que un token creado como Developer es rechazado por el hook de pre-recepción. Comprueba las dos
cosas antes de depurar a ciegas:

```bash
glab api "projects/<project-id>/protected_branches" | python3 -c 'import sys,json;[print(b["name"], [a["access_level"] for a in b["push_access_levels"]]) for b in json.load(sys.stdin)]'
glab api "projects/<project-id>/members/all" | python3 -c 'import sys,json;[print(m["username"], m["access_level"]) for m in json.load(sys.stdin)]'
```

El nivel 30 es Developer y el 40 Maintainer. El bot del token aparece como
`project_<id>_bot_<hash>` en la lista de miembros.

El job clona con ese token, reescribe el tag y hace push:

```bash
git clone --depth 1 --branch "$CI_DEFAULT_BRANCH" "https://oauth2:${GITLAB_PUSH_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" repo
sed -i "s|newTag: [^ ]*|newTag: $CI_COMMIT_SHORT_SHA|" "deploy/overlays/$ENVIRONMENT/kustomization.yaml"
git commit -m "Deploy $CI_COMMIT_SHORT_SHA to $ENVIRONMENT [skip ci]"
git push origin "HEAD:$CI_DEFAULT_BRANCH"
```

**`[skip ci]` es obligatorio**: sin él, ese commit dispara otro pipeline que hace otro commit, en
bucle infinito. `GIT_STRATEGY: none` evita que el runner clone de más, porque clonamos nosotros.

### La puerta manual

En GitLab es una línea: `when: manual` dentro de `rules`. El job aparece como un botón en la UI y el
pipeline no lo corre solo. En GitHub Actions la puerta NO va en el YAML: se declara
`environment: production` en el job y se configura "Required reviewers" en Settings → Environments.

### Pull secret del registry privado

```bash
kubectl create secret docker-registry gitlab-registry -n payplan \
  --docker-server=registry.gitlab.com \
  --docker-username=<deploy-token-user> --docker-password=<deploy-token>
```

Se crea un *deploy token* con scope `read_registry` en Settings → Repository → Deploy tokens, y el
Deployment lo referencia con `imagePullSecrets`. Hay que repetirlo en cada namespace: los Secrets no
cruzan namespaces.

Para no ver el token en pantalla, se crea y se consume en la misma línea:

```bash
eval "$(glab api --method POST projects/<project-id>/deploy_tokens \
  -F name=kind-pull -F username=kind-pull -F 'scopes=["read_registry"]' \
  | python3 -c 'import sys,json;d=json.load(sys.stdin);print("DT_USER=%s DT_TOK=%s"%(d["username"],d["token"]))')"

for ns in payplan payplan-prod; do
  kubectl create namespace $ns --dry-run=client -o yaml | kubectl apply -f -
  kubectl create secret docker-registry gitlab-registry -n $ns \
    --docker-server=registry.gitlab.com \
    --docker-username="$DT_USER" --docker-password="$DT_TOK" --dry-run=client -o yaml | kubectl apply -f -
done
unset DT_USER DT_TOK
```

Las dos banderas de `glab api` no son equivalentes: `--raw-field` manda el valor como texto literal y
`-F` lo interpreta, así que solo `-F` reconoce un array JSON. El patrón
`kubectl create ... --dry-run=client -o yaml | kubectl apply -f -` hace el comando idempotente:
`kubectl create` a secas falla si el objeto ya existe.

### Equivalencias GitHub Actions y GitLab CI

| GitHub Actions | GitLab CI |
|---|---|
| `jobs.<id>.runs-on` | `image:` más el runner elegido por `tags` |
| `steps` con `uses:` | no existe; todo es `script:` con comandos |
| `actions/checkout` | implícito, el runner clona siempre |
| `services:` alcanzable en `localhost` | `services:` alcanzable por su `alias` |
| `needs:` | `needs:` o el orden de `stages` |
| `if: github.ref == 'refs/heads/main'` | `rules: - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH` |
| `secrets.FOO` | variables de CI/CD, enmascarables y protegibles |
| `${{ github.sha }}` | `$CI_COMMIT_SHA` y `$CI_COMMIT_SHORT_SHA` |
| `GITHUB_TOKEN` con `permissions:` | `CI_JOB_TOKEN`, que no puede escribir en el repo |
| `environment:` con Required reviewers | `when: manual` |
| `${GITHUB_SHA::8}` a mano | `$CI_COMMIT_SHORT_SHA` ya viene recortado |

### Dos entornos en Argo CD

Una `Application` por entorno, cada una apuntando a su overlay:

```bash
make argocd-app            # payplan-dev  → deploy/overlays/dev  → namespace payplan
make argocd-app-prod       # payplan-prod → deploy/overlays/prod → namespace payplan-prod
argocd app list
```

---

## 8. Diagnóstico

### Leer el estado de un pod

```bash
kubectl -n <ns> get pods -o wide
kubectl -n <ns> get events --sort-by=.lastTimestamp | tail -15
kubectl -n <ns> describe pod <pod>
kubectl -n <ns> logs <pod> --previous         # logs del contenedor anterior tras un crash
```

La columna `STATUS` dice la familia del problema:

| Estado | Familia |
|---|---|
| `Pending` | scheduling: recursos, nodos, PVC |
| `ImagePullBackOff` | imagen: nombre, tag, credenciales del registry |
| `CreateContainerConfigError` | configuración: Secret o ConfigMap que falta, securityContext |
| `CrashLoopBackOff` | el proceso arranca y muere |

La columna `RESTARTS` cuenta otra historia: un reinicio con marca de tiempo reciente suele ser un
CrashLoop que se curó solo cuando apareció su dependencia.

### Errores que ya encontramos y su causa

| Error | Causa | Arreglo |
|---|---|---|
| `container has runAsNonRoot and image has non-numeric user` | el Dockerfile declara `USER nonroot` por nombre y el kubelet no resuelve nombres a UID | `runAsUser: 65532` y `runAsGroup: 65532` explícitos en el pod |
| `metadata.annotations: Too long: may not be more than 262144 bytes` | `kubectl apply` del lado del cliente guarda una copia del manifiesto en una anotación, y el CRD de ApplicationSet la desborda | `kubectl apply --server-side --force-conflicts` |
| `ComparisonError ... kustomize build failed` con `Sync Status: Unknown` | se borró `pdb.yaml` pero quedó su línea en el kustomization | corregir el kustomization; Argo **no** poda cuando no puede renderizar, por diseño |
| `invalid session: token is expired` | el token de la CLI de Argo CD dura 24 h | `argocd login` de nuevo |
| `package ... is not in std` | ruta de archivo sin `./`, interpretada como ruta de import | prefijar con `./` |
| `token capturado: 0 caracteres` | parsear JSON compacto con un `sed` que esperaba espacios | usar un parser de verdad (`python3 -c`) |
| `No OAuth application ID is configured for github-personal` | `glab` confundió un alias SSH con una instancia GitLab | `glab auth login --hostname gitlab.com` |
| `dial unix /var/run/docker.sock: no such file or directory` en un job | Docker Desktop en macOS pone el socket en `~/.docker/run/`, no en la ruta de Linux; el `docker` de terminal funciona porque usa los *contexts* | `host` y `volumes` con la ruta real en `[runners.docker]` |
| el job lo tomó un `saas-linux-small-amd64` en vez del runner local | job sin `tags` y shared runners habilitados: gana el que esté en línea | poner `tags` en el job, o deshabilitar los shared |
| job colgado en `running` con cientos de segundos | el runner murió a mitad del job y GitLab no se entera hasta el timeout | `glab api --method POST projects/<id>/pipelines/<id>/cancel` |
| un runner desregistrado sigue apareciendo `Online` | GitLab mantiene el estado durante una ventana de gracia tras el último latido | borrarlo con `glab api --method DELETE runners/<id>`; el `config.toml` local es la fuente de verdad |
| `could not create a builder instance with TLS data loaded from environment` | `buildx` con driver `docker-container` no acepta el demonio por variables de entorno con TLS | `docker context create dind` y pasar ese contexto a `buildx create` |
| `did not find expected alphabetic or numeric character while scanning an alias` al lintar el YAML | un escalar plano de YAML no puede contener dos puntos seguidos de espacio; el `sed` llevaba `newTag: ` dentro | envolver la línea en comillas simples |
| `blob unknown to registry` al publicar con `buildx` | buildx genera atestaciones de procedencia por defecto y el registry de GitLab las rechaza (GHCR sí las acepta) | añadir `--provenance=false` al `buildx build` |
| `a field name containing a bracket is not supported in a JSON request body` | `glab api --raw-field 'scopes[]=x'` no vale para arrays | `-F 'scopes=["x"]'`: `-F` interpreta el valor, `--raw-field` lo manda como texto |
| `HTTP Basic: Access denied` en el job que hace push | la variable con el token de escritura no existe o no está expuesta al job | crearla como variable de CI enmascarada y no protegida |
| `You are not allowed to push code to protected branches` | el bot del token de proyecto es Developer y `main` solo admite push de Maintainer | recrear el token con rol Maintainer; no bajar la protección de la rama |
| `(GITLAB_PUSH_TOKEN) has already been taken` | la variable de CI ya existe | actualizarla con `PUT .../variables/<clave>`, no crearla con `POST` |
| `! [rejected] main -> main (non-fast-forward)` al hacer push desde tu máquina | el pipeline escribe commits en `main`, así que tu copia local se queda atrás tras cada despliegue | `git pull --rebase origin main` antes de empujar |

Lección transversal: **nunca parsees JSON con `grep` o `sed`**. El formato cambia y el script falla
en silencio. Y **filtra la salida de una API antes de mirarla**: suele traer credenciales.
