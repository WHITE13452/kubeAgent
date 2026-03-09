FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY KubeAgent/ .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o kubeagent .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates curl bash \
    && curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/

COPY --from=builder /app/kubeagent /usr/local/bin/

ENTRYPOINT ["kubeagent"]
