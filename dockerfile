# Build do server
FROM golang:1.25 AS builder
WORKDIR /app

# Copia o go.mod primeiro
COPY go.mod ./

# Baixa dependências (no seu caso só stdlib)
RUN go mod tidy

# Copia todo o restante do projeto
COPY . .

# Build do servidor
RUN go build -o server server.go

# Etapa de execução
FROM debian:bookworm-slim
WORKDIR /app

# Instala certificados de CA para HTTPS funcionar
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copia o binário e a pasta static
COPY --from=builder /app/server .
COPY --from=builder /app/static ./static

# Expõe a porta usada pelo servidor
EXPOSE 8080

# Comando para rodar o servidor
CMD ["./server"]
