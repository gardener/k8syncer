#### BUILDER ####
FROM golang:1.20 as builder

WORKDIR /go/src/github.com/gardener/k8syncer
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION


#### K8SYNCER CONTROLLER ####
FROM gcr.io/distroless/static:nonroot as k8syncer

COPY --from=builder /go/bin/k8syncer /k8syncer

WORKDIR /
USER nonroot:nonroot

ENTRYPOINT ["/k8syncer"]
