FROM golang:alpine as builder
COPY . /usr/src/egw-web-service
WORKDIR /usr/src/egw-web-service
RUN go build -o web-service main.go

FROM alpine
EXPOSE 8080
COPY --from=builder /usr/src/egw-web-service/web-service /usr/local/bin
CMD ["/usr/local/bin/web-service"]
