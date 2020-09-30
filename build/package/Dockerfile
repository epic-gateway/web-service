FROM golang:alpine as builder

ENV GOOS=linux
ARG GITLAB_TOKEN

# install and configure git. we need it because some of our modules
# (e.g., egw-resource-model) are private
RUN apk add git
RUN git config --global url."https://oauth2:${GITLAB_TOKEN}@gitlab.com/acnodal".insteadOf https://gitlab.com/acnodal

WORKDIR /usr/src/egw-ws
COPY . ./

# build the web service (static)
RUN go build  -tags 'osusergo netgo' -o /usr/local/bin/ ./cmd/egw-ws

# download golang-migrate
RUN apk add curl
RUN curl -sL https://github.com/golang-migrate/migrate/releases/download/v4.11.0/migrate.linux-amd64.tar.gz | tar -C /usr/local/bin -xzf -


# start fresh
FROM ubuntu:20.04

# install dependencies
RUN apt-get update \
    && apt-get install -y postgresql-client libelf-dev \
    && rm -rf /var/lib/apt/lists/*

# copy executables from the builder image
WORKDIR /usr/local/bin
COPY --from=builder /usr/local/bin/migrate.linux-amd64 ./migrate
COPY --from=builder /usr/local/bin/egw-ws ./

# copy db migration scripts
WORKDIR /usr/local/share/egw-ws
COPY ./db ./db/

# copy PFC components from the PFC docker image
COPY --from=registry.gitlab.com/acnodal/packet-forwarding-component/pfc:demo202008031754 /tmp/.acnodal/bin/* /usr/local/bin/

EXPOSE 8080
CMD ["/usr/local/bin/egw-ws", "--debug"]
