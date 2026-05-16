FROM golang:1-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /scorebot .

FROM alpine:3
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /scorebot /scorebot
EXPOSE 8080
ENTRYPOINT ["/scorebot"]
