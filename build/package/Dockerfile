FROM golang:alpine as builder

ENV GOOS=linux
ARG GITLAB_AUTHN

# install and configure git. we need it because some of our modules
# (e.g., egw-resource-model) are private
RUN apk add git
RUN git config --global url."https://${GITLAB_AUTHN}@gitlab.com/acnodal".insteadOf https://gitlab.com/acnodal

WORKDIR /opt/acnodal/src
COPY . ./

# build the web service (static)
RUN go build  -tags 'osusergo netgo' -o ../bin/egw-ws main.go


# start fresh
FROM golang:alpine
ENV bin=/opt/acnodal/bin/egw-ws

# copy executables from the builder image
COPY --from=builder ${bin} ${bin}

EXPOSE 8080

# The softlink is because Dockerfile variable interpolation happens at
# run-time so if you have variables in the CMD string they won't get
# resolved to their values.  This lets us have a hard-coded CMD string
# that links to the image-specific command.
RUN ln -s ${bin} /opt/acnodal/bin/cmd
CMD ["/opt/acnodal/bin/cmd"]
