//go:build ignore

// Ejemplo de la sesión 0 bis: qué atrapa `go vet` que el compilador deja pasar.
//
// La primera línea, `//go:build ignore`, es una "restricción de compilación": excluye este
// archivo de `go build ./...`, `go vet ./...` y `go test ./...`. Así `make test` no se rompe
// por un ejemplo que ES incorrecto a propósito. Para analizarlo hay que nombrarlo explícito:
//
//	go vet ejemplos/sesion0bis/vet/main.go
//	go run ejemplos/sesion0bis/vet/main.go
package main

import "fmt"

// totalCentavos calcula el total de un plan. Fíjate en la línea después del return.
func totalCentavos(cuotas int) int {
	return cuotas * 1250
	fmt.Println("esto nunca se ejecuta") // vet: código inalcanzable
	// Este segundo return existe solo para que COMPILE: Go exige que una función con valor de
	// retorno termine en una "sentencia terminal" (return, panic, for infinito...). Sin él, el
	// compilador dice "missing return" y ni siquiera llegamos a vet.
	return 0
}

func main() {
	nombre := "Ada" // := declara e infiere el tipo (string). Como `nombre = "Ada"` en Python.
	cuotas := 4

	// Esto COMPILA porque Printf acepta argumentos de cualquier tipo (any).
	// Pero %d espera un entero y recibe un string, y %s espera string y recibe un int.
	// En Python `"%d" % "Ada"` explota con TypeError. En Go no explota: imprime basura.
	fmt.Printf("plan de %d cuotas para %s\n", nombre, cuotas)

	fmt.Println("total:", totalCentavos(cuotas))
}
