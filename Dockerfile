FROM alpine:3.10
RUN apk add jq
COPY --chown=0:0 image/ /
CMD ["/xcluster-cni.sh", "start"]
