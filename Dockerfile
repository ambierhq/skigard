# Image used by GoReleaser (dockers_v2). The prebuilt static binary is copied from the
# build context; nothing is compiled here. Run as non-root, no shell, no package manager.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/skigard /usr/local/bin/skigard
ENV HOME=/home/nonroot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/skigard"]
CMD ["--help"]
