FROM ubuntu:18.04

RUN apt-get update\
     && apt-get install curl -y --no-install-recommends\
     && apt-get install build-essential -y\
     && apt-get clean \
     && rm -rf /var/lib/apt/lists/*

RUN make build

COPY build/linux-amd64/bin/main /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/main" ]