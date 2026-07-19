FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
COPY *.go ./
RUN go build -o gateway-core .

FROM alpine:3.20
COPY --from=build /app/gateway-core /usr/local/bin/gateway-core
EXPOSE 8080
ENTRYPOINT ["gateway-core"]
