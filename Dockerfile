FROM ubuntu:18.04

RUN apt-get update\
     && apt-get install curl -y --no-install-recommends\
     && apt-get clean \
     && rm -rf /var/lib/apt/lists/*
EXPOSE 8059

COPY build/linux-amd64/bin/main /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/main" ]