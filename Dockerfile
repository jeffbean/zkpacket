FROM golang:1.8

RUN apt-get update && apt-get install -y --no-install-recommends libpcap-dev && rm -rf /var/lib/apt/lists/*
RUN curl https://glide.sh/get | sh

COPY . /go/src/github.com/jeffbean/zkpacket
WORKDIR /go/src/github.com/jeffbean/zkpacket

# RUN glide install
RUN go build && cd zkload && go build 
CMD ["./zkpacket"]