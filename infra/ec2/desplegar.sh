#!/usr/bin/env bash
# Crea o actualiza la EC2 de Inventario con CloudFormation y la conecta al pipeline de CD, sin
# pasos manuales:
#   1. lee los parámetros de parametros.env (VPC y subred por defecto si se dejan vacías);
#   2. crea o actualiza la pila (instancia, grupo de seguridad, llave SSH e IP elástica);
#   3. espera a que termine el arranque de la instancia (swap, Docker y .env);
#   4. carga los secretos EC2_HOST, EC2_USUARIO y EC2_LLAVE_SSH en el repositorio de GitHub;
#   5. lanza el workflow CD, que despliega la API.
# Se puede ejecutar de nuevo: actualiza la pila solo si la plantilla o los parámetros cambiaron.
#
# Requisitos: AWS CLI v2 con credenciales (aws configure o variables AWS_*), GitHub CLI
# autenticado (gh auth login) con permiso de administrador en el repositorio, y ssh.
#
# Uso:  infra/ec2/desplegar.sh
#       AWS_REGION=us-east-1 PILA=voltrix-inventario REPOSITORIO=org/repo infra/ec2/desplegar.sh
# Para eliminar todo: aws cloudformation delete-stack --stack-name voltrix-inventario
set -euo pipefail

directorio=$(cd "$(dirname "$0")" && pwd)
plantilla="$directorio/inventario.yaml"
archivo_parametros="$directorio/parametros.env"
pila="${PILA:-voltrix-inventario}"
usuario_ssh=ubuntu

error() { echo "Error: $*" >&2; exit 1; }
paso() { printf '\n==> %s\n' "$*"; }

for comando in aws gh ssh; do
  command -v "$comando" >/dev/null || error "falta '$comando' en el PATH"
done
aws sts get-caller-identity >/dev/null || error "la AWS CLI no tiene credenciales válidas (aws configure)"
gh auth status >/dev/null 2>&1 || error "la GitHub CLI no está autenticada (gh auth login)"
repositorio="${REPOSITORIO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"

# --- 1. Parámetros --------------------------------------------------------------------------
# Sin arreglos asociativos: funciona también con el bash 3.2 de macOS.
tipo=t3.micro
vpc=""
subred=""
sobrescrituras=()
while IFS='=' read -r clave valor || [[ -n "$clave" ]]; do
  clave="${clave//[[:space:]]/}"
  if [[ -z "$clave" || "$clave" == \#* ]]; then
    continue
  fi
  valor="${valor#"${valor%%[![:space:]]*}"}"
  valor="${valor%"${valor##*[![:space:]]}"}"
  case "$clave" in
    VpcId) vpc="$valor" ;;
    PublicSubnet) subred="$valor" ;;
    *)
      if [[ "$clave" == InstanceType && -n "$valor" ]]; then
        tipo="$valor"
      fi
      if [[ -n "$valor" ]]; then
        sobrescrituras+=("$clave=$valor")
      fi
      ;;
  esac
done < "$archivo_parametros"

if [[ -z "$vpc" ]]; then
  vpc=$(aws ec2 describe-vpcs --filters Name=isDefault,Values=true --query 'Vpcs[0].VpcId' --output text)
  [[ "$vpc" != None ]] ||
    error "la región no tiene VPC por defecto: defina VpcId y PublicSubnet en $archivo_parametros"
fi
if [[ -z "$subred" ]]; then
  # Zonas de la región donde existe el tipo de instancia (t3.micro no está en todas).
  zonas=$(aws ec2 describe-instance-type-offerings --location-type availability-zone \
    --filters "Name=instance-type,Values=$tipo" --query 'InstanceTypeOfferings[].Location' --output text)
  [[ -n "$zonas" ]] || error "$tipo no está disponible en esta región"
  subred=$(aws ec2 describe-subnets --filters \
    "Name=vpc-id,Values=$vpc" "Name=default-for-az,Values=true" \
    "Name=availability-zone,Values=$(tr -s '[:space:]' ',' <<<"$zonas" | sed 's/,$//')" \
    --query 'sort_by(Subnets, &AvailabilityZone)[0].SubnetId' --output text)
  [[ "$subred" != None ]] ||
    error "no hay una subred por defecto donde exista $tipo: defina PublicSubnet en $archivo_parametros"
fi
sobrescrituras+=("VpcId=$vpc" "PublicSubnet=$subred")

# --- 2. Pila de CloudFormation ----------------------------------------------------------------
paso "Creando o actualizando la pila $pila en $(aws configure get region || echo "${AWS_REGION:-}")"
printf '    %s\n' "${sobrescrituras[@]}"
aws cloudformation deploy --stack-name "$pila" --template-file "$plantilla" \
  --parameter-overrides "${sobrescrituras[@]}" --no-fail-on-empty-changeset \
  --tags Proyecto=voltrix Equipo=1 Servicio=inventario

salida() {
  aws cloudformation describe-stacks --stack-name "$pila" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}
ip=$(salida PublicIp)
llave_id=$(salida LlaveId)

# --- 3. Arranque de la instancia --------------------------------------------------------------
temporal=$(mktemp -d)
trap 'rm -rf "$temporal"' EXIT
llave="$temporal/llave"
(umask 077 && aws ssm get-parameter --name "/ec2/keypair/$llave_id" --with-decryption \
  --query Parameter.Value --output text > "$llave")

ssh_ec2() {
  ssh -i "$llave" -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new \
    -o UserKnownHostsFile="$temporal/known_hosts" "$usuario_ssh@$ip" "$@"
}

paso "Esperando a que la instancia $ip acepte SSH"
for intento in $(seq 1 30); do
  ssh_ec2 true 2>/dev/null && break
  [[ $intento -lt 30 ]] || error "no se pudo entrar por SSH a $ip"
  sleep 10
done

paso "Esperando a que termine el arranque (swap, Docker y .env)"
estado=0
ssh_ec2 'cloud-init status --wait >/dev/null' || estado=$?
# 0: terminó bien; 2: terminó con advertencias no críticas.
[[ $estado -eq 0 || $estado -eq 2 ]] ||
  error "el arranque de la instancia falló; revisar /var/log/voltrix-inventario.log en la EC2"
ssh_ec2 'docker compose version >/dev/null && test -f ~/voltrix-inventario/.env' ||
  error "la instancia no quedó lista (Docker o .env); revisar /var/log/voltrix-inventario.log"

# --- 4. Secretos del pipeline -----------------------------------------------------------------
paso "Cargando los secretos del CD en $repositorio"
# Los valores van por la entrada estándar: no quedan en la línea de comandos.
printf '%s' "$ip" | gh secret set EC2_HOST --repo "$repositorio"
printf '%s' "$usuario_ssh" | gh secret set EC2_USUARIO --repo "$repositorio"
gh secret set EC2_LLAVE_SSH --repo "$repositorio" < "$llave"

# --- 5. Despliegue ----------------------------------------------------------------------------
if gh api "repos/$repositorio/contents/.github/workflows/cd.yml?ref=main" --silent 2>/dev/null; then
  paso "Lanzando el workflow CD sobre main"
  gh workflow run cd.yml --repo "$repositorio" --ref main
  echo "Seguir el despliegue: gh run watch --repo $repositorio"
else
  paso "cd.yml todavía no está en main: el primer despliegue ocurre al fusionar develop en main"
fi

paso "Listo"
echo "API:   http://$ip:8080/health"
echo "SSH:   la llave está en Parameter Store, /ec2/keypair/$llave_id"
