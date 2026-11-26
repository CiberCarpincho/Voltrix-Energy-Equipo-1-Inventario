#!/usr/bin/env bash
# Elimina el clúster y los recursos IAM/VPC creados por crear.sh.
# Con --todo también detiene Floci y borra su volumen de datos.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/entorno.sh"

terraform destroy -input=false -auto-approve
rm -f "$KUBECONFIG" "$AWS_SHARED_CREDENTIALS_FILE"

if [[ "${1:-}" == "--todo" ]]; then
  docker compose -f "$DIR/compose.yaml" down -v
  # Contenedores y volúmenes que Floci crea por su cuenta (registro ECR y datos de k3s).
  docker rm -f floci-aws-ecr-registry "floci-aws-eks-$CLUSTER" >/dev/null 2>&1 || true
  docker volume rm floci-aws-ecr-registry-data "floci-aws-eks-$CLUSTER" >/dev/null 2>&1 || true
fi
