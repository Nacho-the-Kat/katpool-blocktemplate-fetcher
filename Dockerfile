FROM ubuntu:latest

# syntax=docker/dockerfile:1

FROM golang:1.18

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY *.go ./

# Build
RUN go build -o /block-template-fetcher

# Run
CMD ["/block-template-fetcher"]