# Multi-stage build para optimizar tamaño
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copiar archivos fuente
COPY *.go ./

# Compilar binarios
RUN go build -o worker worker.go types.go
RUN go build -o distributed_system distributed_system.go database.go api.go metrics.go types.go

# Imagen final ligera
FROM alpine:latest

WORKDIR /app

# Instalar ca-certificates para HTTPS
RUN apk --no-cache add ca-certificates

# Copiar binarios compilados
COPY --from=builder /app/worker /app/worker
COPY --from=builder /app/distributed_system /app/distributed_system

# Crear directorio para datos
RUN mkdir -p /app/data_25M

# Exponer puertos
EXPOSE 8080 9001 9002 9003 9004 9005 9006 9007 9008

# Comando por defecto
CMD ["/app/distributed_system", "-mode", "distributed", "-api", ":8080"]
