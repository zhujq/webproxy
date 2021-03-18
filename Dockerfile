
FROM golang:1.10.1-alpine3.7 as builder
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server server.go



FROM ubuntu:latest

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
  && apt-get install -y curl openssh-server zip unzip net-tools inetutils-ping iproute2 tcpdump vim mysql-client redis-tools\
  && mkdir -p /var/run/sshd \
  && echo 'root:root@1234' |chpasswd && sed -ri 's/^#?PermitRootLogin\s+.*/PermitRootLogin yes/' /etc/ssh/sshd_config \
  && sed -ri 's/^#?ClientAliveInterval\s+.*/ClientAliveInterval 10/' /etc/ssh/sshd_config \
  && sed -ri 's/UsePAM yes/#UsePAM yes/g' /etc/ssh/sshd_config && mkdir /root/.ssh \
  && rm -rf /var/lib/apt/lists/*

ADD . /
WORKDIR /
COPY --from=builder /server .
CMD ["/bin/bash", "run.sh"]

EXPOSE 8080
