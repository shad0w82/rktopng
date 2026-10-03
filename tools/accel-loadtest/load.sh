#!/bin/bash
# Puts load on the RK3588 accelerators so the dashboard can be seen reacting:
#   VPU  hardware decode + encode (ffmpeg with rkmpp)      RGA  hardware scaling (scale_rkrga)
#   NPU  three INT8 matrix-multiplication processes, one per core (no model file needed)
# Run it ON THE BOARD, as a normal user (needs ffmpeg with rkmpp/rkrga, gcc and librknnrt):
#   ./load.sh [seconds]        default 30
cd "$(dirname "$0")" || exit 1
secs=${1:-30}

[ -x npu_load ] || gcc -O2 npu_load.c -o npu_load -lrknnrt || exit 1
[ -f test.mp4 ] || ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc2=size=1920x1080:rate=30" -t 10 \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p test.mp4 || exit 1

echo "loading VPU + RGA + NPU for ${secs}s (Ctrl-C to stop)"
trap 'kill $(jobs -p) 2>/dev/null' EXIT
timeout "$secs" ffmpeg -hide_banner -loglevel error -stream_loop -1 -hwaccel rkmpp -hwaccel_output_format drm_prime \
  -i test.mp4 -t 3600 -vf scale_rkrga=w=1280:h=720:format=nv12 -c:v h264_rkmpp -b:v 4M -f null - &
for _ in 1 2 3; do ./npu_load "$secs" >/dev/null & done
wait
