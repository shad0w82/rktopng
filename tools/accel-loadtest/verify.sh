#!/bin/bash
# Checks the exporter's accelerator numbers against the kernel's own files while the accelerators are busy.
# Run it ON THE BOARD as a normal user in the docker group: it starts the exporter binary given as $1
# (default ../../dist/rktopng-linux-arm64) in a privileged container on port 9893, and a second privileged
# container that copies the NPU/RGA debugfs load files (root only) twice a second. ~1 minute.
#   ./verify.sh [exporter-binary]      then:   python3 compare.py
cd "$(dirname "$0")" || exit 1
bin=$(realpath "${1:-../../dist/rktopng-linux-arm64}")
[ -x npu_load ] || gcc -O2 npu_load.c -o npu_load -lrknnrt || exit 1
[ -f test.mp4 ] || ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc2=size=1920x1080:rate=30" -t 10 \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p test.mp4 || exit 1
docker rm -f rktopng-verify rktopng-raw >/dev/null 2>&1

docker run -d --name rktopng-verify --privileged -p 9893:9893 -v "$bin:/rktopng:ro" \
  -v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /:/host/root:ro -v /etc/os-release:/host/os-release:ro -v /etc/passwd:/host/passwd:ro \
  -e RKTOP_OSRELEASE_PATH=/host/os-release -e RKTOP_PASSWD_PATH=/host/passwd -e RKTOP_PORT=9893 \
  -e RKTOP_PROC_PATH=/host/proc -e RKTOP_SYS_PATH=/host/sys -e RKTOP_ROOTFS_PATH=/host/root alpine:3.21 /rktopng >/dev/null
docker run -d --name rktopng-raw --privileged -v /sys:/host/sys:ro -v "$PWD:/out" alpine:3.21 sh -c \
  'while true; do echo "T $(date +%s)" >> /out/raw.log; cat /host/sys/kernel/debug/rknpu/load /host/sys/kernel/debug/rkrga/load >> /out/raw.log 2>&1; sleep 0.5; done' >/dev/null
trap 'docker rm -f rktopng-verify rktopng-raw >/dev/null 2>&1' EXIT

rm -f raw.log phases.log sample.jsonl sessions_*.txt
sleep 3
python3 sampler.py 9893 46 > sample.jsonl & SAMPLER=$!
ph() { echo "$(date +%s) $1" >> phases.log; }
ff() { timeout "$1" ffmpeg -hide_banner -loglevel error -stream_loop -1 -hwaccel rkmpp -hwaccel_output_format drm_prime -i test.mp4 \
  -t 3600 -vf scale_rkrga=w=1280:h=720:format=nv12 -c:v h264_rkmpp -b:v 4M -f null - 2>/dev/null; }

ph idle1; sleep 4
ph video; ff 10 & P=$!; sleep 6; cat /proc/mpp_service/sessions-summary > sessions_video.txt 2>&1; wait $P
ph idle2; sleep 4
ph npu_1core; ./npu_load 8 >/dev/null
ph idle3; sleep 4
ph npu_3cores; for _ in 1 2 3; do ./npu_load 8 >/dev/null & done; wait $(jobs -p | grep -v "^$SAMPLER$")
ph idle4; sleep 3
wait $SAMPLER
echo "done: now run  python3 compare.py"
