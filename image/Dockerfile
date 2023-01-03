FROM alpine:latest
RUN apk add jq iproute2
COPY --chown=0:0 . /
CMD ["/xcluster-cni.sh", "start"]
