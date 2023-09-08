FROM golang:1.17-alpine as builder

ENV GOOS=linux

WORKDIR /opt/acnodal/src
COPY . ./

# build the web service (static)
RUN go build  -tags 'osusergo netgo' -o ../bin/web-service main.go


# start fresh
FROM golang:1.17-alpine
ENV bin=/opt/acnodal/bin/web-service

# copy executables from the builder image
COPY --from=builder ${bin} ${bin}

EXPOSE 8080

# The softlink is because Dockerfile variable interpolation happens at
# run-time so if you have variables in the CMD string they won't get
# resolved to their values.  This lets us have a hard-coded CMD string
# that links to the image-specific command.
RUN ln -s ${bin} /opt/acnodal/bin/cmd
CMD ["/opt/acnodal/bin/cmd"]
