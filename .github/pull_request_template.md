## Tipo de cambio

- [ ] Historia de usuario
- [ ] Corrección
- [ ] Infraestructura, CI/CD o configuración
- [ ] Documentación

## Historia de usuario

HU-XX · (título) <!-- o "No aplica" -->

## Qué cambia

-

## Criterios de aceptación cubiertos

<!-- Solo si es una historia de usuario. -->

- [ ] c1:
- [ ] c2:

## Pruebas

- [ ] Cada criterio tiene su prueba (servicio con dobles, manejador con httptest, repositorio de integración).
- [ ] `go test ./...` pasa, también con `INVENTARIO_TEST_DSN` si toqué el repositorio o las migraciones.
- [ ] `golangci-lint run` pasa (incluye las reglas de capas).

## Revisión de arquitectura (DAS)

- [ ] Cada capa usa solo la de abajo: `app/api` → `app/business/services` → `app/infraestructure`; todas pueden usar `app/business/domain`.
- [ ] El servicio recibe interfaces de `app/business/services/puertos.go`, no `app/infraestructure` directamente.
- [ ] Las consultas son parametrizadas y los errores pasan por `traducirError`.
- [ ] Las respuestas siguen `docs/openapi.yaml` (nombres JSON, códigos HTTP y códigos de error).
- [ ] Si cambié el esquema, agregué una migración nueva (no edité una existente) y admite la versión anterior de la API.
- [ ] Si cambié el contrato, lo acordé con los equipos que lo consumen.

## Despliegue

<!-- Al fusionar en main se despliega en la EC2 (cd.yml). -->

- [ ] Si agregué variables de entorno, actualicé `app/core`, `docker-compose.yaml`, `.env.example` y el README.

**Notas para el despliegue** (cambios en el `.env` de la EC2, pasos manuales):

- Ninguna.
