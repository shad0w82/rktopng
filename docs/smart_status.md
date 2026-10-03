# RkTopNG — Finestra S.M.A.R.T.: fonti, stato di salute, indipendenza dal produttore

La finestra si apre dal riquadro di un disco nella card **Disk I/O** (desktop). Mostra lo stato di salute
del disco, le cifre che contano, l'identità, i registri del disco e la tabella completa degli attributi.
È **di sola lettura**: non avvia self-test e non scrive nulla sul disco.

Nell'interfaccia: `frontend/src/ui/smart/` (componenti) e `frontend/src/lib/data/smart.ts` (formati, carte, tabella, testi: funzioni pure testate). Il
frontend non rifà nessun giudizio: stato, motivi e colore di ogni cifra li decide l'exporter.

Dati: `GET /api/smart/<disco>` (es. `/api/smart/nvme0n1`). Il disco si legge **quando la finestra si
apre** (non c'è polling) e il risultato resta in cache 30 s (8 s se il disco non era leggibile). Più
finestre aperte insieme condividono una sola lettura, e `smartctl` non gira mai due volte in parallelo.

## 1. Come si legge un disco

| Disco | Comando | Perché |
|---|---|---|
| NVMe | `smartctl -j -a /dev/nvmeXnY` | il log di salute NVMe è definito dalla specifica: stessi campi per ogni marca. `-a` legge anche il log dei self-test quando il disco ce l'ha |
| SATA / altro | `smartctl -j -x -n standby /dev/sdX` | `-x` aggiunge i registri **standard** ATA (Device Statistics, SCT). `-n standby`: un disco in standby non viene svegliato |

`smartctl` esce con un codice a bit. I bit 0-1 significano "non sono riuscito ad aprire il disco" (la
risposta ha `available: false` e un `reason`: `standby`, `permission`, `unsupported`, `smartctl_missing`,
`disabled`, `failed`). Gli altri bit descrivono il disco e i dati restano validi: per esempio i Lexar NM790
escono con 4 solo perché non hanno il log dei self-test.

Servono root (o `CAP_SYS_RAWIO`) e `smartmontools`, come per il polling di SMART. Dal kernel, senza root, arrivano
due cose che `smartctl` non dà per gli NVMe: i limiti di temperatura del disco (`hwmon temp1_max` / `temp1_crit`) e il
collegamento PCIe attuale e massimo del disco (`/sys/class/nvme/nvmeX/device/{current,max}_link_{speed,width}`).

## 2. Indipendenza dal produttore

Il significato di un attributo SATA dipende dal produttore: l'ID 177 è `Wear_Leveling_Count` su un Samsung,
`Total_LBAs_Written` conta settori mentre `Lifetime_Writes_GiB` è già in GiB, l'ID 197 su alcuni modelli si chiama
`Not_In_Use`. Per questo la finestra si fonda su tre livelli:

1. **Sempre corretto, per ogni marca**: la tabella mostra ciò che `smartctl` riporta (nome dal suo database, che
   conosce il produttore), e lo stato usa solo regole universali (§3).
2. **Fonti standard per prime**. Per i SATA: *Device Statistics* (settori letti e scritti, indicatore standard di
   usura degli SSD, contatori di settori riallocati, errori non correggibili ed errori CRC) e *SCT* (temperatura
   minima e massima di sempre, limiti di funzionamento, cronologia di ~21 ore). Per gli NVMe: il log di salute.
3. **Riconoscimento per nome, solo dove manca uno standard**: usura, dati scritti/letti e spegnimenti anomali
   quando il disco non ha i registri standard. Le regole stanno in `exporter/collectors/smartdetail_attrs.go`;
   un attributo si riconosce dal **nome** (con l'ID solo per i contatori di errore, e solo se anche il nome è di
   quel tipo: `Not_In_Use` non diventa mai un contatore). Se nessun nome combacia la cifra è **assente**
   (`n/a` nella UI): non si indovina mai.

Il test `TestRolesAgainstDriveDB` controlla le regole contro **tutti** i nomi del database di `smartctl`
(`/usr/share/smartmontools/drivedb.h`, licenza GPL: si legge dal sistema, non è nel repository; con
`RKTOP_DRIVEDB=<percorso>` si indica un'altra copia). Verifica per esempio che i contatori NAND/flash non siano scambiati
per scritture dell'host, che ogni nome di "spegnimento anomalo" sia riconosciuto e che ID come il 198 non diventino
contatori d'errore quando il produttore li usa per altro (`Read_Sectors_Tot_Ct`).

Per ogni cifra la risposta dice da dove viene (`wear_source`, `written_source`, `unsafe_source`: "device statistics",
"NVMe health log" o "attribute 241 Total_LBAs_Written").

**Limiti noti**: SAS/SCSI hanno identità, stato, temperatura, ore e difetti "grown" ma non la tabella completa;
un adattatore USB che non passa S.M.A.R.T. risponde `unsupported`; gli spegnimenti anomali dei SATA non hanno uno
standard, quindi dipendono dal riconoscimento per nome (Samsung `POR_Recovery_Count`, Kingston/Crucial
`Unsafe_Shutdown_Count` o `Unexpect_Power_Loss_Ct`, HDD `Power-Off_Retract_Count`, ...).

## 3. Stato di salute: OK / Warning / Failing

Lo stato è il peggiore dei motivi trovati; ogni motivo è elencato nella finestra.

| | Failing (`fail`) | Warning (`warn`) |
|---|---|---|
| **Ogni disco** | l'autovalutazione del disco non è superata (`smart_status.passed = false`) | temperatura attuale ≥ limite del disco (SATA: limite di funzionamento SCT; NVMe: `temp1_max`) |
| **NVMe** | `critical_warning` ≠ 0 · spare ≤ soglia del disco · usura ≥ 100 % | errori media > 0 · spare < 50 % · usura ≥ 80 % |
| **SATA** | un attributo ha toccato la soglia **ora** (`when_failed = now`, verdetto di `smartctl`) · usura ≥ 100 % | un attributo l'ha toccata **in passato** (`when_failed = past`) · contatore > 0 su settori riallocati, in attesa, non correggibili offline, errori non correggibili, errori CRC · voci nel log errori ATA · ultimo self-test non riuscito · usura ≥ 80 % |

L'usura è l'indicatore standard degli SSD (Device Statistics, "Percentage Used Endurance Indicator"); se manca,
`100 − valore normalizzato` di un attributo di usura riconosciuto per nome, solo se il valore sta tra 0 e 100.

**Mostrati ma mai contati** (non cambiano lo stato): spegnimenti anomali, voci del log errori NVMe (alcuni dischi ne
scrivono per comandi respinti, innocui), minuti sopra la soglia di temperatura, il massimo storico di temperatura
(un picco passato non è un guasto di oggi) e l'attributo "worst" di qualunque attributo.

I numeri di soglia (80 % / 100 % di usura, 50 % di spare) sono in `smartdetail_parse.go`
(`wearWarnPct`, `wearFailPct`, `spareWarnPct`).

## 4. Cosa significano Normalized, Worst, Threshold, Raw (tabella SATA)

- **Raw**: la quantità vera, nell'unità propria dell'attributo (ore, settori, °C…). Si legge dal testo di `smartctl`
  ("34 (Min/Max 20/45)" → 34) perché il campo numerico può contenere byte di servizio.
- **Normalized**: punteggio calcolato dal disco; più alto è meglio, scende quando le cose peggiorano; la scala è del
  produttore (spesso 100 = nuovo, ma l'attributo 195 dei Samsung parte da 200).
- **Worst**: il Normalized più basso mai registrato; non risale mai.
- **Threshold**: se il Normalized scende a questo valore o sotto, il disco dichiara l'attributo guasto. `0` ("—") =
  nessuna soglia: quell'attributo non può guastarsi.
- "critical" in tabella = attributo marcato *pre-fail* dal disco, oppure contatore d'errore, oppure indicatore di usura.

## 5. Forma della risposta

`device`, `available`, `reason`, `message`, `read_at`, `smartctl`, `protocol` (`ATA`/`NVMe`/`SCSI`), poi:
`identity` (modello, famiglia, firmware, seriale, WWN, capacità, collegamento attuale e massimo, `known_model`),
`health` (`state`: `ok`/`warn`/`fail`, o `unknown` se il disco non si è lasciato leggere; `passed`, `reasons[]`), `vitals` (temperatura e suoi limiti, ore, cicli, usura, spare, scritto,
letto, spegnimenti anomali, contatori, `levels{}` per colorare), `attributes[]` (SATA; gli attributi di traffico dell'host portano anche `bytes`, il grezzo convertito in byte) oppure `nvme{}` (NVMe),
`logs` (voci del log errori, self-test) e `temp_history` (cronologia SCT, dal più vecchio, ogni 10 min).
Le cifre che il disco non espone sono assenti dal JSON, non zero.

Il numero di serie e il WWN si leggono **solo** da questo endpoint: non sono esportati né in `/metrics` né in `/api/info`
(l'etichetta `serial` di `smart_info` è stata tolta).
