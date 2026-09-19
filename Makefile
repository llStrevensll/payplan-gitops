# Atajos del laboratorio. `make help` los lista.
APP      := payplan
IMAGE    := ghcr.io/llstrevensll/$(APP)
TAG      ?= dev
CLUSTER  := payplan
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help run test build docker-build compose-up compose-down kind-up kind-down kind-load deploy-local argocd-install argocd-password argocd-ui argocd-app argocd-app-prod

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

run: ## Corre la API en local (puerto 8080)
	go run ./cmd/api

test: ## go vet + tests con detector de carreras y cobertura
	go vet ./...
	go test -race -cover ./...

build: ## Compila el binario en bin/api con la versión de git
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/api ./cmd/api

docker-build: ## Construye la imagen $(IMAGE):$(TAG)
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(TAG) .

compose-up: ## Levanta Postgres 16 + API con docker compose
	docker compose up --build -d

compose-down: ## Apaga y borra el entorno de compose (incluye el volumen)
	docker compose down -v

kind-up: ## Crea el cluster local kind "$(CLUSTER)" (puerto 18080 → API)
	kind create cluster --name $(CLUSTER) --config deploy/kind/cluster.yaml

kind-down: ## Borra el cluster kind
	kind delete cluster --name $(CLUSTER)

kind-load: docker-build ## Carga la imagen local dentro de kind (sin registry)
	kind load docker-image $(IMAGE):$(TAG) --name $(CLUSTER)

deploy-local: kind-load ## Sesión 6: aplica el overlay dev con kubectl (antes de Argo CD)
	kubectl apply -k deploy/overlays/dev
	kubectl -n payplan rollout status deploy/payplan-api --timeout=120s

argocd-install: ## Sesión 7: instala Argo CD en el namespace argocd
	kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
	kubectl -n argocd rollout status deploy/argocd-server --timeout=300s

argocd-password: ## Contraseña inicial del usuario admin de Argo CD
	@kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d; echo

argocd-ui: ## Port-forward a la UI: https://localhost:8443 (usuario admin)
	kubectl -n argocd port-forward svc/argocd-server 8443:443

argocd-app: ## Registra la Application de dev en Argo CD
	kubectl apply -f argocd/application.yaml

argocd-app-prod: ## Registra la Application de prod (namespace payplan-prod, 3 réplicas)
	kubectl apply -f argocd/application-prod.yaml
