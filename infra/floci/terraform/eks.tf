resource "aws_eks_cluster" "inventario" {
  name     = var.nombre_cluster
  version  = var.version_kubernetes
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    subnet_ids              = [for s in aws_subnet.inventario : s.id]
    endpoint_public_access  = true
    endpoint_private_access = false
  }

  tags = {
    proyecto = "voltrix-energy"
    equipo   = "equipo-1"
    servicio = "inventario-de-activos"
  }

  depends_on = [aws_iam_role_policy_attachment.cluster]
}
