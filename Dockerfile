# Estágio de Compilação
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Copia os arquivos de dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código fonte
COPY . .

# Compila o binário (ajuste o caminho se o ponto de entrada principal for em cmd/...)
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/main.go

# Estágio de Execução
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copia o binário compilado do estágio anterior
COPY --from=builder /app/server .

# Expõe a porta que o Cloud Run vai injetar (padrão 8080)
EXPOSE 8080

# Executa o programa
CMD ["./server"]

