# Variables comunes para trabajar contra Floci. Úsese con: source infra/floci/entorno.sh
DIR_FLOCI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL:-http://localhost:4566}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"
export AWS_PAGER=""
# Credenciales locales (no versionadas): [default] es el par público test/test que usan
# Terraform y la AWS CLI; [inventario-despliegue] es la clave IAM que crea Terraform y
# con la que kubectl obtiene su token (Floci no acepta test/test para EKS).
export AWS_SHARED_CREDENTIALS_FILE="$DIR_FLOCI/credenciales"
export AWS_CONFIG_FILE=/dev/null
[[ -f "$AWS_SHARED_CREDENTIALS_FILE" ]] ||
  printf '[default]\naws_access_key_id = test\naws_secret_access_key = test\n' >"$AWS_SHARED_CREDENTIALS_FILE"

export CLUSTER="${CLUSTER:-voltrix-inventario}"
export CONTEXTO="${CONTEXTO:-floci-inventario}"
export PERFIL_DESPLIEGUE="inventario-despliegue"
# Kubeconfig propio para no tocar ~/.kube/config.
export KUBECONFIG="$DIR_FLOCI/kubeconfig"

export TERRAFORM_IMAGEN="${TERRAFORM_IMAGEN:-hashicorp/terraform:1.17.1}"

# Terraform en contenedor: comparte la red del host para llegar a localhost:4566.
terraform() {
  docker run --rm --network host \
    -u "$(id -u):$(id -g)" -e HOME=/tmp \
    -e TF_VAR_floci_endpoint="$AWS_ENDPOINT_URL" \
    -e TF_VAR_nombre_cluster="$CLUSTER" \
    -v "$DIR_FLOCI/terraform:/trabajo" -w /trabajo \
    "$TERRAFORM_IMAGEN" "$@"
}
