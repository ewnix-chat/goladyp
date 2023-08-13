# Use the official Go image as the base image
FROM golang:1.16 AS build

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download the dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o goladyp goladyp.go

# Use a smaller image as the base for the final deployment
FROM alpine:3.14

# Copy the binary from the build stage to the final image
COPY --from=build /app/goladyp /usr/local/bin/

# Set environment variables if needed
ENV SMTP_SERVER=
ENV SMTP_USERNAME=
ENV SMTP_PASSWORD=
ENV FROM_EMAIL=
ENV TO_EMAIL=
ENV LDAP_SERVER=
ENV LDAP_BIND_DN=
ENV LDAP_BIND_PASSWORD=
ENV LDAP_BASE_DN=

# Expose the port that your Go application will listen on
EXPOSE 8080

# Command to run the Go application
CMD ["goladyp"]

