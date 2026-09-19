#!/usr/bin/env bash
# Prepara una Mac para el laboratorio. Es idempotente: puedes correrlo varias veces.
set -euo pipefail

need() { command -v "$1" >/dev/null 2>&1; }

if ! need brew; then
  echo "Falta Homebrew. Instálalo desde https://brew.sh y vuelve a correr este script."
  exit 1
fi

# Herramientas por brew. Si ya están instaladas, se salta.
for f in go kind kubectl argocd k6 golang-migrate; do
  if brew list --formula "$f" >/dev/null 2>&1; then
    echo "✓ $f ya instalado"
  else
    echo "→ instalando $f"; brew install "$f"
  fi
done

if ! need docker; then
  echo
  echo "Falta Docker. Instala Docker Desktop u OrbStack y ábrelo antes de continuar."
  echo "  https://www.docker.com/products/docker-desktop/   |   https://orbstack.dev"
fi

echo
echo "Versiones:"
go version
kind version
kubectl version --client 2>/dev/null | head -1
argocd version --client --short 2>/dev/null || true
k6 version 2>/dev/null | head -1
