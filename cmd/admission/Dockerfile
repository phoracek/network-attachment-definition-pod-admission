FROM golang:1.10 AS builder
WORKDIR /go/src/github.com/phoracek/network-attachment-definition-pod-admission/
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . .
RUN dep ensure --vendor-only
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/admission github.com/phoracek/network-attachment-definition-pod-admission/cmd/admission

FROM fedora
COPY --from=builder /bin/admission /bin/admission
ENTRYPOINT ["/bin/admission"]