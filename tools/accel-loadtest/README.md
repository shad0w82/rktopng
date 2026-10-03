# Prova degli acceleratori sotto carico

Strumenti da eseguire **sulla board** (non sul Mac) per vedere muoversi la sezione Acceleratori e per
controllare che i numeri dell'exporter siano quelli veri. Servono `ffmpeg` con `rkmpp` e `rkrga` (quello di
Jellyfin lo ha), `gcc` e `librknnrt` (già presenti sulla CM3588 con i driver Rockchip); per `verify.sh` anche Docker.

Copia la cartella sulla board e lavora lì:

```bash
scp -r tools/accel-loadtest <board>:
ssh <board>
cd accel-loadtest
```

| File | A cosa serve |
|---|---|
| `load.sh [secondi]` | genera carico per N secondi (default 30): decodifica + scala + codifica H.264 in hardware (**VPU** e **RGA**) e tre processi che moltiplicano matrici INT8 (**NPU**, un core ciascuno, senza bisogno di un modello). Si guarda il cruscotto intanto. |
| `verify.sh [binario]` | avvia l'exporter (default `../../dist/rktopng-linux-arm64`) in un container privilegiato e, in un secondo container, copia due volte al secondo i file del kernel che solo root legge (`rknpu/load`, `rkrga/load`); poi alterna riposo, video, NPU su 1 core e NPU su 3 core. Dura circa 50 s. |
| `compare.py` | da lanciare dopo `verify.sh`: confronta, secondo per secondo, i valori dell'exporter con quelli del kernel (NPU e RGA da debugfs, VPU da `/proc/mpp_service/load`) e conta le differenze (devono essere 0). |
| `npu_load.c`, `sampler.py` | usati dagli script sopra: il generatore di carico NPU e il campionatore dell'exporter. |

`load.sh` e `verify.sh` creano nella cartella `test.mp4` (una clip di prova da 10 s) e l'eseguibile `npu_load`.

Risultato della prima prova (2026-10-02, kernel 6.1.141, librknnrt 2.3.0): NPU, RGA e VPU dell'exporter coincidono
con i file del kernel in ogni secondo; il numero di sessioni VPU è 2 (un decoder più un encoder) con ffmpeg attivo e
torna a 0 appena finisce.
