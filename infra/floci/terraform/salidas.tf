output "nombre_cluster" {
  value = aws_eks_cluster.inventario.name
}

output "endpoint" {
  value = aws_eks_cluster.inventario.endpoint
}

output "estado" {
  value = aws_eks_cluster.inventario.status
}

output "rol_cluster" {
  value = aws_iam_role.cluster.arn
}

output "usuario_despliegue" {
  value = aws_iam_user.despliegue.arn
}

output "clave_despliegue_id" {
  value = aws_iam_access_key.despliegue.id
}

output "clave_despliegue_secreto" {
  value     = aws_iam_access_key.despliegue.secret
  sensitive = true
}
