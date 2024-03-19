#### K8SYNCER CONTROLLER ####
FROM gcr.io/distroless/static:nonroot as k8syncer

ARG TARGETOS
ARG TARGETARCH
WORKDIR /
COPY bin/k8syncer-$TARGETOS.$TARGETARCH /k8syncer
USER nonroot:nonroot

ENTRYPOINT ["/k8syncer"]
