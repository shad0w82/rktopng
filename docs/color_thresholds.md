# RkTopNG — Tabella colori gauge / soglie

Colori semantici usati da gauge (QUICK_LOOKUP), barre e thermal zones della dashboard.
Le soglie sono **inclusive sul limite inferiore** (es. CPU 50 → giallo).

## Colori
| Nome  | Hex       | Variabile CSS |
|-------|-----------|---------------|
| 🟢 good   | `#46c25a` | `--good`   |
| 🟡 warn   | `#e6b34a` | `--warn`   |
| 🟠 orange | `#eb8b3a` | `--orange` |
| 🔴 crit   | `#f2564d` | `--crit`   |

## Soglie
| Metrica                          | 🟢 good | 🟡 warn | 🟠 orange | 🔴 crit |
|----------------------------------|:------:|:------:|:--------:|:------:|
| **CPU %**                        | < 50   | 50–74  | 75–89    | ≥ 90   |
| **RAM %**                        | < 60   | 60–79  | 80–91    | ≥ 92   |
| **Temperatura °C** (SoC + thermal zones + NVMe) | < 55 | 55–69 | 70–84 | ≥ 85 |
| **Capacità disco %** (volumi montati) | < 70 | 70–84 | 85–94 | ≥ 95 |

## Dove sono nel codice
Nel mockup `mockups/rktopng-mobile.html`, funzioni JS:
- `cpuGC(p)` → CPU %
- `ramGC(p)` → RAM %
- `socGC(t)` → Temperatura (gauge SoC del QUICK_LOOKUP, thermal zones e tile NVMe)
- `diskGC(p)` → Capacità disco % (card Storage)

Note: `loadC`/`tempC` sono vecchie funzioni a 3 bande (senza arancio), usate ancora da
barre generiche/acceleratori e dal busy% della card Disk I/O; le gauge, le thermal zones e la
capacità disco usano le 4 bande qui sopra.
