# Winrate — comparativa de redes entrenadas

Fecha de la evaluación: 08/10/2026
Redes evaluadas: `w1.bin` … `w5.bin` (arquitectura 128→64→1, ~107k parámetros, entrenadas el 22/09/2026).
`weights.bin` (la que carga el motor) era una copia de `w5.bin`; desde el 08/10/2026 es una copia de `w3.bin`, la mejor red.

## Metodología

Cada red se enfrenta al motor clásico (mismo negamax, evaluación material/clásica en lugar de la red)
usando el comando `arena`:

```powershell
go run ./cmd/arena -weights wN.bin -games 15 -cores 2 -seed 7 -ms 30
```

- Aperturas aleatorias fijas (mismo `seed` para todas las redes → aperturas compartidas).
- Nota: desde el 08/10/2026 `arena` usa semilla **aleatoria por defecto** (`-seed N` para fijarla y reproducir).
  Los resultados de abajo se obtuvieron con `-seed 7`.
- Partidas emparejadas por colores (cada apertura se juega con cada color).
- 30 ms/movimiento, profundidad ≤ 6, tope 240 plies.
- Score = (victorias + empates/2) / partidas. 50% = igualado con el clásico.

## Resultados rápidos (12 partidas por red)

| Red | Victorias | Empates | Derrotas | Score |
|---|---|---|---|---|
| w1 | 3 | 0 | 9 | 25,0% |
| w2 | 2 | 0 | 10 | 16,7% |
| w3 | 5 | 0 | 7 | 41,7% |
| w4 | 4 | 0 | 8 | 33,3% |
| w5 (= weights.bin) | 5 | 0 | 7 | 41,7% |

## Desempate w3 vs w5 (30 partidas por red)

| Red | Victorias | Empates | Derrotas | Score |
|---|---|---|---|---|
| **w3** | 17 | 0 | 13 | **56,7%** |
| w5 (= weights.bin) | 12 | 0 | 18 | 40,0% |

## Acumulado

| Red | Partidas | Score acumulado |
|---|---|---|
| **w3** | 42 | **52,4%** |
| w5 (= weights.bin) | 42 | 40,5% |
| w4 | 12 | 33,3% |
| w1 | 12 | 25,0% |
| w2 | 12 | 16,7% |

## Conclusión

- **`w3.bin` es la mejor red de las entrenadas** y única con score ≥ 50% frente al clásico en el desempate.
- `w5.bin` queda segunda, con ~12 puntos porcentuales menos.
- **`weights.bin` ahora es `w3.bin`** (promovida el 08/10/2026).
- Muestra limitada (~40 partidas por líder); para mayor confianza, repetir con más partidas y semillas.

## Cronología de entrenamiento

| Red | Dataset | Fecha |
|---|---|---|
| w1 | `dataset.bin` (238.370 muestras) | 22/09/2026 17:35 |
| w2 | (siguiente iteración) | 22/09/2026 17:39 |
| w3 | `relabel.bin` (39.729 muestras) | 22/09/2026 18:18 |
| w4 | `relabel4.bin` (29.797 muestras) | 22/09/2026 18:32 |
| w5 | `relabel5.bin` (19.865 muestras) | 22/09/2026 19:05 |
