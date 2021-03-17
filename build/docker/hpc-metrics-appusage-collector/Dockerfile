# stage 0: compile go programs
FROM golang:1.16-alpine as build
RUN mkdir -p /tmp/app
WORKDIR /tmp/app
ADD metrics ./metrics
ADD pkg ./pkg
ADD go.mod .
ADD go.sum .
# it seems that we need to build static go binary so that it can be found in the alpine container??
# - same issue found here: https://github.com/github/hub/issues/1818
RUN GOPATH=/tmp/go CGO_ENABLED=0 GOOS=linux go install github.com/Donders-Institute/tg-toolset-golang/metrics/...

# stage 1: build hpc_app_usage_collector container
FROM alpine:latest

# application metadata
LABEL donders.ru.nl.app_name "hpc-metrics-appusage-collector"
LABEL donders.ru.nl.app_maintainer "h.lee@donders.ru.nl"
LABEL donders.ru.nl.app_code_repository "https://github.com/Donders-Institute/tg-toolset-golang"

# copy binary from the build stager
WORKDIR /root
COPY --from=build /tmp/go/bin/hpc-metrics-appusage-collector .

RUN pwd && ls -l && /root/hpc-metrics-appusage-collector -h

## entrypoint in shell form
ENTRYPOINT ["/root/hpc-metrics-appusage-collector"]