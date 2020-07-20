FROM golang:alpine as builder

# build the web service
WORKDIR /usr/src/egw-web-service
COPY . ./
RUN go build -o /usr/local/bin/web-service main.go

# download golang-migrate
RUN apk add curl
RUN curl -sL https://github.com/golang-migrate/migrate/releases/download/v4.11.0/migrate.linux-amd64.tar.gz | tar -C /usr/local/bin -xzf -


# start fresh
FROM alpine

# install executables
RUN apk add postgresql-client
WORKDIR /usr/local/bin
COPY --from=builder /usr/local/bin/migrate.linux-amd64 ./migrate
COPY --from=builder /usr/local/bin/web-service ./

# copy db migration scripts
WORKDIR /usr/local/share/egw/web-service
COPY ./db ./db/

EXPOSE 8080
CMD ["/usr/local/bin/web-service"]
