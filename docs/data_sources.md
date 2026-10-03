# CM3588 RKTop — Sorgenti dati (verificate)

Mappatura delle metriche da visualizzare nella dashboard alle relative sorgenti
del kernel Linux, **verificate direttamente sulla board CM3588** via SSH.

## Board di riferimento

| | |
|---|---|
| Modello | FriendlyElec **CM3588** (NAS Kit) |
| SoC | Rockchip **RK3588** |
| Kernel | `6.1.141` aarch64 (BSP FriendlyElec) |
| OS | Ubuntu 24 (release ufficiale FriendlyElec) |
| RAM | 16 GB, **nessuno swap** configurato |
| CPU | 8 core: cpu0–3 = Cortex-**A55** (`0xd05`, LITTLE), cpu4–7 = Cortex-**A76** (`0xd0b`, big) |

> ⚠️ Su questa board i core **LITTLE sono cpu0–3** e i **big sono cpu4–7** — l'opposto
> di quanto scritto nel README di `ajokela/rktop` (riferito a un Orange Pi). Non assumere
> l'ordinamento: derivarlo da `/proc/cpuinfo` (`CPU part`).

## Legenda permessi

- 🟢 **user** — leggibile senza privilegi (utente normale `shad0w`)
- 🔴 **root** — richiede root: vive in `debugfs` (`/sys/kernel/debug`, montato `drwx------ root`)

Stato verifica: tutti i path sotto sono stati testati sulla board. I formati di esempio
sono output reali catturati il giorno della verifica.

---

## CPU

| Metrica | Path | Perm | Formato / esempio | Note parsing |
|---|---|---|---|---|
| Carico per-core | `/proc/stat` (righe `cpu0`…`cpu7`) | 🟢 | `cpu0 891 0 382 3570 41 0 46 0 0 0` | Contatori **cumulativi**: `load = (Δtotal − Δidle) / Δtotal`. Servono 2 letture. Campi: user nice system idle iowait irq softirq steal… `idle` reale = idle + iowait |
| Carico aggregato + breakdown | `/proc/stat` riga `cpu ` | 🟢 | idem | Per percentuali user/system/iowait/idle globali |
| Frequenza per-core | `/sys/devices/system/cpu/cpu{N}/cpufreq/scaling_cur_freq` | 🟢 | `1800000` | In **kHz** → `/1000` = MHz |
| Range freq (min/max) | `.../cpu{N}/cpufreq/scaling_{min,max}_freq` | 🟢 | kHz | Raggruppare i valori unici per identificare i cluster big.LITTLE |
| Governor | `/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor` | 🟢 | `ondemand` | |
| Tipo core (A55/A76) | `/proc/cpuinfo` campo `CPU part` | 🟢 | `0xd05` / `0xd0b` | `0xd05`→A55, `0xd0b`→A76 |
| ctxt / intr / softirq | `/proc/stat` righe `ctxt`, `intr`, `softirq` | 🟢 | `ctxt 471681` | Cumulativi → rate/s dal delta |
| Processi running/blocked | `/proc/stat` righe `procs_running`, `procs_blocked` | 🟢 | `procs_running 8` | Istantanei |

---

## Temperature

Sette zone termiche, tutte 🟢 **leggibili senza root** via sysfs. Valore in **milli-°C**
(`/1000` = °C).

| Zona | `type` (thermal) | `name` (hwmon) | Path temp |
|---|---|---|---|
| SoC | `soc-thermal` | `soc_thermal` (hwmon0) | `/sys/class/thermal/thermal_zone0/temp` |
| Big core 0 | `bigcore0-thermal` | `bigcore0_thermal` (hwmon1) | `thermal_zone1/temp` |
| Big core 1 | `bigcore1-thermal` | `bigcore1_thermal` (hwmon2) | `thermal_zone2/temp` |
| Little core | `littlecore-thermal` | `littlecore_thermal` (hwmon3) | `thermal_zone3/temp` |
| Center | `center-thermal` | `center_thermal` (hwmon4) | `thermal_zone4/temp` |
| GPU | `gpu-thermal` | `gpu_thermal` (hwmon5) | `thermal_zone5/temp` |
| NPU | `npu-thermal` | `npu_thermal` (hwmon6) | `thermal_zone6/temp` |

Esempio: `type=soc-thermal temp=44384` → **44.38 °C**.

> Nota: il `type` in `/sys/class/thermal` usa il **trattino** (`soc-thermal`), mentre
> `hwmon/*/name` usa l'**underscore** (`soc_thermal`). Consigliato leggere da
> `/sys/class/thermal/` per non dipendere da `lm-sensors`. Gli indici `hwmonN` **non sono
> stabili** tra i boot: risolverli sempre leggendo il file `name`.

---

## GPU (Mali, driver Panthor)

| Metrica | Path | Perm | Formato | Note |
|---|---|---|---|---|
| **Carico %** | `/sys/class/devfreq/fb000000.gpu/load` | 🟢 | `0@300000000Hz` | `load%@freqHz`. **Niente root** — usare questo, non il debugfs mali |
| Frequenza | `/sys/class/devfreq/fb000000.gpu/cur_freq` | 🟢 | `300000000` | In **Hz** → `/1e6` = MHz |
| Temperatura | `thermal_zone5/temp` (`gpu-thermal`) | 🟢 | milli-°C | |
| (alt.) Utilizzo dettagliato | `/sys/kernel/debug/mali0/dvfs_utilization` | 🔴 | `busy_time:X idle_time:Y` | Esiste ma serve root; il `load` devfreq è sufficiente |

---

## NPU (RKNPU — specifico RK3588)

| Metrica | Path | Perm | Formato | Note |
|---|---|---|---|---|
| **Carico per-core** (3 core) | `/sys/kernel/debug/rknpu/load` | 🔴 | `NPU load:  Core0:  0%, Core1:  0%, Core2:  0%,` | Regex `Core(\d+):\s*(\d+)%` |
| Frequenza (condivisa) | `/sys/class/devfreq/fdab0000.npu/cur_freq` | 🟢 | `1000000000` | In **Hz** → 1.0 GHz. Leggibile senza root |
| Versione driver | `/sys/kernel/debug/rknpu/version` (root); ripiego `/sys/module/rknpu/version` | 🔴 / 🟢 | `RKNPU driver: v0.9.8` / `0.9.8` | Statico, leggere una volta |
| Temperatura | `thermal_zone6/temp` (`npu-thermal`) | 🟢 | milli-°C | |

---

## RGA (acceleratore grafica 2D — specifico RK3588)

Tutto in **debugfs → 🔴 root**.

| Metrica | Path | Formato | Note |
|---|---|---|---|
| Carico per-scheduler | `/sys/kernel/debug/rkrga/load` | vedi sotto | 3 scheduler: `rga3`, `rga3`, `rga2` |
| Versione driver | `/sys/kernel/debug/rkrga/driver_version` | `RGA multicore Device Driver: v1.3.10` | Statico |
| Frequenza | `/sys/kernel/debug/clk/<clock>/clk_rate` (Hz) per `clk_rga3_0_core`, `clk_rga3_1_core`, `clk_rga2_core` | un file minuscolo per clock; ripiego: `clk_summary` (269 KB, ~14 ms, colonna 5 = rate Hz) | 750 MHz |

Output reale di `rkrga/load`:
```
num of scheduler = 3
================= load ==================
scheduler[0]: rga3
	 load = 0%
-----------------------------------
scheduler[1]: rga3
	 load = 0%
-----------------------------------
scheduler[2]: rga2
	 load = 0%
-----------------------------------
```
Parsing: associare ogni `scheduler[N]: <nome>` alla riga `load = X%` successiva →
`rga3_0`, `rga3_1`, `rga2_2`.

> Nota `clk_summary` (usato solo come ripiego, vedi `FUNZIONAMENTO.md` «Costo di un gather»): su questo kernel ha colonne extra. Il **rate in Hz è il 5° campo**
> (`clk_rga3_0_core 0 1 0 750000000 …`).

---

## VPU / MPP — encoder & decoder video (specifico RK3588)

Interfaccia **MPP (Media Process Platform)** di Rockchip, esposta in `/proc/mpp_service/`.
**Non usata dai due tool di riferimento**; RkTopNG la legge per **tutte** le unità
disponibili sulla board (collector `exporter/collectors/vpu.go`).

| Metrica | Path | Perm | Note |
|---|---|---|---|
| **Carico per-unità video** | `/proc/mpp_service/load` | 🟢 | `load:` + `utilization:` per ogni core. Leggibile **senza root** |
| Intervallo di campionamento | `/proc/mpp_service/load_interval` | 🔴 write | Valore in ms (già `1000` di default). Scrittura **richiede root**; in lettura il load funziona già con il default |
| Sessioni attive enc/dec | `/proc/mpp_service/sessions-summary` | 🟢 | Vuoto se nessuna sessione; mostra processi che usano il VPU |
| Versione MPP | `/proc/mpp_service/version` | 🟢 | Statica |
| Frequenza encoder | `/sys/class/devfreq/fdbd0000.rkvenc-core/cur_freq`, `fdbe0000…` | 🟢 | Hz (es. `786431991` → ~786 MHz). Nessun devfreq per i decoder |

Formato di `/proc/mpp_service/load` (un core per riga):
```
fdbd0000.rkvenc-core      load:   0.00% utilization:   0.00%
```
Parsing: `parts = split_whitespace`; `parts[0]`=device, `parts[2]`=load%, `parts[4]`=utilization%.

Unità video presenti sulla board (mappatura indirizzo → nome):

| Indirizzo | Unità | Tipo |
|---|---|---|
| `fdbd0000.rkvenc-core` | ENC Core0 | Encoder H.264/H.265 |
| `fdbe0000.rkvenc-core` | ENC Core1 | Encoder H.264/H.265 |
| `fdc38100.rkvdec-core` | DEC Core0 | Decoder video |
| `fdc48100.rkvdec-core` | DEC Core1 | Decoder video |
| `fdc70000.av1d` | AV1 Decoder | Decoder AV1 |
| `fdb51000.avsd-plus` | AVS+ Decoder | Decoder AVS+ |
| `fdb50400.vdpu` | VDPU | Decoder video legacy |
| `fdb90000.jpegd` | JPEG Decoder | |
| `fdba0000/fdba4000/fdba8000/fdbac000.jpege-core` | JPEG Encoder ×4 | |
| `fdbb0000.iep` | IEP | Image Enhancement Processor |

> Il collector legge tutte le unità della tabella: **rkvenc ×2 + rkvdec ×2** (ENC/DEC Core0/1),
> **AV1 decoder**, **JPEG enc/dec**, **IEP**; tutte dalla stessa sorgente e senza root.

---

## Memoria

| Metrica | Path | Perm | Note |
|---|---|---|---|
| RAM (total/free/available/cached…) | `/proc/meminfo` | 🟢 | In **kB**. `MemTotal: 16334100 kB`. Usato = Total − Available |
| Swap | `/proc/swaps` | 🟢 | **Vuoto su questa board** (nessuno swap) |
| ZRAM | `/sys/block/zram0/mm_stat` | — | **Assente** (zram non configurato) |

---

## Sistema / Rete / Disco

| Metrica | Path | Perm | Formato / esempio |
|---|---|---|---|
| Uptime | `/proc/uptime` | 🟢 | `231307.24 1843825.27` (uptime, idle) — secondi |
| Load average | `/proc/loadavg` | 🟢 | `0.03 0.02 0.00 1/525 40555` (1/5/15min, run/tot, lastpid) |
| Hostname | `/proc/sys/kernel/hostname` | 🟢 | |
| Kernel release | `/proc/sys/kernel/osrelease` | 🟢 | |
| Nome board | `/proc/device-tree/model` | 🟢 | `FriendlyElec CM3588` (terminato da `\0`) |
| **SoC + vendor** | `/proc/device-tree/compatible` | 🟢 | vedi sezione "Pannello SYS" sotto |
| Connessioni TCP | `/proc/net/sockstat` + `sockstat6`: `inuse` (v4 e v6) + `tw` | 🟢 | contatori già pronti (O(1)); ripiego: righe di `/proc/net/tcp` e `tcp6`, che costa di più perché il kernel percorre tutti i socket |
| Mount / filesystem | `/proc/mounts` + `statvfs()` | 🟢 | filtrare fs virtuali (tmpfs, proc, sysfs…) |
| I/O rete (RX/TX) | `/sys/class/net/{iface}/statistics/{rx,tx}_bytes` | 🟢 | cumulativi → rate dal delta. Iface presenti: `eth0`, `lo`, `tailscale0` (si escludono `lo`, `veth*`, `docker*`, `br-*`) |
| Tipo e velocità dell'interfaccia | `/sys/class/net/{iface}/{type,speed,wireless,tun_flags,device}` | 🟢 | `eth0`: type 1 + `device` → ethernet, `speed` 1000; `tailscale0`: type 65534 + `tun_flags` → vpn, `speed` -1 (assente) |
| Versione RKNN runtime | `/usr/lib/librknnrt.so` | 🟢 | parsing ELF (`librknnrt version:`) |
| Versione RKLLM runtime | `/usr/lib/librkllmrt.so` | — | **Assente** (solo RKNN installato) |

---

## Pannello SYS — info statiche della board

Campi mostrati nel riquadro **SYS** del tool rktop. Sono **info statiche** (lette una volta
all'avvio e messe in cache). Tutte 🟢 tranne le versioni driver NPU/RGA (🔴 debugfs).

| Campo | Sorgente | Perm | Formato / esempio |
|---|---|---|---|
| Board name | `/proc/device-tree/model` | 🟢 | `FriendlyElec CM3588` (NUL-terminated) |
| **SoC** | `/proc/device-tree/compatible` | 🟢 | voce `rockchip,rk3588` → `RK3588` |
| **Vendor** | `/proc/device-tree/compatible` | 🟢 | parte vendor di `rockchip,rk3588` → `Rockchip` |
| Architettura CPU | `/proc/cpuinfo` (`CPU architecture` + `CPU part`) | 🟢 | es. `ARMv8 (A55+A76)` |
| Host | `/proc/sys/kernel/hostname` | 🟢 | `CM3588` |
| Kernel | `/proc/sys/kernel/osrelease` | 🟢 | `6.1.141` |
| NPU driver | `/sys/kernel/debug/rknpu/version` (ripiego: `/sys/module/rknpu/version`) | 🔴 / 🟢 | `RKNPU driver: v0.9.8` |
| VPU driver (MPP) | `/proc/mpp_service/version` (o `/sys/module/rk_vcodec/version`) | 🟢 | `c79104d97229 author: Yandong Lin 2025-08-25 video: rockchip: mpp: …` (un commit, non un numero di versione) |
| GPU driver (Mali kbase) | `/sys/module/valhall_kbase/version` (qualunque `*kbase*`) | 🟢 | `g29p0-00eac0 (UK version 1.36)`; il modello è in `/sys/class/misc/mali0/device/gpuinfo` (`Mali-G610 4 cores r0p0 0x0A080607`) |
| RGA driver | `/sys/kernel/debug/rkrga/driver_version` | 🔴 | `RGA multicore Device Driver: v1.3.10` |
| RKNN runtime | `/usr/lib/librknnrt.so` (parsing ELF) | 🟢 | `librknnrt version: X.Y.Z` |
| RKLLM runtime | `/usr/lib/librkllmrt.so` (parsing ELF) | — | **Assente** su questa board |

### Rilevamento SoC + vendor (verificato)

> `/proc/device-tree/model` contiene `FriendlyElec CM3588` (**non** "RK3588"), quindi la
> vecchia regex `RK\d+` sul model falliva → mostrava `Unknown RK`. La sorgente corretta è
> **`/proc/device-tree/compatible`**, dove le voci sono coppie `vendor,soc` separate da NUL:

```
friendlyelec,cm3588
rockchip,rk3588      ← SoC + vendor canonici
```

Logica (implementata in `exporter/collectors/info.go`, funzione `socVendorAndModel()`):
1. Leggi `/proc/device-tree/compatible` (fallback `/sys/firmware/devicetree/base/compatible`).
2. Splitta sul byte NUL (`\0`).
3. Trova la voce `vendor,soc` dove `soc` inizia per `rk` (case-insensitive).
4. **SoC** = `soc` in maiuscolo (`rk3588` → `RK3588`); **Vendor** = `vendor` capitalizzato (`rockchip` → `Rockchip`).
5. Se nessuna voce corrisponde: vendor `Rockchip`, SoC `Unknown RK`.

Risultato nel pannello: `SoC: Rockchip RK3588`. Approccio **vendor-agnostic** (funziona su
altre board Rockchip senza hardcoding).

---

## 🎁 Sensori bonus CM3588 NAS (non sfruttati dai tool di riferimento)

Tutti 🟢 **senza root**, esposti via hwmon/devfreq. Risolvere sempre per `name` (indici non stabili).

| Metrica | Sorgente | Perm | Esempio | Note |
|---|---|---|---|---|
| **Temperatura SSD NVMe** | hwmon `name=nvme` → `temp{1,2,3}_input` | 🟢 | `temp1_input=33850` → 33.85 °C | Più sensori (composite + per-sensore). Fino a 3 device NVMe sul NAS Kit |
| **Tensione ingresso** | hwmon `name=simple_vin` → `in0_input` | 🟢 | `in0_input=12220` → **12.22 V** | Alimentazione DC |
| **USB-C PD** | hwmon `name=tcpm_source_psy*` → `in0_input`, `curr1_input` | 🟢 | | Tensione/corrente del Power Delivery |
| **Ventola (PWM)** | hwmon `name=pwmfan` → `pwm1`, `pwm1_enable` | 🟢 | | ⚠️ `fan1_input` **vuoto**: niente RPM/tachimetro, solo duty cycle PWM (0–255) |
| **Frequenza DDR** | `/sys/class/devfreq/dmc/cur_freq` | 🟢 | `528000000` → 528 MHz | Controller memoria |
| **Encoder video HW** | `/sys/class/devfreq/fdbd0000.rkvenc-core`, `fdbe0000.rkvenc-core` | 🟢 | `786431991` Hz | 2× RKVENC, freq (load non esposto qui) |
| **Display (VOP)** | `/sys/class/devfreq/fdd90000.vop/cur_freq` | 🟢 | | Video Output Processor |

---

## Permessi: cosa richiede root e come gira il servizio

La **stragrande maggioranza** delle metriche è leggibile **senza root**: CPU, tutte le temperature, GPU (carico e
frequenza), frequenza NPU, memoria, rete, dischi (I/O, capacità, temperatura NVMe) e tutti i sensori bonus del NAS.

Richiedono **root** soltanto:
- **debugfs** (`/sys/kernel/debug`, modo 0700): carico NPU (`rknpu/load`), carico RGA, versione del driver RGA
  (`rkrga/`) e clock RGA (`clk/<clock>/clk_rate`). La versione del driver NPU ha un ripiego senza root
  (`/sys/module/rknpu/version`).
- **S.M.A.R.T.**: `smartctl` deve aprire `/dev/sdX` e `/dev/nvmeX` (temperatura dei SATA, salute, ore, identità e la
  finestra di ogni disco).

**Scelta fatta**: il servizio gira **come root**: nell'installazione nativa con `User=root` nell'unit systemd, in Docker
come container `privileged` con `/proc`, `/sys` e `/dev` dell'host. Così i dati che richiedono root ci sono sempre, senza
regole `sudo` da mantenere. Se il processo gira senza privilegi, il servizio funziona ugualmente ma **non emette** quei
dati (i log dicono perché, una volta per disco) e l'interfaccia mostra `n/a`. Dettagli in
[FUNZIONAMENTO.md](FUNZIONAMENTO.md) §3.4 e §7.

---

## Contatori cumulativi (richiedono 2 letture)

Questi NON sono valori istantanei: il backend deve mantenere lo stato precedente e
calcolare il delta su un intervallo di polling:
- `/proc/stat` → carico CPU, ctxt, intr, softirq
- `/sys/class/net/*/statistics/*_bytes` → rate di rete
- `/sys/kernel/debug/mali0/dvfs_utilization` (se usato) → utilizzo GPU

Istantanei (lettura singola): tutte le frequenze, temperature, tensioni, `meminfo`,
`loadavg`, NPU/RGA load (già espressi in %).

---

## Storage, ZFS, SMART, USB-C (aggiunti nel backend, verificati sulla board)

| Dato | Sorgente | Root? | Note |
|---|---|---|---|
| Volumi montati | `<proc>/1/mounts` (fallback `/proc/mounts`) + `statfs()` | no | In un container `/proc/self/mounts` è la tabella del *container*: si legge pid 1 dell'host. Si filtrano overlay (tranne `/`), Docker, snap, fuse di sistema. `%` = used/(used+avail) come `df` |
| Stato pool ZFS | `/proc/spl/kstat/zfs/<pool>/state` | no | `ONLINE`/`DEGRADED`/… Il throughput logico per pool (`…/<pool>/io`) NON esiste su questo ZFS |
| Inventario dischi | `/sys/block/<dev>/{size,device/model,device/vendor,queue/rotational}` | no | `size` è in settori da 512 B. Bus dal symlink di `/sys/block/<dev>` (`/nvme/`, `/ata`, `/usb`, `mmcblk`). Escluse le partizioni di boot eMMC (`mmcblk*boot*`, `*rpmb`), loop, zram, md, dm |
| Temperatura NVMe | `hwmon name=nvme` → `temp1_input` (Composite) | no | L'indice hwmon non è stabile: il drive si identifica dal symlink `hwmon*/device` (→ `…/nvme/nvme0`) e si mappa su `nvme0n1` |
| SMART (temp, salute, ore) | `smartctl -j -n standby -i -H -A /dev/<dev>` | **sì** | Polling in background (60 s), `-n standby` non sveglia gli HDD. Il kernel non ha `drivetemp`. SATA: attributo 190 → `temperature.current` |
| SMART completo (finestra del disco) | NVMe `smartctl -j -a`, SATA `smartctl -j -x -n standby`; NVMe anche `/sys/class/nvme/nvmeX/hwmon*/temp1_{max,crit}` e `device/{current,max}_link_{speed,width}` | **sì** (smartctl) / no (sysfs) | **Su richiesta**, non in polling. `-x` dà i registri standard ATA (Device Statistics, SCT con temperatura min/max di sempre, limiti e cronologia). L'NVMe esce con exit status 4 se non ha il log dei self-test: dati comunque validi. Dettagli in [smart_status.md](smart_status.md) |
| USB-C: contratto PD | `/sys/class/power_supply/tcpm-source-psy-*/{online,usb_type}` | no | `usb_type` = `[C] PD PD_PPS`: la voce tra parentesi è il tipo attivo. `online=0` = nessun contratto (non "0 V") |
| USB-C: porta | `/sys/class/typec/port0/{power_role,data_role,power_operation_mode,orientation,usb_power_delivery_revision}` + esistenza di `port0-partner` | no | Ruolo attuale = valore tra parentesi (`source [sink]`) |
| Nome OS | `/etc/os-release` → `PRETTY_NAME` | no | Nel container: `RKTOP_OSRELEASE_PATH` |
