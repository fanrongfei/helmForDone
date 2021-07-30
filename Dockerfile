FROM golang:1.14 as builder

WORKDIR /tmp/build
COPY . .
RUN GOOS=linux go build -mod=vendor -ldflags="-s -w"

# ---

FROM alpine as loader
COPY kubectl.zip /usr/local/bin/
COPY helm.zip /usr/local/bin/

WORKDIR /tmp
RUN unzip  /usr/local/bin/kubectl.zip -d /usr/local/bin/ && unzip  /usr/local/bin/helm.zip -d /usr/local/bin/ && chmod +x /usr/local/bin/helm && chmod +x /usr/local/bin/kubectl
# ---

FROM busybox:glibc

COPY --from=loader /usr/local/bin/helm /usr/local/bin/helm
COPY --from=loader /usr/local/bin/kubectl /usr/local/bin/kubectl

COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /tmp/build/drone-helm3 /usr/local/bin/drone-helm3

RUN mkdir /root/.kube

CMD /usr/local/bin/drone-helm3
