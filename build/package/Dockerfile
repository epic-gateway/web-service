FROM golang:alpine as builder

# build the web service
WORKDIR /usr/src/egw-ws
COPY . ./
RUN go build -o /usr/local/bin/ ./cmd/egw-ws

# download golang-migrate
RUN apk add curl
RUN curl -sL https://github.com/golang-migrate/migrate/releases/download/v4.11.0/migrate.linux-amd64.tar.gz | tar -C /usr/local/bin -xzf -


# start fresh
FROM alpine

# install executables
RUN apk add postgresql-client busybox-extras
WORKDIR /usr/local/bin
COPY --from=builder /usr/local/bin/migrate.linux-amd64 ./migrate
COPY --from=builder /usr/local/bin/egw-ws ./

# copy db migration scripts
WORKDIR /usr/local/share/egw-ws
COPY ./db ./db/

EXPOSE 8080
CMD ["/usr/local/bin/egw-ws", "--debug"]
