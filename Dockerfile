FROM golang:1.17.13-alpine as builder

ENV GOOS=linux
WORKDIR /opt/epic-gateway/src
COPY . ./

# build the executable (static)
RUN go build  -tags 'osusergo netgo' -o ../bin/web-service main.go


# start fresh
FROM alpine:3.16.7

# copy executable from the builder image
ENV bin=/opt/epic-gateway/bin/web-service
COPY --from=builder ${bin} ${bin}

EXPOSE 8080

CMD ["/opt/epic-gateway/bin/web-service"]
