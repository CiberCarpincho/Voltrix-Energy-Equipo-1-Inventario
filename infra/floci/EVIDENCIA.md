# Evidencia Floci EKS · 2026-11-26T00:20:02-05:00

## Floci

```
NAME                    IMAGE               STATUS
voltrix-floci-floci-1   floci/floci:2.5.0   Up About a minute (healthy)
```

## Clúster (aws eks describe-cluster)

```
---------------------------------------------------------------------------
|                             DescribeCluster                             |
+----------+--------------------------------------------------------------+
|  endpoint|  https://localhost:6500                                      |
|  estado  |  ACTIVE                                                      |
|  nombre  |  voltrix-inventario                                          |
|  rol     |  arn:aws:iam::000000000000:role/voltrix-inventario-cluster   |
|  version |  1.33                                                        |
+----------+--------------------------------------------------------------+
||                               subredes                                ||
|+-----------------------------------------------------------------------+|
||  subnet-82e270aa                                                      ||
||  subnet-f9c94151                                                      ||
|+-----------------------------------------------------------------------+|
```

## Rol del clúster: políticas adjuntas

```
--------------------------------------------------------------------------------
|                           ListAttachedRolePolicies                           |
+------------------------------------------------------------------------------+
||                              AttachedPolicies                              ||
|+-------------------------------------------------+--------------------------+|
||                    PolicyArn                    |       PolicyName         ||
|+-------------------------------------------------+--------------------------+|
||  arn:aws:iam::aws:policy/AmazonEKSClusterPolicy |  AmazonEKSClusterPolicy  ||
|+-------------------------------------------------+--------------------------+|
```

## Usuario de despliegue: política en línea

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "eks:DescribeCluster",
            "Effect": "Allow",
            "Resource": "arn:aws:eks:us-east-1:000000000000:cluster/voltrix-inventario",
            "Sid": "LeerSoloEsteCluster"
        }
    ]
}
```

## kubectl get nodes (contexto floci-inventario)

```
NAME                               STATUS   ROLES                  AGE   VERSION        INTERNAL-IP   EXTERNAL-IP   OS-IMAGE           KERNEL-VERSION   CONTAINER-RUNTIME
i-e4b0f81dfda73006a.ec2.internal   Ready    control-plane,master   85s   v1.33.1+k3s1   10.40.2.10    <none>        K3s v1.33.1+k3s1   7.2.9-arch1-1    containerd://2.0.5-k3s1
```

## kubectl get pods -A

```
NAMESPACE     NAME                                      READY   STATUS    RESTARTS   AGE
kube-system   coredns-697968c856-cwshw                  1/1     Running   0          81s
kube-system   local-path-provisioner-774c6665dc-5jsms   1/1     Running   0          81s
kube-system   metrics-server-6f4c6675d5-gkglg           1/1     Running   0          81s
```
