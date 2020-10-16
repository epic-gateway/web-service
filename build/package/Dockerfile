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
RUN go build  -tags 'osusergo netgo' -o /usr/local/bin/egw-ws main.go


# start fresh
FROM golang:alpine

# copy executables from the builder image
WORKDIR /usr/local/bin
COPY --from=builder /usr/local/bin/egw-ws ./

EXPOSE 8080
CMD ["/usr/local/bin/egw-ws"]
