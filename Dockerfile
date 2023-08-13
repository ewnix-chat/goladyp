FROM golang:1.16 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o goladyp goladyp.go

EXPOSE 8080

CMD ["goladyp"]

