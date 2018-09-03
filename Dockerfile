FROM golang:1.10 as gobuild
WORKDIR /go/src/stash.hcom/run/kube-graffiti
ENV CGO_ENABLED=0 GOOS=linux
USER $UID
COPY . .
RUN go build -a -v

FROM alpine:3.7
LABEL maintainer="Hcom Tech Runtime Platform <htechruntimeplatform@expedia.com>"
RUN apk add --no-cache ca-certificates apache2-utils git openssh-client

COPY --from=gobuild /go/src/stash.hcom/run/kube-graffiti/kube-graffiti /
ENTRYPOINT ["/kube-graffiti"]
