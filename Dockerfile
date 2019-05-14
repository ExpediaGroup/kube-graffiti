FROM golang:1.12 as gobuild
WORKDIR /go/src/github.com/HotelsDotCom/kube-graffiti
ENV CGO_ENABLED=0 GOOS=linux
USER $UID
COPY . .
RUN go build -a -v

FROM alpine:3.7
RUN apk add --no-cache ca-certificates apache2-utils git openssh-client

COPY --from=gobuild /go/src/github.com/HotelsDotCom/kube-graffiti/kube-graffiti /
ENTRYPOINT ["/kube-graffiti"]
