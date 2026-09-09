FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ircintel ./cmd/ircintel \
    && mkdir -p /out/data \
    && chown 65532:65532 /out/data

FROM scratch
COPY --from=build /out/ircintel /ircintel
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/ircintel"]
