variable "floci_endpoint" {
  description = "URL del emulador Floci vista desde Terraform"
  type        = string
  default     = "http://localhost:4566"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "nombre_cluster" {
  type    = string
  default = "voltrix-inventario"
}

variable "version_kubernetes" {
  type    = string
  default = "1.33"
}
