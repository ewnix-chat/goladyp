FROM golang:1.16 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o goladyp goladyp.go

FROM alpine:3.14

COPY --from=build /app/goladyp /usr/local/bin/

EXPOSE 8080

CMD ["/usr/local/bin/goladyp"]

