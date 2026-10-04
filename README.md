# RkTopNG

Web dashboard di monitoraggio per board **Rockchip RK3588** (FriendlyElec **CM3588**).
Stile `btop`, ma con metriche specifiche RK3588 (NPU, RGA, GPU Mali, VPU enc/dec) e
sensori NAS (temp NVMe, tensione 12V, USB-C PD, ventola).

> **Documentazione completa del funzionamento** (schemi di backend e frontend, uso di ogni endpoint,
> catalogo delle metriche, installazione): [`docs/FUNZIONAMENTO.md`](docs/FUNZIONAMENTO.md).

## Architettura

Un solo programma, **l'exporter** (Go, un binario statico, porta 9888), che legge `/proc`, `/sys`, debugfs e `smartctl`
e serve sia i dati sia la dashboard:

```
┌──────────────────────── CM3588 (RK3588) ─────────────────────────┐
│  exporter (Go, :9888)  ─ legge /proc · /sys · debugfs · smartctl │
│    ├─ /                 la dashboard (Svelte) incorporata        │
│    └─ /metrics · /api/* i dati, per chiunque li voglia usare     │
└──────────────────────────────────────────────────────────────────┘
          ▲                                  ▲
   browser: la nostra dashboard      la TUA dashboard, Prometheus, script…
```

- **exporter** — legge sysfs/procfs/debugfs. Vedi `docs/data_sources.md` per la mappatura completa
  metrica → path (verificata sulla board).
- **dashboard** — interfaccia Svelte (mobile e desktop) incorporata nel binario: è soltanto un client degli endpoint
  qui sotto.
- **L'exporter non dipende dalla nostra interfaccia.** Chi vuole può costruire la propria dashboard (Grafana, un'altra
  web app, uno script) su `/metrics` (formato Prometheus, tutto) o sui JSON `/api/*`, senza usare la nostra.

## Metriche esposte (prefisso `rk3588_`)

`cpu_usage_percent`, `cpu_freq_mhz`, `temp_celsius`, `gpu_load_percent`, `gpu_freq_mhz`,
`npu_load_percent`†, `npu_freq_mhz`, `rga_load_percent`†, `rga_freq_mhz`†,
`vpu_load_percent`, `vpu_utilization_percent`, `memory_bytes`, `swap_bytes`,
`nvme_temp_celsius`, `input_voltage_volts`, `usb_pd_voltage_volts`, `usb_pd_current_amps`,
`fan_pwm`, `fan_rpm`, `uptime_seconds`, `load_average`, `network_*_bytes_per_second`,
`tcp_connections`, `soc_info` (include il nome dell'OS).

Storage e dischi:

| Metrica | Contenuto |
|---|---|
| `filesystem_size_bytes` / `_used_bytes` / `_avail_bytes` | volumi montati reali (label `mount`, `fstype`, `source`); la % come `df` è used/(used+avail) |
| `zfs_pool_online`, `zfs_pool_state` | salute dei pool ZFS (da `/proc/spl/kstat/zfs/*/state`) |
| `disk_info`, `disk_size_bytes` | inventario dischi: modello, bus (nvme/sata/usb/emmc…), dimensione |
| `disk_read_bytes_per_second`, `disk_write_bytes_per_second`, `disk_busy_percent` | I/O per disco |
| `disk_temp_celsius` | una temperatura per disco (label `source`: `hwmon` per gli NVMe, `smart` per gli altri) |
| `smart_up`, `smart_temperature_celsius`, `smart_healthy`, `smart_power_on_hours`, `smart_info`† | dati SMART, letti in background ogni 60 s |

USB-C: `usb_pd_online`, `usb_pd_type`, `typec_port_info` (ruoli della porta, partner collegato).

† Richiedono **root**: debugfs (NPU/RGA load, versioni driver) e SMART (serve anche il comando
`smartctl`, pacchetto `smartmontools`; l'immagine Docker lo include). Senza root o senza
`smartctl` l'exporter funziona ugualmente e semplicemente non emette quei dati. Nel container
servono `privileged: true` e il mount di `/dev` (già nel `docker-compose.yml`).

## Endpoint HTTP (porta 9888)

| Endpoint | Contenuto |
|---|---|
| `/` | **la dashboard** (interfaccia incorporata nel binario). `/exporter` è una pagina con i link agli endpoint |
| `/metrics` | formato Prometheus, **tutto** (anche i processi). Per Prometheus/Grafana o una dashboard propria |
| `/api/info` | JSON con l'identità **statica** di board e dischi (`soc_info`, `disk_info`, `disk_size_bytes`, `smart_info`): la UI lo chiede **una volta** all'apertura. L'identità SMART arriva da un polling in background, quindi può mancare nei primissimi secondi dopo l'avvio |
| `/api/stream` | Server-Sent Events: una spinta al secondo (`RKTOP_INTERVAL`) con le sole metriche **che cambiano** (niente identità statica, niente metriche per-processo; il conteggio `procs` c'è). ~8-9 KB per push, valori a 2 decimali |
| `/api/processes?sort=cpu\|mem\|pid\|name\|user\|threads&limit=N` | lista processi ordinabile + `total` (tutti i processi, per il badge "N task"); da chiamare solo quando la sezione è visibile |
| `/api/smart/<disco>` | report S.M.A.R.T. completo di un disco (stato OK/Warning/Failing, cifre, attributi o log NVMe, log del disco), letto **su richiesta**; funziona con qualsiasi marca, vedi [docs/smart_status.md](docs/smart_status.md) |
| `/api/stats` | istantanea JSON di **tutto** (comoda per ispezionare i dati); `?view=live` o `?view=info` la restringono come stream e info |

`/api/stream` accetta anche `?view=all` per includere tutto (debug).

## Deploy (sulla board CM3588)

Prerequisito: **Docker** + plugin compose.
```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER   # poi ri-login
```

Avvio (dalla cartella del progetto):
```bash
docker compose up -d --build
```

La dashboard è su `http://<board>:9888/` (con l'indirizzo Tailscale o LAN della board).

Stop / log:
```bash
docker compose logs -f exporter
docker compose down
```

## Dietro un reverse proxy (sottopercorso) e con la password

Entrambe le cose sono **facoltative** e spente per impostazione predefinita.

| Variabile | Effetto |
|---|---|
| `RKTOP_BASE_PATH=/rktopng` | serve tutto sotto `/rktopng/` (come Scrutiny e rkmon): il proxy lascia il prefisso nell'indirizzo |
| `RKTOP_AUTH_USER` + `RKTOP_AUTH_PASSWORD` | **ogni** richiesta chiede utente e password (HTTP Basic), `/metrics` compreso. Vanno impostate tutte e due |

Nginx Proxy Manager: una *custom location* `/rktopng` che punta a `http://<ip-board>:9888` (senza percorso dopo la porta).
La dashboard si apre su `https://<host>/rktopng/`. Prometheus, con la password: `basic_auth` nello `scrape_config`
e `metrics_path: /rktopng/metrics`. Dettagli e motivazioni: [`docs/FUNZIONAMENTO.md` §7.4](docs/FUNZIONAMENTO.md).

## Installazione senza Docker (nativa)

L'app (backend + interfaccia) è un singolo binario statico: non serve Docker. Su una macchina con Go e
Node (Node serve solo a compilare l'interfaccia):

```bash
make build-arm64            # → dist/rktopng-linux-arm64
scp dist/rktopng-linux-arm64 deploy/native/{install.sh,rktopng.service,rktopng.env.example} utente@board:~/
ssh utente@board 'sudo apt install -y smartmontools && sudo ./install.sh ./rktopng-linux-arm64'
```

`install.sh` copia il binario in `/usr/local/bin/rktopng`, crea `/etc/rktopng/rktopng.env`
(configurazione, mai sovrascritta negli upgrade) e abilita il servizio systemd `rktopng`,
che gira come root (debugfs e SMART). Stato e log: `systemctl status rktopng`,
`journalctl -u rktopng -f`. Rimozione: `sudo ./install.sh --uninstall`.

Le variabili `RKTOP_*` (porta, intervallo, SMART…) sono documentate in
`deploy/native/rktopng.env.example`; i prefissi `RKTOP_PROC_PATH` & co. servono solo nel container.

## Sviluppo

```bash
make test        # test: backend Go + frontend (Vitest)
make vet         # go vet + controllo dei tipi (svelte-check)
make dev RKTOP_API=http://<ip-board>:9888   # interfaccia con ricarica a caldo, dati da una board vera
make build-arm64 # un solo binario con l'interfaccia dentro → dist/
```

Per pubblicare una versione (tag + release GitHub con i binari e le somme di controllo): `make release VERSION=0.1.1`
e `make release-publish VERSION=0.1.1`, vedi [`docs/FUNZIONAMENTO.md` §8.5](docs/FUNZIONAMENTO.md).

Il frontend (Svelte + Vite + TypeScript) è in `frontend/`; il build finisce in `exporter/web/dist` e il
backend lo incorpora (`go:embed`). Dettagli in [`docs/FUNZIONAMENTO.md`](docs/FUNZIONAMENTO.md#6-il-frontend).

## Verifica veloce dell'exporter

```bash
docker compose up -d --build
curl -s localhost:9888/metrics | grep '^rk3588_' | head
```

## Note

- `RKTOP_PROC_PATH` / `RKTOP_SYS_PATH` / `RKTOP_ROOTFS_PATH`: prefissi dei filesystem host
  montati nel container (vedi `docker-compose.yml`).
- Storico: l'exporter non ne conserva (l'interfaccia tiene una cinquantina di campioni in memoria, nel browser). Per uno
  storico lungo punta un tuo Prometheus a `/metrics`.

## Licenza

MIT, vedi [LICENSE](LICENSE).
