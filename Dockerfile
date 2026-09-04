# Multi-stage: compile in a full toolchain image, copy only the binary into
# the runtime stage. Distroless because the build is static — there's no libc
# to bring along, and nothing left in the image to exec into if it's ever
# reached from outside.
FROM golang:1.27.1-bookworm@sha256:648f440f42a0958804efb24df176f806f9d353b41f1c0627f666428e40310f6b AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static, so the distroless base below is enough. -trimpath keeps build
# machine paths out of the binary. The mkdir rides along in the same RUN
# purely to avoid a second layer for it - it's otherwise unrelated to the
# build: baked into this stage (which has a shell) rather than the
# distroless one below (which doesn't), because DB_PATH defaults to a file
# under here, and a write-path token is issued by running this same binary
# again against that file (`token issue`), so it needs to exist and be
# writable before the distroless stage's own USER ever runs.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hush-hush ./cmd/hush-hush && \
    mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=build /out/hush-hush /hush-hush
# Docker seeds a freshly created named volume from the image directory it's
# mounted over, ownership included - baking that in here is what lets
# compose.yaml's volume come up already writable by nonroot, with no
# separate chown step needed at first run.
COPY --from=build --chown=nonroot:nonroot /out/data /data

# The :nonroot base image variant already sets the user, so this is explicit
# rather than load-bearing — a project stamped from this template that swaps
# the base image keeps the guarantee anyway.
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/hush-hush"]
