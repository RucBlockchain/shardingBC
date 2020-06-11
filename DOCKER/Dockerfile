FROM alpine:3.11
#COPY ./my_app /usr/local/bin/my_app
#LABEL maintainer="hello@tendermint.com"

# Tendermint will be looking for the genesis file in /tendermint/config/genesis.json
# (unless you change `genesis_file` in config.toml). You can put your config.toml and
# private validator file into /tendermint/config.
#
# The /tendermint/data dir is used by tendermint to store state.


# OS environment setup
# Set user right away for determinism, create directory for persistence and give our user ownership
# jq and curl used for extracting `pub_key` from private validator while
# deploying tendermint with Kubernetes. It is nice to have bash so the users
# could execute bash commands.


ENV TERM=linux
ENV LANG=en_US.UTF-8
ENV TMHOME /tendermint
#改成清华源
#RUN echo "https://mirror.tuna.tsinghua.edu.cn/alpine/v3.4/main/" > /etc/apk/repositories


RUN apk update && \
    apk upgrade && \
    apk add libc6-compat libstdc++ libgmpxx go git patch gcc=7.1.0-r0 gmp g++=7.1.0-r0&&\
    apk --no-cache add curl jq bash && \
    addgroup -g 1000 -S tmuser && \
    adduser -u 100 -S -G tmuser tmuser sudoRUN -h "$TMHOME"
RUN export PATH=$PATH:$(go env GOPATH)/bin
RUN go get github.com/bazelbuild/bazelisk
RUN ln -s $(go env GOPATH)/bin/bazelisk /usr/bin/bazel
#Run the container with tmuser by default. (UID=100, GID=1000)
USER 100:1000

# Expose the data directory as a volume since there's mutable state in there
VOLUME [ $TMHOME ]

WORKDIR $TMHOME

# p2p and rpc port
EXPOSE 26656 26657

#ENTRYPOINT ["/usr/bin/tendermint"]
#CMD ["node", "--moniker=`hostname`"]
STOPSIGNAL SIGTERM

ARG BINARY=tendermint
COPY $BINARY /usr/bin/tendermint






