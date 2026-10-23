FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=api

RUN CGO_ENABLED=0 go build -o /svc ./cmd/${SERVICE}

FROM gcr.io/distroless/static:nonroot
COPY --from=build /svc /svc
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/svc"]
