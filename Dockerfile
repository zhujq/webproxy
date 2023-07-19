FROM golang:1.18-alpine3.16 as builder

WORKDIR $GOPATH/src/wserver
COPY . .

RUN apk add --no-cache git && set -x && \
    go mod init && go get -d -v
RUN CGO_ENABLED=0 GOOS=linux go build -o /server server.go



FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y curl openssh-server zip unzip net-tools inetutils-ping iproute2 tcpdump vim  tzdata\
  && mkdir -p /var/run/sshd \
  && sed -ri 's/^#?PermitRootLogin\s+.*/PermitRootLogin yes/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?ClientAliveInterval\s+.*/ClientAliveInterval 30/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?ClientAliveCountMax\s+.*/ClientAliveCountMax 1000/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?TCPKeepAlive\s+.*/TCPKeepAlive yes/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?PasswordAuthentication\s+.*/PasswordAuthentication no/' /etc/ssh/sshd_config \
  && sed -ri 's/^#PubkeyAuthentication\s+.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config \
  && sed -ri 's/UsePAM yes/#UsePAM yes/g' /etc/ssh/sshd_config && mkdir /root/.ssh \
  && echo "Asia/Shanghai" > /etc/timezone &&  rm -f /etc/localtime   && dpkg-reconfigure -f noninteractive tzdata \
  && rm -rf /var/lib/apt/lists/*

ADD . /
WORKDIR /
COPY --from=builder /server .
CMD ["/bin/bash", "run.sh"]

EXPOSE 80
