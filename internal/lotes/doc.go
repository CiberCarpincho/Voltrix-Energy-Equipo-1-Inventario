// Package lotes es el núcleo de procesamiento paralelo de la capa de lógica de negocio
// (DAS, sección 4): un pool acotado de goroutines que aplica los cambios de estado de un lote.
//
// Diseño acordado en el DAS, para implementar con la historia de actualización por lotes:
//
//   - El número de workers viene de config.Config.Workers y no supera el pool de conexiones.
//   - Descomposición de datos: cada cambio se asigna al worker hash(id) mod N, así todos los
//     cambios de un mismo activo van al mismo worker y se aplican en orden de llegada.
//   - Cada worker escribe el resultado en la posición original del cambio, de modo que la
//     respuesta sale en el orden de entrada sin ordenar al final.
//   - La versión secuencial de referencia es el mismo código con un solo worker; el benchmark
//     de benchmarks/ compara ambas sobre el mismo lote.
//
// Este paquete no conoce HTTP ni SQL: recibe la función que aplica un cambio (provista por
// internal/servicio, que a su vez usa el repositorio).
package lotes
