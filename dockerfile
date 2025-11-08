# Etapa de build
FROM golang:1.25 AS builder
WORKDIR /app

# Copia o go.mod primeiro (não precisa de go.sum)
COPY go.mod ./

# Baixa dependências da standard library (não há externas, então é rápido)
RUN go mod tidy

# Copia todo o restante do projeto
COPY . .

# Build do servidor
RUN go build -o server server.go

# Etapa de execução
FROM debian:bookworm-slim
WORKDIR /app

# Copia o binário e a pasta static
COPY --from=builder /app/server .
COPY --from=builder /app/static ./static

# Expõe a porta usada pelo servidor
EXPOSE 8080

# Comando para rodar o servidor
CMD ["./server"]
