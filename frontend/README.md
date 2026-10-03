# RkTopNG — frontend

Interfaccia della dashboard: **Svelte 5 + Vite + TypeScript**. Due interfacce separate, **mobile** e
**desktop**, che condividono lo stesso strato dati. Il build viene incorporato nel binario Go
(`exporter/web`), quindi in produzione non serve Node.

Documentazione completa: [`../docs/FUNZIONAMENTO.md`](../docs/FUNZIONAMENTO.md) (sezione 6).
Specifica visiva: i mockup approvati in [`../mockups/`](../mockups/).

## Comandi

```bash
npm install
RKTOP_API=http://<ip-board>:9888 npm run dev   # ricarica a caldo; /api inoltrato a un backend vero
npm test                                          # Vitest
npm run check                                     # tipi (svelte-check + tsc)
npm run build                                     # scrive in ../exporter/web/dist
```

(Dalla radice del progetto: `make dev`, `make test`, `make frontend`.)

## Struttura

```
src/lib/api/     tipi del JSON e client (fetch + EventSource)
src/lib/data/    funzioni pure e testate: metriche, calcoli derivati, acceleratori, storico, soglie, formati, nomi, finestra S.M.A.R.T.
src/lib/state/   store reattivi (rune di Svelte 5) e collegamento al backend
src/lib/charts/  disegno dei grafici su canvas (portato dai mockup) e colori dei token
src/ui/          le due interfacce (Mobile, Desktop) e layout.ts (quale delle due sta disegnando)
src/ui/parts/    pezzi condivisi da mobile e desktop: Card, Badge, Stat, Gauge, BarRow/RowGroup, ChartTile, AreaChart, Spark (canvas)…
src/ui/sections/ le card di ogni sezione (Cpu, Temperature, Accelerators, Memory, Storage, Network, Processes), usate da tutte e due
src/ui/desktop/  solo desktop: TopBar, Overview
src/ui/smart/    solo desktop: la finestra S.M.A.R.T. di un disco (SmartWindow, SmartCards, SmartTempChart, SmartTable); i calcoli sono in lib/data/smart.ts
src/ui/mobile/   solo mobile: TabBar (i pulsanti di sezione)
```

Regola: i calcoli stanno in `lib/data`, gli store tengono lo stato ed eseguono l'I/O, i componenti
mostrano soltanto.

Regola degli indirizzi: **mai un indirizzo che inizia con `/`** (`api/info`, non `/api/info`), perché la dashboard
può stare sotto un sottopercorso dietro un reverse proxy; `vite.config.ts` usa `base: './'`. Lo verifica
`src/lib/api/relative-urls.test.ts`.
