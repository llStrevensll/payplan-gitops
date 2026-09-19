// Punto de entrada del servicio payplan.
// Aquí SOLO se cablean las piezas (configuración, router, servidor HTTP)
// y se maneja el apagado ordenado. La lógica vive en internal/.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/llStrevensll/payplan/internal/server"
)

// version se sobreescribe al compilar con: -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	// Logs en JSON: Kubernetes, Loki o Datadog los parsean sin esfuerzo.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Configuración por variables de entorno (12-factor). Sin archivos de config.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.NewRouter(version),
		ReadHeaderTimeout: 5 * time.Second,  // evita ataques slowloris (cabeceras que nunca llegan)
		ReadTimeout:       10 * time.Second, // tiempo máximo para leer la petición completa
		WriteTimeout:      15 * time.Second, // tiempo máximo para escribir la respuesta
		IdleTimeout:       60 * time.Second, // cuánto vive una conexión keep-alive sin uso
	}

	// ctx se cancela cuando llega SIGTERM (lo envía Kubernetes al apagar el pod) o SIGINT (Ctrl+C).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// El servidor corre en su propia goroutine para que main pueda esperar la señal.
	go func() {
		slog.Info("payplan escuchando", "addr", srv.Addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("el servidor falló", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done() // bloquea hasta recibir la señal
	slog.Info("señal recibida, apagando ordenadamente")

	// Shutdown deja de aceptar conexiones nuevas y espera a que terminen las peticiones en vuelo,
	// con un límite de 20 s. terminationGracePeriodSeconds en Kubernetes debe ser mayor (30 s).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("apagado forzado", "err", err)
	}
	slog.Info("adiós")
}
