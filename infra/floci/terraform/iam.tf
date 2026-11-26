# Rol del plano de control: solo el servicio EKS puede asumirlo y solo lleva la
# política administrada que EKS necesita para operar el clúster.
data "aws_iam_policy_document" "confianza_eks" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  name               = "${var.nombre_cluster}-cluster"
  assume_role_policy = data.aws_iam_policy_document.confianza_eks.json
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# Usuario con el que el equipo obtiene el kubeconfig y despliega: solo puede leer
# la descripción de ESTE clúster (lo que necesita `aws eks update-kubeconfig`).
resource "aws_iam_user" "despliegue" {
  name = "${var.nombre_cluster}-despliegue"
}

data "aws_iam_policy_document" "despliegue" {
  statement {
    sid       = "LeerSoloEsteCluster"
    actions   = ["eks:DescribeCluster"]
    resources = [aws_eks_cluster.inventario.arn]
  }
}

# Floci rechaza a propósito el par público test/test en la autenticación de EKS,
# así que kubectl firma sus tokens con la clave de este usuario.
resource "aws_iam_access_key" "despliegue" {
  user = aws_iam_user.despliegue.name
}

resource "aws_iam_user_policy" "despliegue" {
  name   = "eks-describe-inventario"
  user   = aws_iam_user.despliegue.name
  policy = data.aws_iam_policy_document.despliegue.json
}
