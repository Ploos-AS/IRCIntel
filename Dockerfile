FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/ircintel ./cmd/ircintel

FROM scratch
COPY --from=build /out/ircintel /ircintel
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/ircintel"]
