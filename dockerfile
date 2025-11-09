# ETAPA 1: BUILD (Compilação)
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copia os arquivos de módulo (go.mod e go.sum são essenciais para o download do /pq)
COPY go.mod .
COPY go.sum .

# Baixa as dependências do Go e o driver 'pq'
RUN go mod download

# Copia o código fonte
COPY server.go .

# Compila a aplicação Go, criando um executável estático chamado 'server'
RUN CGO_ENABLED=0 go build -o /server server.go

# ETAPA 2: PRODUCTION (Ambiente de Execução)
FROM alpine:latest

# Instala certificados CA para que o Go possa fazer requisições HTTPS (ViaCEP)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copia o executável 'server' e os arquivos estáticos
COPY --from=builder /server .
COPY static/ ./static

# Expor a porta 8080 (padrão no server.go)
EXPOSE 8080

# Comando para iniciar o servidor
CMD ["./server"]