# Build environment for the desktop app (cmd/desktop): Go plus the GTK/WebKit headers Wails needs.
# Used by "make desktop"; the source is bind-mounted, nothing is copied into the image.
FROM golang:1.27-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev \
    && rm -rf /var/lib/apt/lists/*
