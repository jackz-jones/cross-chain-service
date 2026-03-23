FROM golang:1.22.7 as builder
COPY . /cross-chain-service
RUN cd /cross-chain-service && make build

# the second stage
FROM ubuntu:24.04
RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN sed -i "s@http://.*archive.ubuntu.com@http://mirrors.huaweicloud.com@g" /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i "s@http://.*security.ubuntu.com@http://mirrors.huaweicloud.com@g" /etc/apt/sources.list.d/ubuntu.sources&& \
    apt-get update && \
    apt-get install -y vim net-tools inetutils-ping telnet tree gcc g++ p7zip-full ca-certificates curl

ENV TZ "Asia/Shanghai"
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y tzdata && \
    echo $TZ > /etc/timezone && \
    ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && \
    dpkg-reconfigure tzdata -f noninteractive

COPY --from=builder /cross-chain-service/cross-chain-service /cross-chain-service/bin/cross-chain-service
COPY --from=builder /cross-chain-service/etc/crosschain.yaml /cross-chain-service/etc/crosschain.yaml
RUN mkdir -p /cross-chain-service/logs/
WORKDIR /cross-chain-service/
CMD ["./bin/cross-chain-service"]
