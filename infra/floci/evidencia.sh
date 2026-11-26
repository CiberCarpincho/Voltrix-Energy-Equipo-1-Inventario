#!/usr/bin/env bash
# Imprime en Markdown la evidencia del clúster para la entrega:
#   infra/floci/evidencia.sh > infra/floci/EVIDENCIA.md
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/entorno.sh"

seccion() {
  echo
  echo "## $1"
  echo
  echo '```'
  shift
  "$@"
  echo '```'
}

echo "# Evidencia Floci EKS · $(date --iso-8601=seconds)"
seccion "Floci" docker compose -f "$DIR/compose.yaml" ps --format 'table {{.Name}}\t{{.Image}}\t{{.Status}}'
seccion "Clúster (aws eks describe-cluster)" aws eks describe-cluster --name "$CLUSTER" \
  --query 'cluster.{nombre:name,estado:status,version:version,endpoint:endpoint,rol:roleArn,subredes:resourcesVpcConfig.subnetIds}' \
  --output table
seccion "Rol del clúster: políticas adjuntas" aws iam list-attached-role-policies --role-name "$CLUSTER-cluster" --output table
seccion "Usuario de despliegue: política en línea" aws iam get-user-policy --user-name "$CLUSTER-despliegue" \
  --policy-name eks-describe-inventario --query 'PolicyDocument' --output json
seccion "kubectl get nodes (contexto $(kubectl config current-context))" kubectl get nodes -o wide
seccion "kubectl get pods -A" kubectl get pods -A
