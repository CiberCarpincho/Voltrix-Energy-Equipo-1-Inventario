# Clúster EKS emulado en Floci (T-INV-06)

[Floci](https://github.com/floci-io/floci) emula AWS en `http://localhost:4566`. Su servicio EKS
crea cada clúster como un contenedor k3s con un API server real, así que `aws eks` y `kubectl`
funcionan como en AWS. Todo se crea con código: `compose.yaml` levanta Floci y `terraform/`
crea la red, el IAM y el clúster.

## Requisitos

- Docker en modo root (Floci usa el socket de Docker para arrancar k3s).
- AWS CLI v2 y `kubectl`.
- No hace falta instalar Terraform: los scripts lo ejecutan en el contenedor `hashicorp/terraform:1.17.1`.
- Puertos libres: `4566` (Floci) y `6500` (API server del clúster).

## Uso

```bash
infra/floci/crear.sh                # Floci + IaC + kubeconfig, y muestra los nodos (≈ 1–2 min)
source infra/floci/entorno.sh       # deja AWS CLI y kubectl apuntando a Floci en esta terminal
kubectl get nodes
infra/floci/evidencia.sh > infra/floci/EVIDENCIA.md   # evidencia para la entrega
infra/floci/destruir.sh             # borra el clúster, el IAM y la red
infra/floci/destruir.sh --todo      # además apaga Floci y borra sus datos
```

`crear.sh` es idempotente: si el clúster ya existe, solo comprueba el estado.

## Qué crea

| Recurso | Archivo | Detalle |
|---|---|---|
| VPC y 2 subredes | `terraform/red.tf` | `10.40.0.0/16`, en `us-east-1a` y `us-east-1b` (EKS exige dos zonas) |
| Rol del clúster | `terraform/iam.tf` | Solo `eks.amazonaws.com` puede asumirlo; solo tiene `AmazonEKSClusterPolicy` |
| Usuario de despliegue | `terraform/iam.tf` | Solo `eks:DescribeCluster` sobre este clúster; su clave firma los tokens de `kubectl` |
| Clúster `voltrix-inventario` | `terraform/eks.tf` | Kubernetes 1.33 (Floci usa `rancher/k3s:v1.33.1-k3s1`) |

## Decisiones y particularidades de Floci

- **Estado persistente.** La imagen de Floci guarda todo en memoria por defecto. `compose.yaml`
  activa `FLOCI_STORAGE_MODE=persistent` para que el IAM y el clúster sobrevivan a un reinicio,
  y Floci vuelve a adoptar el contenedor k3s.
- **Autenticación de `kubectl`.** Floci rechaza a propósito el par público `test`/`test` en EKS
  porque el token da acceso de administrador. Por eso Terraform crea una clave IAM para el
  usuario de despliegue y `crear.sh` la guarda en `infra/floci/credenciales` (perfil
  `inventario-despliegue`). Ese archivo, el `kubeconfig` y el estado de Terraform no se versionan.
- **El 409 en la primera creación.** Floci tarda en responder el primer `CreateCluster` mientras
  arranca k3s. El SDK reintenta con el mismo `clientRequestToken`, Floci contesta
  `409 ResourceInUseException` y el clúster sí queda creado. `crear.sh` lo detecta, importa el
  clúster al estado de Terraform y completa el `apply`.
- **Kubeconfig propio.** Se escribe en `infra/floci/kubeconfig` (contexto `floci-inventario`) para
  no tocar `~/.kube/config`.
- **Almacenamiento.** Se conserva el provisionador `local-path` de k3s para el PVC de PostgreSQL de
  los manifiestos (T-INV-07).
