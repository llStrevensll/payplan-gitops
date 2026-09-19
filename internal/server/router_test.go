package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test de tabla: la forma idiomática en Go de probar varios casos con el mismo código.
// Cada fila es un caso; t.Run crea un subtest con nombre para que el fallo sea legible.
func TestRutasDeSalud(t *testing.T) {
	h := NewRouter("test-1.0")

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantKey    string // clave del JSON a verificar ("" = no verificar cuerpo)
		wantValue  string
	}{
		{"healthz responde ok", http.MethodGet, "/healthz", http.StatusOK, "status", "ok"},
		{"readyz responde ready", http.MethodGet, "/readyz", http.StatusOK, "status", "ready"},
		{"version devuelve la versión compilada", http.MethodGet, "/version", http.StatusOK, "version", "test-1.0"},
		{"ruta inexistente da 404", http.MethodGet, "/nope", http.StatusNotFound, "", ""},
		{"método no permitido da 405", http.MethodPost, "/healthz", http.StatusMethodNotAllowed, "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// httptest.NewRecorder simula un ResponseWriter; no se abre ningún puerto.
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, quería %d", rec.Code, tc.wantStatus)
			}
			if tc.wantKey == "" {
				return
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("la respuesta no es JSON: %v", err)
			}
			if body[tc.wantKey] != tc.wantValue {
				t.Fatalf("%s = %q, quería %q", tc.wantKey, body[tc.wantKey], tc.wantValue)
			}
		})
	}
}
