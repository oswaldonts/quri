# go-income-mgr: income manager HTTP API written in Go.
#
# Same job definition for every environment; per-environment values live in
# <env>.vars.hcl and the image is passed at deploy time:
#
#   cd deploy/nomad
#   nomad job run -var-file=dev.vars.hcl -var image=go-income-mgr:<tag> go-income-mgr.nomad.hcl
#
# Requirements on the client node:
#   - host_volume "go-income-mgr-data" owned by UID 65532 (distroless nonroot user):
#     SQLite database file, persisted across deploys
#
# No secrets yet, so there is no vault block.

variable "image" {
  type        = string
  description = "Container image: local build in dev (go-income-mgr:<tag>), registry in prod"
}

variable "namespace" {
  type        = string
  description = "Nomad namespace"
  default     = "dev"
}

variable "http_port" {
  type        = number
  description = "Static host port (loopback only through host_network \"local\")"
  default     = 8080
}

job "go-income-mgr" {
  namespace = var.namespace
  type      = "service"

  group "api" {
    count = 1

    # Deregister the service before stopping the task so in-flight requests can finish.
    shutdown_delay = "5s"

    network {
      port "http" {
        static       = var.http_port
        to           = 8080
        host_network = "local"
      }
    }

    volume "data" {
      type      = "host"
      source    = "go-income-mgr-data"
      read_only = false
    }

    service {
      name     = "go-income-mgr"
      provider = "nomad"
      port     = "http"

      check {
        type     = "http"
        path     = "/health"
        interval = "15s"
        timeout  = "3s"
      }
    }

    task "api" {
      driver = "docker"

      # Time between SIGTERM and SIGKILL; the API drains requests for up to 10s.
      kill_timeout = "15s"

      config {
        image = var.image
        ports = ["http"]
      }

      volume_mount {
        volume      = "data"
        destination = "/data"
      }

      env {
        PORT          = "8080"
        DATABASE_PATH = "/data/income.db"
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
