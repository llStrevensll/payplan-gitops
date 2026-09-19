// Ejemplo de la sesión 0 bis: qué es una "carrera de datos" (data race) y cómo la atrapa -race.
//
// Sin detector:  go run ./ejemplos/sesion0bis/carrera
// Con detector:  go run -race ./ejemplos/sesion0bis/carrera
package main

import (
	"fmt"
	"sync"
)

func main() {
	contador := 0 // memoria compartida: dos goroutines la leen y escriben a la vez

	// sync.WaitGroup es un contador de tareas pendientes: Add suma, Done resta, Wait bloquea hasta 0.
	// En Python sería lanzar dos threading.Thread y hacer .join() a cada uno.
	var wg sync.WaitGroup

	// Go solo tiene una palabra para bucles: `for`. Esta es la forma estilo C.
	for i := 0; i < 2; i++ {
		wg.Add(1)

		// `go` lanza la función en una goroutine: un hilo ligero que corre EN PARALELO con main.
		// `func() { ... }()` es una función anónima invocada en el acto, como una lambda en Python.
		go func() {
			// defer: "ejecuta esto al salir de la función, pase lo que pase". Aquí garantiza el Done.
			defer wg.Done()
			for j := 0; j < 100_000; j++ { // el guion bajo es solo legibilidad: 100_000 == 100000
				contador++ // NO es atómico: leer, sumar 1, escribir. Las dos goroutines se pisan aquí.
			}
		}()
	}

	wg.Wait()
	fmt.Println("contador =", contador, "(esperaba 200000)")
}
