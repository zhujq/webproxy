
FROM golang:1.13.5-alpine3.10 AS builder
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server server.go



FROM ubuntu:latest

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y curl openssh-server zip unzip net-tools inetutils-ping iproute2 tcpdump git vim mysql-client redis-tools\
  && mkdir -p /var/run/sshd \
  && sed -ri 's/^#?PermitRootLogin\s+.*/PermitRootLogin yes/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?ClientAliveInterval\s+.*/ClientAliveInterval 60/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?ClientAliveCountMax\s+.*/ClientAliveCountMax 1000/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?TCPKeepAlive\s+.*/TCPKeepAlive yes/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?PasswordAuthentication\s+.*/PasswordAuthentication no/' /etc/ssh/sshd_config \
  && sed -ri 's/^#PubkeyAuthentication\s+.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config \
  && sed -ri 's/UsePAM yes/#UsePAM yes/g' /etc/ssh/sshd_config && mkdir /root/.ssh \
  && rm -rf /var/lib/apt/lists/*

ADD . /
WORKDIR /
COPY --from=builder /server .
CMD ["/bin/bash", "run.sh"]

EXPOSE 8080
