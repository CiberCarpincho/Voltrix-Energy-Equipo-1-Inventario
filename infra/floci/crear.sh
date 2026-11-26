#!/usr/bin/env bash
# Crea el clúster EKS emulado de Inventario en Floci, solo con código:
#   1. levanta Floci (compose.yaml)
#   2. aplica la IaC de terraform/ (VPC, IAM de mínimo privilegio y clúster EKS)
#   3. genera un kubeconfig propio y comprueba los nodos con kubectl
# Requisitos: Docker en modo root, AWS CLI v2 y kubectl. Terraform corre en un contenedor.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$DIR/entorno.sh"

echo "==> Levantando Floci en $AWS_ENDPOINT_URL"
docker compose -f "$DIR/compose.yaml" up -d
for _ in $(seq 1 60); do
  curl -fsS "$AWS_ENDPOINT_URL/_localstack/health" >/dev/null 2>&1 && break
  sleep 2
done
curl -fsS "$AWS_ENDPOINT_URL/_localstack/health" >/dev/null || { echo "Floci no respondió"; exit 1; }

echo "==> Aplicando la IaC (Terraform $TERRAFORM_IMAGEN)"
terraform init -input=false

# Floci no responde a tiempo el primer CreateCluster (arranca el contenedor k3s mientras
# tanto); el SDK lo reintenta y Floci contesta 409 aunque el clúster sí se está creando.
# En ese caso se espera a que el clúster sea visible, se importa al estado y se completa el apply.
if ! terraform apply -input=false -auto-approve; then
  for _ in $(seq 1 60); do
    aws eks describe-cluster --name "$CLUSTER" >/dev/null 2>&1 && break
    sleep 2
  done
  aws eks describe-cluster --name "$CLUSTER" >/dev/null || exit 1
  echo "==> El clúster ya existe en Floci; importándolo al estado de Terraform"
  aws eks wait cluster-active --name "$CLUSTER"
  terraform state list | grep -qx 'aws_eks_cluster.inventario' ||
    terraform import -input=false aws_eks_cluster.inventario "$CLUSTER"
  terraform apply -input=false -auto-approve
fi

echo "==> Esperando a que el clúster $CLUSTER esté ACTIVE"
aws eks wait cluster-active --name "$CLUSTER"

echo "==> Guardando la clave del usuario de despliegue (perfil $PERFIL_DESPLIEGUE)"
aws configure set aws_access_key_id "$(terraform output -raw clave_despliegue_id)" --profile "$PERFIL_DESPLIEGUE"
aws configure set aws_secret_access_key "$(terraform output -raw clave_despliegue_secreto)" --profile "$PERFIL_DESPLIEGUE"
chmod 600 "$AWS_SHARED_CREDENTIALS_FILE"

echo "==> Generando el kubeconfig en $KUBECONFIG (contexto $CONTEXTO)"
aws eks update-kubeconfig --name "$CLUSTER" --alias "$CONTEXTO" \
  --profile "$PERFIL_DESPLIEGUE" --kubeconfig "$KUBECONFIG" >/dev/null

echo "==> Nodos del clúster"
for _ in $(seq 1 30); do
  kubectl get nodes --no-headers 2>/dev/null | grep -q ' Ready' && break
  sleep 2
done
kubectl get nodes -o wide

echo
echo "Listo. Para usar kubectl en esta terminal:"
echo "  source infra/floci/entorno.sh"
