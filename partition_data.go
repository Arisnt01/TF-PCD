package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Script para dividir ratings.csv en 8 particiones
func main() {
	inputFile := "data_25M/ratings.csv"
	numPartitions := 8

	fmt.Printf("Particionando %s en %d partes...\n", inputFile, numPartitions)

	// Abrir archivo de entrada
	file, err := os.Open(inputFile)
	if err != nil {
		log.Fatalf("Error abriendo archivo: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Leer header
	if !scanner.Scan() {
		log.Fatal("Archivo vacío o sin header")
	}
	header := scanner.Text()

	// Contar líneas totales (para el progreso)
	fmt.Println("Contando líneas totales...")
	totalLines := 0
	for scanner.Scan() {
		totalLines++
	}
	fmt.Printf("Total de líneas: %d\n", totalLines)

	// Calcular líneas por partición
	linesPerPartition := totalLines / numPartitions
	fmt.Printf("Líneas por partición: ~%d\n", linesPerPartition)

	// Re-abrir archivo para lectura
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Scan() // Skip header again

	// Crear archivos de salida
	writers := make([]*bufio.Writer, numPartitions)
	files := make([]*os.File, numPartitions)

	for i := 0; i < numPartitions; i++ {
		filename := filepath.Join("data_25M", fmt.Sprintf("ratings_part%d.csv", i+1))
		f, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Error creando partición %d: %v", i+1, err)
		}
		defer f.Close()

		files[i] = f
		writers[i] = bufio.NewWriter(f)

		// Escribir header en cada partición
		writers[i].WriteString(header + "\n")
	}

	// Distribuir líneas
	currentPartition := 0
	lineCount := 0
	linesInCurrentPartition := 0

	fmt.Println("Distribuyendo datos...")

	for scanner.Scan() {
		line := scanner.Text()

		writers[currentPartition].WriteString(line + "\n")
		linesInCurrentPartition++
		lineCount++

		// Cambiar a siguiente partición cuando se alcance el límite
		if linesInCurrentPartition >= linesPerPartition && currentPartition < numPartitions-1 {
			writers[currentPartition].Flush()
			fmt.Printf("Partición %d completada: %d líneas\n", currentPartition+1, linesInCurrentPartition)
			currentPartition++
			linesInCurrentPartition = 0
		}

		// Indicador de progreso
		if lineCount%1000000 == 0 {
			fmt.Printf("Procesadas %d líneas (%.1f%%)\n", lineCount, float64(lineCount)*100/float64(totalLines))
		}
	}

	// Flush última partición
	writers[currentPartition].Flush()
	fmt.Printf("Partición %d completada: %d líneas\n", currentPartition+1, linesInCurrentPartition)

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error leyendo archivo: %v", err)
	}

	fmt.Println("\n✓ Particionamiento completado exitosamente!")
	fmt.Printf("Total líneas procesadas: %d\n", lineCount)

	// Mostrar resumen
	fmt.Println("\nArchivos creados:")
	for i := 0; i < numPartitions; i++ {
		filename := filepath.Join("data_25M", fmt.Sprintf("ratings_part%d.csv", i+1))
		info, _ := os.Stat(filename)
		fmt.Printf("  - %s (%.2f MB)\n", filename, float64(info.Size())/1024/1024)
	}
}
