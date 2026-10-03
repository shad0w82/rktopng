# RkTopNG — Come funziona

Documentazione del funzionamento complessivo: cosa fa il sistema, come è fatto il backend, come usare
ogni endpoint, come è pensato il frontend e come si installa. Gli schemi sono in ASCII, così si leggono
ovunque.

> **Stato al 2026-10-03.** Backend e frontend sono completi e provati sulla board: l'interfaccia desktop e
> quella mobile mostrano tutte le sezioni, e sul desktop ogni disco ha la sua finestra S.M.A.R.T. L'interfaccia
> è in inglese; questa documentazione è in italiano.

## Indice

1. [Cos'è](#1-cosè)
2. [Vista d'insieme](#2-vista-dinsieme)
3. [Il backend (exporter Go)](#3-il-backend-exporter-go)
4. [Gli endpoint HTTP](#4-gli-endpoint-http)
5. [Catalogo delle metriche e dove compaiono nella UI](#5-catalogo-delle-metriche-e-dove-compaiono-nella-ui)
6. [Il frontend](#6-il-frontend)
7. [Installazione: Docker o nativa](#7-installazione-docker-o-nativa)
8. [Sviluppo](#8-sviluppo)
9. [Decisioni di progetto e limiti noti](#9-decisioni-di-progetto-e-limiti-noti)
10. [Documenti correlati](#10-documenti-correlati)

---

## 1. Cos'è

**RkTopNG** è una dashboard web di monitoraggio per la board **FriendlyElec CM3588** (SoC **Rockchip
RK3588**). È pensata come un `btop` da browser, con in più i dati specifici dell'RK3588 (NPU, RGA, GPU
Mali, VPU) e dei sensori della scheda NAS (temperatura dei dischi, tensione di ingresso, USB-C PD,
ventola).

Due parti:

- **Backend (exporter)** — un programma Go, un solo binario statico, che legge i dati dal sistema
  (`/proc`, `/sys`, debugfs, `smartctl`) e li espone via HTTP.
- **Frontend** — una pagina web (Svelte) che mostra i dati in tempo reale, con una versione per
  telefono e una per desktop.

Può girare **in un container Docker** oppure **installata direttamente sul sistema senza Docker**
(servizio systemd). Il codice è lo stesso.

**L'exporter non dipende dalla nostra interfaccia**: la dashboard Svelte è soltanto un client dei suoi endpoint. Chi
vuole può costruire la propria dashboard (Grafana, un'altra web app, uno script) su `/metrics` o sui JSON `/api/*`.

---

## 2. Vista d'insieme

```
┌────────────────────────── CM3588 (RK3588) ───────────────────────────┐
│                                                                      │
│  Kernel / hardware                                                   │
│    /proc · /sys (hwmon, devfreq, thermal, typec, block, power)       │
│    debugfs (root) · smartctl (root)                                  │
│                  │  lettura di file e comandi                        │
│                  ▼                                                   │
│   ┌─────────────── rktopng · un solo binario Go ───────────────┐     │
│   │                                                            │     │
│   │  COLLECTOR   cpu · temp · gpu · npu · rga · vpu · ddr      │     │
│   │              memoria · dischi · filesystem · zfs · smart   │     │
│   │              rete · power · usb-c · processi               │     │
│   │                    │                                       │     │
│   │                    ▼                                       │     │
│   │         Registro Prometheus  (unica fonte dei dati)        │     │
│   │                    │                                       │     │
│   │                    ▼                                       │     │
│   │  Server HTTP :9888                                         │     │
│   │   ├─ /metrics         Prometheus, tutto                    │     │
│   │   ├─ /api/info        identità statica (una volta)         │     │
│   │   ├─ /api/stream      SSE, solo metriche live              │     │
│   │   ├─ /api/processes   tabella dei processi                 │     │
│   │   ├─ /api/smart/<disco> report S.M.A.R.T. su richiesta     │     │
│   │   └─ /api/stats       tutto, per ispezione                 │     │
│   └──────────────────────────────┬─────────────────────────────┘     │
└──────────────────────────────────│───────────────────────────────────┘
                                   │  HTTP
                 ┌─────────────────┴─────────────────┐
                 ▼                                   ▼
           Browser · frontend Svelte           Altre dashboard · Prometheus
           (mobile / desktop)                  (opzionali, a scelta di chi usa)
```

Idea chiave: **ogni dato nasce una volta sola, in un collector, e finisce in un registro Prometheus.**
Da lì escono sia il formato Prometheus (`/metrics`) sia i JSON per la UI. Aggiungere una metrica
significa scrivere un collector: compare da sola in tutti gli output.

---

## 3. Il backend (exporter Go)

Codice in `exporter/`. Un solo processo, nessun database, nessuno storico: legge e serve.

### 3.1 Schema interno

```
 main.go ── registries.go crea 3 registri ── registra i collector ── server.go: rotte, prefisso, password ── avvia il server HTTP
              │     · all  (tutti)                       → /metrics, ?view=all
              │     · live (senza processi né identità)  → /api/stream, ?view=live
              │     · info (solo identità)               → /api/info
              ▼
   ┌────────────────────────── collectors/ ───────────────────────────┐
   │  ogni collector implementa  Describe()  e  Collect()             │
   │  Collect() viene chiamato a ogni "gather" (scrape o push SSE),   │
   │  legge i file e produce le metriche (nome + label + valore)      │
   └──────────────────────────────────────────────────────────────────┘
              │ Gather()
              ▼
   stats.go ── gatherSnapshot(registro della vista, vista) ──▶ JSON { timestamp, metrics }
                 · vista "all"  → tutto
                 · vista "live" → senza identità statica e senza processi
                 · vista "info" → solo identità statica
                 · arrotonda a 2 decimali, scarta NaN/∞
```

### 3.2 I collector

| File | Cosa legge | Root? | Metriche principali |
|---|---|---|---|
| `info.go` | device-tree, `/proc/sys/kernel/osrelease`, `/etc/os-release`, debugfs, `/proc/mpp_service/version`, `/sys/module/*kbase*/version` | solo driver RGA (e NPU, che ha un ripiego senza root) | `soc_info` (con le etichette `npu_driver`, `rga_driver`, `vpu_driver`, `gpu_driver`) |
| `cpu.go` | `/proc/stat`, `cpufreq`, governor | no | `cpu_usage_percent`, `cpu_freq_mhz`, `cpu_seconds_total`, `cpu_governor_info`, `procs`, `context_switches_total` |
| `thermal.go` | `/sys/class/thermal/thermal_zone*` | no | `temp_celsius{zone}` (7 sensori) |
| `gpu.go` | devfreq `fb000000.gpu` | no | `gpu_load_percent`, `gpu_freq_mhz` |
| `npu.go` | debugfs `rknpu/load`, devfreq `fdab0000.npu` | **sì** (carico) | `npu_load_percent{core}`, `npu_freq_mhz` |
| `rga.go` | debugfs `rkrga/load`, `clk/<clock>/clk_rate` | **sì** | `rga_load_percent{scheduler}`, `rga_freq_mhz` |
| `vpu.go` | `/proc/mpp_service/{load,sessions-summary}`, devfreq | no | `vpu_load_percent{unit}`, `vpu_utilization_percent`, `vpu_freq_mhz`, `vpu_sessions` |
| `ddr.go` | devfreq `dmc` | no | `ddr_load_percent`, `ddr_freq_mhz` |
| `memory.go` | `/proc/meminfo` | no | `memory_bytes{type}`, `swap_bytes{type}` (incl. CMA) |
| `power.go` | hwmon: `simple_vin`, `tcpm_source_psy`, `pwmfan`, `nvme` | no | `input_voltage_volts`, `usb_pd_voltage_volts`, `usb_pd_current_amps`, `fan_pwm`, `nvme_temp_celsius` |
| `typec.go` | `power_supply/tcpm-source-psy*`, `typec/port*` | no | `usb_pd_online`, `usb_pd_type`, `typec_port_info` |
| `diskio.go` | `/proc/diskstats` | no | `disk_read/write_bytes_per_second`, `disk_busy_percent` |
| `disks.go` | `/sys/block/*` | no | `disk_info`, `disk_size_bytes` |
| `disktemp.go` | hwmon (NVMe) + cache SMART | no | `disk_temp_celsius{device,source}` |
| `smart.go` | `smartctl -j` (in background) | **sì** | `smart_up`, `smart_temperature_celsius`, `smart_healthy`, `smart_power_on_hours`, `smart_info` |
| `smartdetail*.go` | `smartctl -j -x` / `-a` **su richiesta**, hwmon e sysfs degli NVMe | **sì** | nessuna metrica: serve `/api/smart/<disco>` (vedi [smart_status.md](smart_status.md)) |
| `filesystem.go` | mount di pid 1 + `statfs()` | no | `filesystem_{size,used,avail}_bytes` |
| `zfs.go` | `/proc/spl/kstat/zfs/*/state` | no | `zfs_pool_online`, `zfs_pool_state` |
| `system.go` | `/proc/uptime`, `loadavg`, `net/sockstat`, `/sys/class/net` | no | `uptime_seconds`, `load_average`, `network_*_bytes_per_second`, `network_info{iface,kind}`, `network_speed_mbps{iface}`, `tcp_connections` |
| `processes.go` | `/proc/<pid>/stat`, `cmdline` | no | `process_*` (primi 20), lista completa per `/api/processes` |

Tutti i nomi hanno il prefisso `rk3588_`. Per ogni sorgente, il percorso preciso e come è stata
verificata sulla board sono in [`data_sources.md`](data_sources.md).

### 3.3 Come vengono letti i dati

- **Valori istantanei** (frequenze, temperature, tensioni, meminfo): si legge il file a ogni gather.
- **Contatori che richiedono due letture** (throughput dei dischi, rete, uso CPU per core, CPU% dei
  processi): il collector ricorda la lettura precedente e calcola la differenza nel tempo. Per questo
  *la primissima richiesta dopo l'avvio non contiene queste metriche*: compaiono dalla seconda.
- **SMART** è lento e cambia piano: un thread in background interroga `smartctl` su ogni disco ogni
  60 s (`RKTOP_SMART_INTERVAL`) e tiene l'ultimo risultato in cache; i gather leggono la cache. Usa
  `-n standby`, quindi non sveglia gli HDD fermi. Se un poll fallisce restano i valori buoni
  precedenti e `smart_up` va a 0.
- **Processi**: la scansione di `/proc/<pid>/stat` di tutti i processi avviene solo quando qualcuno la
  chiede (`/api/processes` o uno scrape di `/metrics`), non a ogni push dello stream. La lista resta in
  cache 1 s: più richieste ravvicinate la condividono, e questo tiene sensata anche la CPU% (il kernel
  conta il tempo CPU a tick da 10 ms). `/proc/<pid>/cmdline` si legge solo per le righe restituite.
- **Ogni client dello stream** ha il suo ciclo (un tick al secondo), ma i collector *live* sono avvolti
  da una cache (`collectors.Cached`, `cache.go`): ciascuno viene eseguito **al massimo una volta ogni 70 %
  dell'intervallo** (700 ms con 1 s) e il risultato è condiviso da tutti i client, dai registri e da
  `/metrics`. Serve per due motivi: (1) le velocità (uso CPU, throughput dei dischi, rete) sono la
  differenza tra due letture consecutive: se due client leggono a pochi millisecondi di distanza il secondo
  vede un intervallo nullo e il dato **mancava** (si vedevano 4 core su 8); (2) il costo non cresce con le
  schede aperte. Limite: la primissima lettura dopo una lunga pausa (nessun client) è la media dall'ultima
  volta, quindi per un secondo può non essere "istantanea".

#### Costo di un gather

Ogni lettura di un file di `/proc`, `/sys` o debugfs fa lavorare il kernel, e alcune sono molto più
costose di quanto sembri. Misurate sulla board (profilo `pprof` di 20 s, con uno stream aperto e
`/api/processes` richiesto ogni 2 s, che è ciò che fa l'interfaccia):

| Sorgente | Costo | Come si evita |
|---|---|---|
| temperatura NVMe (hwmon) | ~6 ms per lettura: il kernel manda un comando SMART al disco, 3 dischi × 3 sensori | cache condivisa di 3 s (`nvmeCacheTTL`) tra `power.go` e `disktemp.go` |
| `debugfs/clk/clk_summary` | 269 KB, ~14 ms: scarica tutto l'albero dei clock | si legge solo il `clk_rate` dei 3 clock RGA (0,01 ms); il dump resta come ripiego |
| `/proc/net/tcp` e `tcp6` | percorre la tabella hash di tutti i socket | `/proc/net/sockstat` e `sockstat6` (contatori già pronti); la tabella intera è il ripiego |
| scansione di tutti i processi (~570) | ~8-28 ms | solo su richiesta; un buffer riusato per le letture; non più dentro ogni gather |
| collector dell'identità (`soc_info`, `disk_info`) | piccolo, ma inutile ripeterlo ogni secondo | stanno solo nei registri *all* e *info* |

Risultato (CPU dell'exporter, un core = 100%): **~14,6% → ~4,6%** con la UI aperta, e circa 0% con nessun
client. Il resto è quasi tutto la scansione dei processi, che serve alla tabella.

Per misurare di nuovo: avviare con `RKTOP_PPROF=1` e prendere un profilo
(`go tool pprof 'http://<board>:9888/debug/pprof/profile?seconds=20'`).

### 3.4 Permessi: cosa richiede root

Quasi tutto è leggibile da utente normale. Richiedono **root**:

| Dato | Perché |
|---|---|
| carico NPU e RGA, versione driver RGA (e NPU, con ripiego su `/sys/module/rknpu/version`), clock RGA | stanno in debugfs (`/sys/kernel/debug`, modo 0700) |
| SMART (temperatura SATA, salute, ore, identità) | serve aprire `/dev/sdX`, `/dev/nvmeX` (e il comando `smartctl`) |

**Questo non dipende da Docker o nativo**: dipende da *con quale utente gira il processo*. Nelle
installazioni previste il processo è già root, quindi i dati che richiedono root **ci sono sempre**:

| Modo di esecuzione | Utente | Dati che richiedono root |
|---|---|---|
| **Nativa, servizio systemd** (`install.sh`) | **root** (`User=root` nell'unit) | disponibili |
| **Docker** (`docker-compose.yml`) | root nel container, `privileged: true` | disponibili |
| Avvio a mano come utente normale (`./rktopng`) | l'utente | **assenti** |
| Avvio a mano con `sudo ./rktopng` | root | disponibili |
| Servizio nativo con `User=` cambiato in un utente normale | l'utente | **assenti** |

Quindi usare l'app in modo nativo **non** fa perdere nulla: il servizio installato gira come root. Si
perdono quei dati solo se si sceglie di far girare il processo senza privilegi.

Senza root il servizio funziona ugualmente e semplicemente **non emette** quei dati (i log dicono
perché, una volta per disco). La UI deve mostrare "n/a" quando una metrica manca.

### 3.5 Configurazione (variabili d'ambiente)

| Variabile | Default | Significato |
|---|---|---|
| `RKTOP_PORT` | `9888` | porta HTTP |
| `RKTOP_INTERVAL` | `1s` | ogni quanto `/api/stream` spinge un aggiornamento |
| `RKTOP_BASE_PATH` | (vuoto) | serve **tutto** sotto questo percorso (es. `/rktopng`), per un reverse proxy che lo lascia nell'indirizzo; vedi §7.4 |
| `RKTOP_AUTH_USER`, `RKTOP_AUTH_PASSWORD` | (vuoti) | se impostate **tutte e due**, **ogni** richiesta chiede utente e password (HTTP Basic), `/metrics` compreso; impostarne una sola è un errore e il servizio non parte; vedi §7.4 |
| `RKTOP_PROC_TOP` | `20` | processi emessi come metriche `process_*` |
| `RKTOP_SMART` | (attivo) | `off`/`0`/`false` disattiva SMART (polling e `/api/smart`) |
| `RKTOP_SMART_INTERVAL` | `60s` | periodo di polling SMART (valori sotto 5 s o non validi → si usa il default) |
| `RKTOP_SMARTCTL` | `smartctl` | percorso del comando |
| `RKTOP_PROC_PATH`, `RKTOP_SYS_PATH` | `/proc`, `/sys` | solo nel container: dove sono montati `/proc` e `/sys` dell'host |
| `RKTOP_ROOTFS_PATH` | (vuoto) | solo nel container: prefisso della root dell'host, per `statfs()` |
| `RKTOP_OSRELEASE_PATH` | `/etc/os-release` | file os-release dell'host |
| `RKTOP_PASSWD_PATH` | `/etc/passwd` | per tradurre gli uid dei processi in nomi |
| `RKTOP_DEV_PATH` | `/dev` | dove stanno `/dev/sdX` per SMART |
| `RKTOP_PPROF` | (spento) | se impostato, espone `/debug/pprof/` per profilare CPU e memoria; lascia spento in uso normale, mostra dettagli interni |

Nell'installazione nativa si impostano in `/etc/rktopng/rktopng.env`
(modello: `deploy/native/rktopng.env.example`).

---

## 4. Gli endpoint HTTP

Tutti sulla porta `9888`. Sono in sola lettura (solo `GET`).

| Endpoint | Formato | Contenuto | Chi lo usa |
|---|---|---|---|
| `/` | HTML | **la dashboard** (frontend incorporato nel binario) | persone |
| `/exporter` | HTML | pagina con i collegamenti agli endpoint | persone |
| `/metrics` | testo Prometheus | **tutto**, anche i processi | Prometheus / Grafana, ispezione |
| `/api/info` | JSON | identità **statica** di board e dischi | frontend, **una volta** all'apertura |
| `/api/stream` | SSE | solo le metriche che **cambiano**, una spinta al secondo | frontend, connessione aperta |
| `/api/processes` | JSON | lista processi ordinabile + totale | frontend, solo quando la sezione è visibile |
| `/api/smart/<disco>` | JSON | report S.M.A.R.T. completo di un disco, letto **su richiesta** | frontend, solo quando si apre la finestra del disco |
| `/api/stats` | JSON | istantanea di **tutto** (`?view=live` / `?view=info`) | ispezione, debug, script |

Le risposte JSON hanno `Access-Control-Allow-Origin: *`.

### 4.1 Formato di uno snapshot

`/api/info`, `/api/stream` e `/api/stats` usano lo stesso formato:

```json
{
  "timestamp": 1790896000123,
  "metrics": {
    "rk3588_cpu_usage_percent": [
      { "labels": { "cluster": "little", "core": "0" }, "value": 5.58 },
      { "labels": { "cluster": "big",    "core": "4" }, "value": 27.27 }
    ],
    "rk3588_ddr_freq_mhz": [ { "value": 528 } ]
  }
}
```

- `timestamp` — millisecondi Unix del momento della lettura.
- `metrics` — una chiave per **nome di metrica**; il valore è la lista dei punti (uno per combinazione
  di label). Una metrica senza label ha un solo punto senza `labels`.
- I valori sono arrotondati a 2 decimali. Le metriche che non hanno dati (es. SMART senza root) sono
  semplicemente assenti.

### 4.2 `GET /api/info` — identità statica, una volta

Contiene solo ciò che non cambia mentre il servizio gira: `soc_info`, `disk_info`, `disk_size_bytes`,
`smart_info`. Il frontend lo chiede **una volta** all'apertura e lo tiene in memoria.

```bash
curl -s http://<ip-board>:9888/api/info | jq '.metrics | keys'
# ["rk3588_disk_info","rk3588_disk_size_bytes","rk3588_smart_info","rk3588_soc_info"]
```

Esempio (accorciato):

```json
{ "timestamp": 1790896000123,
  "metrics": {
    "rk3588_soc_info": [ { "labels": { "vendor": "Rockchip", "soc": "RK3588", "model": "FriendlyElec CM3588",
        "kernel": "6.1.141", "os": "Ubuntu 24.04.4 LTS", "npu_driver": "v0.9.8", "rga_driver": "v1.3.10",
        "vpu_driver": "c79104d97229 author: … 2025-08-25 video: rockchip: mpp: …", "gpu_driver": "g29p0-00eac0 (UK version 1.36)" }, "value": 1 } ],
    "rk3588_disk_info": [ { "labels": { "device": "sda", "model": "Samsung SSD 870", "vendor": "ATA",
        "bus": "sata", "rotational": "0" }, "value": 1 }, "…" ],
    "rk3588_disk_size_bytes": [ { "labels": { "device": "sda" }, "value": 2000398934016 }, "…" ],
    "rk3588_smart_info": [ { "labels": { "device": "sda", "model": "Samsung SSD 870 QVO 2TB",
        "firmware": "SVQ02B6Q" }, "value": 1 }, "…" ]
  } }
```

Nota: l'identità SMART arriva da un polling in background, quindi `smart_info` può mancare nei primi
secondi dopo l'avvio del servizio (e per sempre senza root).

### 4.3 `GET /api/stream` — aggiornamenti in tempo reale (SSE)

**Server-Sent Events**: una normale richiesta HTTP che resta aperta; il server invia un evento ogni
`RKTOP_INTERVAL` (1 s). Il browser la gestisce con `EventSource`, che **si riconnette da solo** se la
connessione cade.

- Il primo evento arriva **subito** alla connessione, poi uno ogni secondo.
- Ogni evento è una riga `data: <JSON snapshot>` seguita da una riga vuota.
- Contiene la vista **live**: tutto tranne l'identità statica e le metriche per singolo processo. Il
  conteggio `procs` (processi running/blocked) c'è.
- `?view=all` include tutto (utile per il debug).

Uso nel browser:

```js
const es = new EventSource('/api/stream');
es.onmessage = (ev) => {
  const snap = JSON.parse(ev.data);       // { timestamp, metrics }
  const cpu = snap.metrics['rk3588_cpu_usage_percent'];
  // ... aggiornare gli store e i grafici
};
es.onerror = () => { /* mostrare "offline"; EventSource riprova da solo */ };
```

Da terminale (un solo evento, leggibile):

```bash
curl -sN http://<ip-board>:9888/api/stream | grep --line-buffered '^data:' | head -1 | cut -c7- | jq .
```

**Peso**: circa 8–9 KB per evento, cioè ~65–75 kbit/s e ~700–800 MB al giorno **per ogni client
collegato**. Il flusso non è compresso.

### 4.4 `GET /api/processes` — tabella dei processi

Parametri:

| Parametro | Valori | Default |
|---|---|---|
| `sort` | `cpu`, `mem`, `pid`, `name`, `user`, `threads` | `cpu` |
| `limit` | numero di righe | `25` |
| `reverse` | `1` per invertire l'ordine | no |

Risposta:

```json
{
  "timestamp": 1790896000123,
  "sort": "mem",
  "total": 412,
  "processes": [
    { "pid": 7013, "name": "java", "user": "shad0w", "state": "S",
      "cmd": "/usr/lib/jvm/java-21-openjdk-arm64/bin/java -server …",
      "cpu": 8.7, "mem_pct": 7.25, "mem_bytes": 1211961344, "threads": 60 }
  ]
}
```

- `total` — **tutti** i processi del sistema, non solo le righe restituite (è il numero "N task").
- L'ordinamento è fatto dal server su **tutti** i processi: ordinare per memoria dà i veri più
  pesanti, non i più pesanti tra i primi per CPU. L'ordine predefinito è «il più grande per primo» per
  `cpu`, `mem` e `threads`, A→Z / il più basso per primo per `pid`, `name` e `user`; **a parità di valore
  si ordina per pid**, così le righe non si rimescolano a ogni aggiornamento (centinaia di processi stanno
  a 0 % di CPU).
- Con `reverse=1` l'ordine è invertito **prima** di tagliare a `limit`: si ottiene l'altra estremità della
  lista intera (i meno pesanti, i pid più alti), non le stesse righe capovolte. È ciò che fa il secondo clic
  su un'intestazione della tabella. La risposta riporta anche `reverse`.
- `cmd` è la riga di comando, troncata a 200 caratteri (**può contenere segreti**: vedi §7.3).
- Va chiamato **solo quando la sezione Processi è visibile**, ogni ~2 s e a ogni cambio di ordinamento.
  Il flusso `/api/stream` non porta più i processi. Nell'interfaccia desktop la tabella lo chiede solo
  finché la card è nella finestra (`IntersectionObserver`) e la scheda del browser è in primo piano: fuori
  schermo non si scandiscono i ~570 processi per niente.

### 4.5 `GET /api/stats` — tutto, per ispezione

Istantanea completa (tutte le famiglie, ~15–18 KB). Con `?view=live` o `?view=info` restituisce le
stesse parti di stream e info. Comoda per guardare i dati a mano:

```bash
curl -s http://<ip-board>:9888/api/stats | jq -r '.metrics | keys[]'
curl -s http://<ip-board>:9888/api/stats | jq -r '.metrics["rk3588_disk_temp_celsius"][] | "\(.labels.device)\t\(.value)"'
```

### 4.6 `GET /metrics` — Prometheus

Formato testuale standard di Prometheus (con `# HELP` e `# TYPE`), **tutte** le metriche. Serve a chi
vuole usare Prometheus/Grafana o una dashboard propria al posto della nostra. Per
guardare i dati a mano è la vista più descrittiva:

```bash
curl -s http://<ip-board>:9888/metrics | grep rk3588_disk
```

### 4.7 Sequenza tipica del frontend

```
 Browser                                                   rktopng (:9888)
 │                                                          │
 │ GET / ──────────────────────────────────────────────────▶│  pagina: HTML + JS + CSS incorporati nel binario
 │◀── HTML, JS, CSS ────────────────────────────────────────│
 │                                                          │
 │ GET /api/info ──────────────────────────────────────────▶│  identità statica: UNA volta all'apertura
 │◀── JSON ─────────────────────────────────────────────────│
 │                                                          │
 │ GET /api/stream ────────────────────────────────────────▶│  connessione SSE aperta (EventSource)
 │◀── data: { snapshot } ───────────────────────────────────│  subito, poi uno ogni 1 s (solo metriche live)
 │◀── data: { snapshot } ───────────────────────────────────│
 │  …                                                       │
 │                                                          │
 │  sezione Processi visibile:                              │
 │ GET /api/processes?sort=cpu&limit=20 ───────────────────▶│  ogni ~2 s e a ogni cambio di ordinamento
 │◀── JSON { total, processes[] } ──────────────────────────│
 │                                                          │
 │  finestra S.M.A.R.T. aperta (clic sul tile di un disco): │
 │ GET /api/smart/sda ─────────────────────────────────────▶│  una volta all'apertura
 │◀── JSON { health, vitals, attributes … } ────────────────│
 │                                                          │
 │  connessione persa → EventSource riprova da solo;        │
 │  la UI mostra "offline" finché non torna                 │
```

### 4.8 `GET /api/smart/<disco>` — finestra S.M.A.R.T.

```bash
curl -s localhost:9888/api/smart/nvme0n1 | jq '{state: .health.state, reasons: .health.reasons, vitals}'
```

Il disco è uno dei nomi di `/api/info` (`nvme0n1`, `sda`, …); un nome sconosciuto dà `404`. `smartctl` gira solo quando
si chiede (cache 30 s, una lettura condivisa tra richieste simultanee). Un disco che non si lascia leggere
(standby, senza permessi, adattatore USB senza S.M.A.R.T.) risponde `200` con `available: false` e un `reason`.
Fonti, regole dello stato (OK / Warning / Failing) e funzionamento con qualsiasi marca di disco: [smart_status.md](smart_status.md).

---

## 5. Catalogo delle metriche e dove compaiono nella UI

Le sezioni sono quelle dei mockup (stesso ordine su mobile e desktop). "Calcolo" indica un valore che
la UI **deriva** dalle metriche, non letto così com'è.

| Sezione UI | Elemento | Metriche | Calcolo / note |
|---|---|---|---|
| **Header** | board, SoC, kernel, OS | `soc_info` (label) | da `/api/info` |
| | uptime | `uptime_seconds` | formattato `6d 22h 50m` |
| | indicatore "live" | stato dello stream | verde se l'ultimo evento è recente |
| **Quick look** | gauge SOC | `temp_celsius{zone="soc"}` | soglie: `color_thresholds.md` |
| | gauge CPU | `cpu_usage_percent` | media degli 8 core |
| | gauge RAM | `memory_bytes{type=total,available}` | **(total − available) / total** |
| | tile ventola | `fan_pwm` | 0–255 → %; mostrare anche `n / 255` |
| **Overview** | 6 riquadri dischi | `disk_temp_celsius`, `disk_info`, `disk_size_bytes` | tipo = label `bus`; eMMC senza sensore → `n/a` |
| **CPU** | righe per core (A55 / A76 / A76) | `cpu_usage_percent{cluster,core}`, `cpu_freq_mhz` | raggruppare per cluster |
| | badge governor | `cpu_governor_info{governor}` | |
| | grafici Load per cluster | storico di `cpu_usage_percent` | buffer circolare lato UI |
| | Load breakdown | `cpu_seconds_total{mode}` | **contatore**: differenza tra due eventi → % di user / system / iowait / idle |
| **Temperature** | 7 zone | `temp_celsius{zone}` | zone: soc, bigcore0, bigcore1, littlecore, center, gpu, npu |
| **Accelerators** | NPU (3 core) | `npu_load_percent{core}`, `npu_freq_mhz` | frequenza unica per tutti i core; richiede root |
| | VPU | `vpu_load_percent{unit}`, `vpu_utilization_percent`, `vpu_freq_mhz`, `vpu_sessions` | nomi → vedi §6.6 |
| | RGA | `rga_load_percent{scheduler}`, `rga_freq_mhz` | richiede root |
| | GPU Mali | `gpu_load_percent`, `gpu_freq_mhz` | |
| | versioni driver | `soc_info{npu_driver,rga_driver,vpu_driver,gpu_driver}` | accanto al nome di ogni grafico Load, in piccolo (`v0.9.8`, `c79104d`, `v1.3.10`, `g29p0`); il testo completo nel suggerimento. VPU = commit del driver MPP (non ha un numero di versione), GPU = versione del driver Mali del kernel |
| **Memory** | Avail / Total | `memory_bytes{type=available,total}` | Total = RAM montata: il totale del kernel arrotondato per eccesso ai GB interi (15,6 GiB → "16 GB") |
| | Swap | `swap_bytes{type=total,free,used}` | |
| | DDR Ctrl | `ddr_load_percent`, `ddr_freq_mhz` | banda/frequenza del controller, **non** capacità |
| | CMA | `memory_bytes{type=cma_total,cma_used}` | |
| | breakdown | `memory_bytes{type=used,cached,buffers,free}` | stack che somma al totale; *available* è un valore a parte |
| **Storage** | volumi | `filesystem_{size,used,avail}_bytes{mount,fstype,source}` | **% = used / (used + avail)** (come `df`) |
| | badge ZFS | `zfs_pool_online{pool}`, `zfs_pool_state` | "2/2 online" |
| | tile dei dischi (nella card Disk I/O, sotto le righe) | `disk_info`, `disk_size_bytes`, `smart_info`; al clic `/api/smart/<disco>` | nome, bus, dimensione e **modello** (tre tile per riga). Il modello è quello di SMART se risponde ("Samsung SSD 870 QVO 2TB"), altrimenti quello del kernel, troncato a 16 caratteri ("Samsung SSD 870"). Il **clic apre la finestra S.M.A.R.T.** del disco (firmware, numero di serie e tutto il resto sono lì: il seriale non è esportato altrove). L'eMMC non ha S.M.A.R.T.: il tile resta uguale, non fa nulla e un tooltip dice "S.M.A.R.T. not available" |
| | Disk I/O (R+W) | `disk_busy_percent{device}`, `disk_read/write_bytes_per_second` | per disco: busy% + (R+W) |
| | Throughput (ΣR, ΣW) | `disk_read/write_bytes_per_second` | **somma per bus** (nvme / sata / emmc), usando `bus` di `disk_info` |
| | Disk temperature | `disk_temp_celsius{device,source}` | NVMe da hwmon (1 s), SATA da SMART (~60 s, a gradini) |
| **Network** | per interfaccia | `network_receive/transmit_bytes_per_second{iface}`, `network_info{iface,kind}`, `network_speed_mbps{iface}` | ▼ download / ▲ upload in un'unica unità (KB/s, MB/s o GB/s, scelta dal valore maggiore); etichetta dopo il nome: velocità del collegamento ("1 GbE"), "Wi-Fi" o "VPN". Il tipo (`kind`: ethernet/wifi/vpn/other) lo deduce l'exporter da sysfs (directory `wireless`, tipo 65534 o `tun_flags` per i tunnel, `device` per l'hardware vero), non dal nome; la velocità c'è solo per i cavi col collegamento attivo |
| **Processes** | tabella | `/api/processes` | badge "N task" = `total` |

Metriche esposte ma che l'interfaccia non usa: `usb_pd_*` e `typec_port_info` (la parte USB-C),
`input_voltage_volts`, `load_average`, `tcp_connections`, `context_switches_total`, `smart_healthy`,
`smart_power_on_hours`, `zfs_pool_state`. Sono in `/metrics` e in `/api/stats` per chi le vuole.

---

## 6. Il frontend

### 6.1 Stato

- I **mockup** HTML (`mockups/rktopng-mobile.html` e `mockups/rktopng-desktop.html`, con dati simulati) sono la
  specifica visiva: definiscono layout, componenti, soglie colore e significato dei numeri.
- Il **frontend** in `frontend/` (Svelte 5 + Vite + TypeScript) ha uno strato dati completo e testato, il build
  incorporato nel binario Go e la selezione mobile/desktop.
- Le **sezioni dell'interfaccia**: barra in alto e **Overview** (gauge SoC/CPU/RAM, ventola, temperature dei
  dischi), **CPU**, **Temperature**, **Accelerators**, **Memory**, **Storage**, **Network** e **Processes**. Sia
  l'interfaccia **desktop** sia quella **mobile** le mostrano; le card sono le stesse (cartella `ui/sections`),
  cambia il contorno: sul desktop una griglia con tutte le sezioni, sul telefono i pulsanti di sezione sotto le
  gauge e le card una sotto l'altra.
- La finestra **S.M.A.R.T.** del desktop (`ui/smart/`): si apre dal tile di un disco nella card Disk I/O, legge
  `/api/smart/<disco>` **una volta all'apertura** e mostra stato di salute (OK / Warning / Failing) con i motivi,
  otto cifre, grafico della temperatura delle ultime ore, identità, log del disco e tabella completa. Regole e
  fonti: [smart_status.md](smart_status.md). Sul mobile i tile dei dischi non sono cliccabili, per scelta: su uno schermo piccolo la finestra non serve.

### 6.2 Schema del flusso dei dati

```
   EventSource /api/stream ─────▶ store  live     ← ultimo snapshot, indicizzato per nome metrica
        (ogni 1 s)           └──▶ buffer storico   ← ring buffer ~40 campioni per ogni grafico
                                                      (solo in memoria, nessuna persistenza)
   GET /api/info (1 volta) ─────▶ store  info     ← identità (join con live: bus del disco, modello…)
   GET /api/processes ──────────▶ store  procs    ← solo se la sezione è visibile, ogni ~2 s
   GET /api/smart/<disco> ──────▶ store  smart    ← una volta, quando si apre la finestra di un disco
                                       │
                                       ▼
                       derivazioni (calcolate nello store, non nei componenti)
                       · CPU % = media dei core        · RAM % = (total − available) / total
                       · FS %  = used / (used + avail) · Fan % = pwm / 255
                       · Load breakdown = Δ cpu_seconds_total per modo
                       · Throughput per bus = Σ read/write dei dischi di quel bus
                                       │
                                       ▼
        componenti: gauge · tile · righe con barra · grafici canvas · tabella processi · finestra S.M.A.R.T.
        ──────────  due interfacce separate: MOBILE (card, tab per sezione) · DESKTOP (griglia, tutto insieme)
```

### 6.3 Stack e struttura del progetto

- **Svelte 5** (rune) + **Vite** + **TypeScript**; test con **Vitest**. I font IBM Plex sono incorporati
  (solo woff2): l'app non dipende da Internet.
- **Grafici**: disegnati a mano su canvas, senza librerie (codice portato dai mockup: scale fisse, sfumatura a
  sinistra, legende nella banda superiore). Il grafico della temperatura nella finestra S.M.A.R.T. è un SVG.
- Storico **solo in memoria** (buffer circolari di 40 campioni): nessun database, nessuna persistenza.
- **Lingua dell'interfaccia**: tutti i testi mostrati (etichette, tooltip, messaggi) sono **in inglese**; la documentazione è in italiano.
- **Build**: Vite scrive direttamente in `exporter/web/dist`; il backend Go lo **incorpora nel binario**
  (`go:embed`, pacchetto `exporter/web`) e lo serve su `/` con cache lunga per i file con hash, gzip per
  il testo e ritorno a `index.html` per i percorsi dell'app. Un unico file da installare, stessa origine
  dell'API (nessun CORS), nessun Node a runtime.
- **Sviluppo**: `make dev RKTOP_API=http://<board>:9888` avvia Vite con ricarica a caldo e inoltra `/api`
  a un backend vero.

```
 frontend/src/
 ├── main.ts, app.css          avvio; token di design (colori, font) condivisi da mobile e desktop
 ├── App.svelte                sceglie l'interfaccia e avvia la connessione (una sola per tutta l'app)
 ├── lib/
 │   ├── api/                  types.ts (forma del JSON, anche di /api/smart) · client.ts (fetch + EventSource)
 │   ├── data/                 funzioni PURE, tutte testate:
 │   │   ├── metrics.ts          nomi delle metriche, ricerca per label, chiavi delle serie
 │   │   ├── derive.ts           CPU %, RAM %, % filesystem come df, ripartizione CPU, throughput per bus…
 │   │   ├── inventory.ts        identità di board e dischi (da /api/info)
 │   │   ├── history.ts          storico in memoria (buffer circolari)  ·  ringbuffer.ts
 │   │   ├── thresholds.ts       colori e soglie (come docs/color_thresholds.md)
 │   │   ├── format.ts           "10 MB/s", "1.8T", "6d 22h 50m"…
 │   │   ├── smart.ts            la finestra S.M.A.R.T.: carte, tabella, identità, log, grafico, testi
 │   │   └── names.ts            nomi driver → nomi mostrati (§6.6)
 │   ├── state/                store reattivi (rune): stores.svelte.ts (info, live, connessione,
 │   │                         processi, finestra S.M.A.R.T.) · dashboard.svelte.ts (collega tutto) · ui.svelte.ts (mobile/desktop)
 │   ├── charts/               canvas.ts (dimensiona la tela) · theme.ts (colori dei token) · draw.ts
 │   │                         (i disegni dei mockup: ogni funzione riceve contesto, misure e campioni)
 │   └── test/                 fixtures.ts (snapshot di prova con valori reali della board) · smart-fixtures.ts (risposte di /api/smart)
 └── ui/                       Mobile e Desktop (le due interfacce) · layout.ts (quale delle due sta disegnando)
     ├── parts/                pezzi condivisi: Card · Badge · Stat (tile) · Gauge · BarRow/RowGroup (righe con
     │                         barra, gruppo espandibile con "+") · TileGrid · ChartTile (tile con legenda) · AreaChart · Spark (canvas che si
     │                         ridisegna a ogni push e al ridimensionamento) · SectionHead · Brand · HostInfo · LiveBadge
     ├── sections/             le card di ogni sezione, condivise: Cpu, Temperature, Accelerators, Memory, Storage, Network, Processes
     ├── desktop/              solo desktop: TopBar, Overview (gauge + riquadri dei dischi affiancati)
     ├── smart/                solo desktop: la finestra S.M.A.R.T. (SmartWindow, SmartCards, SmartTempChart, SmartTable)
     └── mobile/               solo mobile: TabBar (i pulsanti di sezione)
```

Regola: i **calcoli stanno in `lib/data`** (funzioni pure sui snapshot), gli **store** tengono lo stato
ed eseguono l'I/O, i **componenti** mostrano soltanto.

Regola degli indirizzi: **mai un indirizzo che inizia con `/`** (`api/info`, non `/api/info`; `favicon.png`, non
`/favicon.png`). La dashboard può stare sotto un sottopercorso dietro un reverse proxy (§7.4) e solo gli
indirizzi relativi la seguono; Vite è configurato con `base: './'`.

### 6.4 Due interfacce: mobile e desktop

Sono due UI dedicate, non un layout responsivo unico. Lo stesso backend; la UI giusta si sceglie lato
client in base a viewport e touch (≤ 760 px o tablet in verticale → mobile), con un interruttore manuale.

Le sezioni (`ui/sections`) non sanno su quale schermo sono: chiedono a `useLayout()` (`ui/layout.ts`) se sono nel
desktop o nel mobile solo dove serve (griglie a una o due colonne, grafici Load in colonna, card dei processi al
posto della tabella). Così un'interfaccia può essere forzata su uno schermo dell'altro tipo, e le finestre strette
del desktop restano utilizzabili.

| | Mobile | Desktop |
|---|---|---|
| Navigazione | barra di icone, **una sezione alla volta** | tutte le sezioni insieme, separate da una riga con l'icona |
| Layout | colonna di card (~340 px); gauge e ventola sempre in vista sotto l'intestazione | griglia a 12 colonne (max 1280 px), card della stessa riga alte uguali |
| Griglie di riquadri (Thermal zones, Disk temperature) | 2 colonne, tante righe quanti sono i riquadri | 4 colonne (3 per i dischi); 2 su finestre più strette |
| Processi | card con pulsanti "ordina per" (secondo tocco inverte, con freccia); 10 righe | tabella con intestazioni cliccabili (secondo clic inverte); 15 righe |
| Dischi (Storage) | tile non cliccabili **nella card Storage**: nome, tipo, dimensione e modello (due per riga) | tile **pulsanti** nella card Disk I/O: nome, tipo, dimensione e modello (tre per riga); il clic apre la finestra S.M.A.R.T. |
| Ordine delle card di Storage | Storage, Disk I/O, Throughput, Disk temperature | Storage, Disk temperature, Disk I/O, Throughput |

### 6.5 Soglie colore

Quattro colori semantici (good / warn / orange / crit) con soglie per CPU %, RAM %, temperatura e
capacità disco. Tabella completa in [`color_thresholds.md`](color_thresholds.md).

### 6.6 Nomi: backend → interfaccia

Il backend usa i nomi dei driver; la UI mostra nomi brevi, con una piccola tabella di traduzione
(`lib/data/names.ts`):

| Backend (`unit` / `scheduler`) | UI |
|---|---|
| `enc_core0`, `enc_core1` | `enc0`, `enc1` |
| `dec_core0`, `dec_core1` | `dec0`, `dec1` |
| `jpeg_enc0…3`, `jpeg_decoder`, `av1_decoder`, `vdpu`, `avs_decoder`, `iep` | `jpeg0…3`, `jpegd`, `av1d`, `vdpu`, `avsd`, `iep` (engine "extra", nascosti dietro il "+") |
| `rga3_0`, `rga3_1`, `rga2_2` | `rga3·0`, `rga3·1`, `rga2` |
| zona `bigcore0`, `bigcore1`, `littlecore` | `big_0`, `big_1`, `little` |

### 6.7 Dati mancanti e errori

- Metrica assente → mostrare **`n/a`** (es. NPU senza root, SMART senza `smartctl`, eMMC senza sensore).
- `usb_pd_online = 0` significa "nessun contratto PD", **non** "0 V".
- L'indicatore di connessione ha quattro stati: `live` (arrivano dati), `connecting`, `stale` (la connessione è
  aperta ma non arriva un evento da più di 4 secondi) e `offline` (connessione caduta). L'`EventSource` si
  riconnette da solo; alla riconnessione il primo evento ripristina lo stato.
- Il primo evento dopo l'avvio non ha le metriche "a differenza" (throughput, rete): l'assenza vale "non ancora
  disponibile".
- Un disco che non si lascia leggere dalla finestra S.M.A.R.T. (standby, senza permessi) mostra il motivo con un
  pulsante "Try again"; le cifre che un disco non riporta mostrano `n/a`.

---

## 7. Installazione: Docker o nativa

Lo stesso programma, due modi di eseguirlo.

```
   NATIVO (systemd)                                    DOCKER
   ────────────────                                    ──────
   systemd ─▶ /usr/local/bin/rktopng                   container "privileged"
              │                                         ├─ /host/proc        ◀─ /proc dell'host
              ├─ legge /proc e /sys direttamente        ├─ /host/sys         ◀─ /sys dell'host
              ├─ esegue smartctl (per SMART)            ├─ /host/root        ◀─ / dell'host (statfs)
              ├─ ascolta sulla porta 9888               ├─ /dev              ◀─ dispositivi dell'host
              └─ config: /etc/rktopng/rktopng.env       ├─ /host/os-release, /host/passwd
                                                        └─ RKTOP_*_PATH puntano a questi mount
```

### 7.1 Nativa (senza Docker)

```bash
make build-arm64     # → dist/rktopng-linux-arm64   (make build-amd64 per x86-64)
scp dist/rktopng-linux-arm64 deploy/native/{install.sh,rktopng.service,rktopng.env.example} utente@board:~/rktopng-install/
ssh -t utente@board 'cd ~/rktopng-install && sudo ./install.sh ./rktopng-linux-arm64'
```

Installa il binario in `/usr/local/bin/rktopng`, crea `/etc/rktopng/rktopng.env` (mai sovrascritto negli
aggiornamenti) e abilita il servizio `rktopng` (come root: debugfs e SMART). Per SMART serve il pacchetto
`smartmontools`. Stato e log: `systemctl status rktopng`, `journalctl -u rktopng -f`. Rimozione:
`sudo ./install.sh --uninstall`. Per **aggiornare**: ricostruire e rilanciare lo stesso `install.sh`.

### 7.2 Docker

```bash
docker compose up -d --build exporter
```

`docker-compose.yml` monta `/proc`, `/sys` e `/` dell'host, imposta le variabili `RKTOP_*_PATH` e usa
`privileged: true` (serve per SMART e debugfs). Il `Dockerfile` (alla radice del repository, che è il
contesto di build) ha tre fasi: **Node** compila l'interfaccia, **Go** compila il backend incorporandola,
**Alpine** (con `smartmontools`) esegue il binario. Con l'interfaccia dentro, l'app è su
`http://<board>:9888/`.

### 7.3 Sicurezza

- **Per impostazione predefinita non c'è nessuna autenticazione** e il server ascolta su tutte le interfacce.
  Pensato per LAN o Tailscale: non esporlo su Internet. Per chiedere una password vedi §7.4.
- `/api/processes` e `/metrics` (`rk3588_process_info`) restituiscono le **righe di comando** dei processi
  (fino a 200 caratteri), che possono contenere password o token passati come argomenti: per questo la password,
  quando c'è, protegge anche `/metrics`.
- Il **numero di serie** (e il WWN) dei dischi si legge solo da `/api/smart/<disco>`: non è in `/metrics` né in `/api/info`.
- Il servizio gira come **root** (necessario per debugfs e SMART). Per usarlo senza root, cambiare
  `User=` nell'unit systemd: SMART e carico NPU/RGA non saranno disponibili.

### 7.4 Dietro un reverse proxy (sottopercorso) e con la password

**Sottopercorso.** Con `RKTOP_BASE_PATH=/rktopng` il servizio risponde solo sotto `/rktopng/` (dashboard,
`/rktopng/api/*`, `/rktopng/metrics`, `/rktopng/exporter`). `/rktopng` senza barra finale viene
rimandato a `/rktopng/`; la porta aperta direttamente (`/`) rimanda al prefisso; tutto il resto è 404.
È lo stesso schema di Scrutiny (`SCRUTINY_WEB_LISTEN_BASEPATH`) e di rkmon (`GOTTY_PATH`): il proxy
**non toglie** il prefisso, lo sa il servizio.

Come fa la dashboard a funzionare sotto qualsiasi prefisso senza ricompilare: **tutti i suoi indirizzi sono
relativi** (`api/info`, `api/stream`, `./assets/…`, `favicon.png`), quindi si appoggiano all'indirizzo della
pagina, che qui finisce con `/`. Per questo:

- nel frontend **nessun indirizzo deve iniziare con `/`**: lo controlla un test (`relative-urls.test.ts`);
- il server **non risponde con `index.html` a un percorso sconosciuto** (la dashboard non ha percorsi propri):
  un indirizzo come `/a/b` darebbe una pagina i cui file si cercano sotto `/a/`;
- l'indirizzo di prova va scritto **senza credenziali** (`https://host/rktopng/`, non
  `https://utente:password@host/rktopng/`): gli indirizzi relativi ereditano le credenziali e `fetch` li rifiuta
  (`Request cannot be constructed from a URL that includes credentials`). Dopo aver inserito la password nella
  finestra del browser l'indirizzo resta pulito e tutto funziona.

Esempio per Nginx Proxy Manager (una *custom location* nello stesso host di Scrutiny e rkmon):

```
Location        /rktopng
Forward         http://<ip-board>:9888      ← senza barra e senza percorso finale: arriva /rktopng/... invariato
```

(è il `proxy_pass http://<ip>:<porta>;` senza URI che NPM scrive già per `/scrutiny` e `/rkmon`).

**Flusso live (`/api/stream`).** nginx può trattenere le risposte prima di inoltrarle. Misurato sulla board con le
stesse direttive di NPM (OpenResty 1.27): gli eventi dell'exporter, che scrive in più pezzi, passano subito anche
senza accorgimenti, mentre un evento scritto in un colpo solo arriva con un evento (1 s) di ritardo. Poiché
il comportamento dipende da come arrivano i byte, la risposta dichiara comunque `X-Accel-Buffering: no`
(nginx la rispetta per quella risposta) e il flusso non dipende più da questo dettaglio.

**Password.** Con `RKTOP_AUTH_USER` e `RKTOP_AUTH_PASSWORD` **ogni** richiesta chiede utente e password (HTTP
Basic): dashboard, `/api/*`, **`/metrics`**, `/debug/pprof/`, anche un 404. Non c'è nessuna eccezione. Il confronto è a tempo
costante. Poi:

- senza le due variabili non cambia nulla (tutto aperto, come prima); con una sola il servizio **non parte**;
- Basic manda le credenziali in chiaro (base64): usare HTTPS (NPM) o una rete fidata come Tailscale;
- Prometheus deve mandare le credenziali: `basic_auth` nello `scrape_config`; con il prefisso, `metrics_path: /rktopng/metrics`;
- nell'installazione nativa la password sta in `/etc/rktopng/rktopng.env`, che `install.sh` rende leggibile solo da root.

---

## 8. Sviluppo

### 8.1 Struttura del repository

```
 CM3588_RKTop/
 ├── exporter/                  backend Go
 │   ├── main.go                avvio e configurazione, registrazione dei collector
 │   ├── server.go              rotte HTTP, sottopercorso (RKTOP_BASE_PATH) e password (RKTOP_AUTH_*)
 │   ├── stats.go               viste JSON (all/live/info), SSE, /api/processes
 │   ├── stats_test.go
 │   ├── collectors/            un file per sorgente di dati (+ *_test.go)
 │   └── web/                   serve il frontend incorporato (go:embed); dist/ è generato da Vite
 ├── frontend/                  interfaccia Svelte + Vite + TypeScript (vedi §6.3)
 ├── mockups/                   mockup HTML approvati (mobile, desktop)
 ├── deploy/native/             unit systemd, install.sh, esempio di configurazione
 ├── tools/accel-loadtest/      carico su VPU/RGA/NPU e verifica dei numeri (da eseguire sulla board)
 ├── docs/                      questa documentazione, data_sources.md, smart_status.md, color_thresholds.md
 ├── Dockerfile                 immagine con interfaccia + backend (3 fasi)
 ├── docker-compose.yml         il servizio exporter (interfaccia + backend)
 ├── Makefile                   test, vet, frontend, dev, build-arm64, build-amd64
 └── README.md
```

### 8.2 Comandi

```bash
make test            # test unitari: backend Go + frontend (Vitest)
make vet             # go vet + svelte-check (tipi)
make frontend        # compila l'interfaccia in exporter/web/dist
make dev RKTOP_API=http://<ip-board>:9888   # interfaccia con ricarica a caldo, dati da una board vera
make build-arm64     # UN binario statico con l'interfaccia dentro → dist/rktopng-linux-arm64
```

Servono Go 1.21 o più recente e Node 22 o più recente (solo per compilare l'interfaccia), entrambi
installabili con Homebrew. `exporter/web/dist` è **generata** e non sta nel repository: `make test` e `make vet`
creano una pagina segnaposto se manca (serve a tenere valido `go:embed`), mentre `make frontend` la sostituisce
con l'app vera. Una compilazione Go "a mano" su un clone nuovo richiede prima `make frontend`.

### 8.3 Aggiungere una metrica

1. Se la sorgente è nuova, descriverla in [`data_sources.md`](data_sources.md) e **verificarla sulla
   board** (lettura da utente normale e da root).
2. Scrivere un collector in `exporter/collectors/` (`Describe` + `Collect`), con prefisso `rk3588_`.
3. Registrarlo in `main.go`.
4. Scrivere i test con un falso `/proc` e `/sys` (vedi `helpers_test.go`: `fakeRoots`, `write`, `link`).
5. Se la metrica **non cambia mai**, aggiungerla a `staticFamilies` in `stats.go`; se è per singolo
   processo, usare il prefisso `rk3588_process_` (resta fuori dal flusso live).

### 8.4 Provare sulla board senza `sudo`

- **Come utente normale**: copiare il binario, avviarlo su una porta libera (es. `RKTOP_PORT=9889`) e
  leggere `/metrics`. SMART e debugfs risulteranno assenti (i log lo dicono).
- **Con i dati root**: costruire l'immagine sulla board e avviarla `--privileged` (l'utente è nel gruppo
  `docker`, quindi equivale a root). Ricordarsi di fermare il processo/container di prova alla fine.

---

## 9. Decisioni di progetto e limiti noti

- **Tipo di DDR (LPDDR4X/5) non leggibile a runtime**: lo imposta il firmware prima del kernel. La UI
  mostra solo la dimensione (`16 GB`), senza il tipo.
- **RAM "usata" = (total − available) / total**, come btop/htop, non (total − free) che conterebbe
  anche la cache.
- **Disk I/O è per disco; Throughput è la somma per bus.** Sono I/O **fisici**: su un RAIDZ1 include la
  parità, su un mirror le scritture contano due volte. Il throughput logico per pool non è disponibile
  (`/proc/spl/kstat/zfs/<pool>/io` non esiste su questo ZFS).
- **Etichetta dei gruppi di volumi**: ZFS per i pool; un volume su una partizione `/dev/…` prende il bus del suo disco (NVMe, SATA, eMMC); la root `/` dei FriendlyElec è un *overlay* il cui disco reale non è visibile dal sistema in esecuzione, e viene etichettata **eMMC** (convenzione di queste schede, valida se c'è un eMMC; altrimenti `OVERLAY`). Logica in `lib/data/storage.ts`.
- **Temperatura SATA solo via SMART** (il kernel non ha `drivetemp`): aggiornata ogni ~60 s, a gradini.
  L'eMMC non espone nessun sensore.
- **Stream senza compressione**: circa 8–9 KB per evento, ~65–75 kbit/s per client.
- **Più client dello stream non moltiplicano il lavoro**: i collector live sono condivisi (cache di ~700 ms),
  quindi con più schede aperte si paga poco più di una sola (~2-3% di un core). Ogni client serializza e invia
  comunque il suo JSON.
- **Storico**: nessuno nel backend. Le sparkline vivono nella memoria del browser e si azzerano a ogni
  ricarica. Chi vuole lo storico lungo punta un proprio Prometheus a `/metrics`.
- **Acceleratori verificati sotto carico (2026-10-02)**: con ffmpeg (decodifica + scalatura RGA + codifica) e
  con matrici INT8 sull'NPU, i valori dell'exporter coincidono secondo per secondo con i file del kernel
  (NPU e RGA da debugfs, VPU da `/proc/mpp_service/load`); `vpu_sessions` vale 2 con un decoder e un encoder
  aperti e torna a 0 a fine lavoro. Lo strumento è in `tools/accel-loadtest/`.
- **S.M.A.R.T.**: la finestra è di sola lettura (nessun self-test avviabile) e esiste solo nell'interfaccia
  desktop, per scelta (su uno schermo piccolo sarebbe eccessiva). Lo stato ha tre livelli con regole documentate. Le cifre vengono dai registri standard prima che dagli
  attributi del produttore, così funziona con qualsiasi marca di disco (vedi [smart_status.md](smart_status.md)).
  Il numero di serie e il WWN si leggono solo da `/api/smart/<disco>` (vedi §7.3).
- **Cosa l'interfaccia non mostra**: la parte USB-C (`usb_pd_*`, `typec_port_info`), la tensione di ingresso, la
  salute dell'eMMC (non ha S.M.A.R.T.) e i clock dei decoder VPU (non hanno un nodo devfreq e l'exporter non li legge).

---

## 10. Documenti correlati

- [`../README.md`](../README.md) — panoramica, metriche, installazione, endpoint (riassunto).
- [`data_sources.md`](data_sources.md) — per ogni dato: file di sistema, come leggerlo, se serve root,
  verifica sulla board.
- [`color_thresholds.md`](color_thresholds.md) — colori e soglie di gauge, barre e temperature.
- [`smart_status.md`](smart_status.md) — finestra S.M.A.R.T.: fonti dei dati, regole dello stato di salute e funzionamento con qualsiasi marca di disco.
- `mockups/` — i mockup approvati (la specifica visiva del frontend).
- `deploy/native/` — file per l'installazione senza Docker.
