FROM golang:1.14 
ENV SOURCE=/go/src/github.com/ONSdigital/ras-rm-party
COPY . $SOURCE
WORKDIR $SOURCE
RUN go build -o ras-rm-party
CMD ./ras-rm-party