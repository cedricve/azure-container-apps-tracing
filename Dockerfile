FROM mcr.microsoft.com/devcontainers/go:1.24-bookworm AS builder
LABEL AUTHOR=cedricve

ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:/usr/local/lib:$PATH
ENV GOSUMDB=off
ENV REPO_PATH=cedricve/slow-api

##############################################################################
# Copy all the relevant source code in the Docker image, so we can build this.

ARG github_username
ARG github_token
RUN git config --global \
    url."https://${github_username}:${github_token}@github.com/".insteadOf \
    "https://github.com/"

##########################################
# Installing some additional dependencies.

RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential cmake pkg-config unzip libgtk2.0-dev \
    curl ca-certificates libcurl4-openssl-dev libssl-dev libjpeg62-turbo-dev && \
    rm -rf /var/lib/apt/lists/*

##############################################################################
# Copy all the relevant source code in the Docker image, so we can build this.


RUN mkdir -p /go/src/github.com/${REPO_PATH}
COPY . /go/src/github.com/${REPO_PATH}

##################
# Build API

RUN cd /go/src/github.com/${REPO_PATH} && \
    go mod download && \
    go build -tags timetzdata,netgo --ldflags '-s -w -extldflags "-static -latomic"' main.go && \
    mkdir -p /project && mv main /project && \
    rm -rf /go/src/github.com

####################################
# Let's create a /dist folder containing just the files necessary for runtime.
# Later, it will be copied as the / (root) of the output image.

WORKDIR /dist
RUN cp -r /project ./

FROM alpine:latest

############################
# Protect by non-root user.

RUN addgroup -S cedricve && adduser -S project -G cedricve && addgroup project video

#################################
# Copy files from previous images

COPY --chown=0:0 --from=builder /dist /

############################
# Move directory to /var/lib

RUN apk update && apk add ca-certificates curl libstdc++ libc6-compat --no-cache && rm -rf /var/cache/apk/*
RUN mv /project/* /home/project/

###########################
# Grant the necessary root capabilities to the process trying to bind to the privileged port
RUN apk add libcap && setcap 'cap_net_bind_service=+ep' /home/project/main

###################
# Run non-root user

USER project

######################################
# By default the app runs on port 80

EXPOSE 8081

######################################
# Check if vault is still running

HEALTHCHECK CMD curl --fail http://localhost:8081 || exit 1   

###################################################
# Leeeeettttt'ssss goooooo!!!
# Run the shizzle from the right working directory.

WORKDIR /home/project
CMD ["/home/project/main", "serve"]
