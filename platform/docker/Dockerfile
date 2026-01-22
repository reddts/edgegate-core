FROM alpine:latest
ENV CONFIG='https://raw.githubusercontent.com/ircfspace/warpsub/main/export/warp#WARP%20(IRCF)'
ENV VERSION=v3.1.8
WORKDIR /degegate
RUN apk add  curl tar gzip libc6-compat # iptables ip6tables

RUN echo "architecture: $(apk --print-arch)" && \    
    case "$(apk --print-arch)" in \
            x86_64) ARCH=amd64 ;; \
            i386|x86) ARCH=386 ;; \
            aarch64) ARCH=arm64 ;; \
            armv7) ARCH=armv7 ;; \
            armv6|armhf) ARCH=armv6 ;; \
            armv5) ARCH=armv5 ;; \
            s390x) ARCH=s390x ;; \
            *) echo "Unsupported architecture: $(apk --print-arch) $(uname -m)" && exit 1 ;; \
        esac && \
        echo "Downloading https://github.com/reddts/edgegate-core/releases/download/${VERSION}/edgegate-cli-linux-$ARCH.tar.gz" && \
        curl -L -o edgegate-cli.tar.gz https://github.com/reddts/edgegate-core/releases/download/${VERSION}/edgegate-cli-linux-$ARCH.tar.gz && \
        tar -xzf edgegate-cli.tar.gz && rm edgegate-cli.tar.gz
COPY edgegate.sh .
RUN chmod +x edgegate.sh

EXPOSE 12334
EXPOSE 12335
EXPOSE 16756
EXPOSE 16450


ENTRYPOINT [ "/degegate/edgegate.sh" ]
