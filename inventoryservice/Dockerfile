FROM golang:1.26-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o service .

FROM alpine:3.24

WORKDIR /app

COPY --from=build /app/service .

CMD ["./service"]