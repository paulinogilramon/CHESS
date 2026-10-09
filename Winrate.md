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

## Round-robin IA vs IA (08/10/2026)

Comparación directa red contra red con `arena -weights A.bin -weights2 B.bin`, **50 partidas por
enfrentamiento** (25 por color), `-ms 30`, profundidad ≤ 6 y semillas aleatorias. Cada red juega
4 enfrentamientos → **200 partidas por red**.

```powershell
go run ./cmd/arena -weights wA.bin -weights2 wB.bin -games 25 -cores 1 -ms 30
```

Matriz de enfrentamientos (victorias de la fila sobre la columna, de 50 partidas):

| Fila \ Col | w1 | w2 | w3 | w4 | w5 |
|---|---|---|---|---|---|
| **w1** | — | 26 | 26 | 29 | 24 |
| **w2** | 24 | — | 19 | 19 | 21 |
| **w3** | 24 | 31 | — | 28 | 29 |
| **w4** | 21 | 31 | 22 | — | 23 |
| **w5** | 26 | 29 | 21 | 27 | — |

Clasificación round-robin (200 partidas cada una, sin empates):

| Puesto | Red | Puntuación |
|---|---|---|
| 1 | **w3** | **56,0%** |
| 2 | w1 | 52,5% |
| 3 | w5 | 51,5% |
| 4 | w4 | 48,5% |
| 5 | w2 | 41,5% |

## Conclusión

- **`w3.bin` es la mejor red de las entrenadas** tanto frente al clásico como en el round-robin IA vs IA.
- Le siguen `w1` y `w5`, muy igualadas; `w2` es claramente la más débil.
- Una muestra de solo 20 partidas por enfrentamiento daba `w5` como líder: es ruido estadístico.
  Con 50 por enfrentamiento el resultado converge y confirma a `w3`.
- **`weights.bin` es `w3.bin`** (promovida el 08/10/2026), por lo que el motor ya usa la mejor red.

## Redes nuevas con `-steps` (09/10/2026)

Entrenadas el 08–09/10/2026 con el nuevo flag `-steps` (presupuesto exacto de actualizaciones Adam):
`w1_1m.bin` … `w4_1m.bin` (mismo archivo de pesos, 106.753 parámetros).

### Vs. clásico (30 partidas por red, `-seed 7 -ms 30`)

| Red | Score | Red antigua (misma data) | Score antigua |
|---|---|---|---|
| w4_1m | **46,7%** | w4 | 33,3% |
| w2_1m | 36,7% | w2 | 16,7% |
| w1_1m | 33,3% | w1 | 25,0% |
| w3_1m | 30,0% | w3 | **53,3%** (referencia hoy) |

### Head-to-head vs `w3` (50 partidas por red, semillas aleatorias)

| Red | Score vs w3 |
|---|---|
| w1_1m | 46,0% |
| w4_1m | 46,0% |
| w3_1m | 42,0% |
| w2_1m | 38,0% |

### Conclusión

- Las redes nuevas **mejoran frente a sus versiones antiguas** en w1, w2 y w4 (w4_1m es la mejor
  de las nuevas con 46,7% vs. el clásico).
- **Ninguna supera a `w3`**: todas quedan por debajo del 50% en el duelo directo contra ella, y
  `w3_1m` es claramente peor que la `w3` original.
- `weights.bin` sigue siendo `w3.bin`; no procede ningún cambio de promoción.

## Cronología de entrenamiento

| Red | Dataset | Fecha |
|---|---|---|
| w1 | `dataset.bin` (238.370 muestras) | 22/09/2026 17:35 |
| w2 | (siguiente iteración) | 22/09/2026 17:39 |
| w3 | `relabel.bin` (39.729 muestras) | 22/09/2026 18:18 |
| w4 | `relabel4.bin` (29.797 muestras) | 22/09/2026 18:32 |
| w5 | `relabel5.bin` (19.865 muestras) | 22/09/2026 19:05 |
