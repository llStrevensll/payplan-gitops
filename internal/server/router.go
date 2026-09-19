// Paquete server: rutas HTTP y middlewares.
// Regla del proyecto: los handlers no tocan la base de datos directamente;
// hablan con servicios de dominio (internal/plans) que a su vez usan un store.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// NewRouter arma el enrutador con todas las rutas y middlewares.
// Devuelve http.Handler (una interfaz) para poder probarlo con httptest sin levantar un puerto.
func NewRouter(version string) http.Handler {
	mux := http.NewServeMux()

	// Go 1.22+: el patrón incluye el método HTTP y admite variables como {id}.
	// No necesitamos un router externo para una API pequeña.
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
	mux.HandleFunc("GET /version", handleVersion(version))

	// Sesión 1: aquí irán las rutas de negocio.
	// mux.HandleFunc("POST /plans", ...)
	// mux.HandleFunc("GET /plans/{id}", ...)

	return withLogging(mux)
}

// handleHealthz es la liveness probe: responde 200 si el proceso está vivo.
// NUNCA consulta la base de datos: si la base tiene lag, no queremos que Kubernetes reinicie la API.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz es la readiness probe: "puedo atender tráfico".
// En la sesión 2 verificará la conexión a Postgres con un ping corto.
func handleReadyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleVersion devuelve la versión compilada.
// Es una closure: la función interna "captura" la variable version.
func handleVersion(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version})
	}
}

// writeJSON centraliza cabecera + status + codificación para no repetirlo en cada handler.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("no se pudo escribir la respuesta", "err", err)
	}
}

// statusRecorder envuelve el ResponseWriter para capturar el status code y poder loguearlo.
// Embebemos la interfaz: heredamos todos sus métodos y sobreescribimos solo WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withLogging es un middleware: recibe un handler y devuelve otro que lo envuelve.
// Este patrón (handler que envuelve handler) es la base de auth, métricas, tracing, etc.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}
