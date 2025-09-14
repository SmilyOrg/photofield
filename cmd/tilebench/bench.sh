#!/bin/bash

set -euo pipefail

go build -o tilebench .

SCENE="${1}"

ZOOM=17
MIN_X=40700
MAX_X=40720
MIN_Y=74100
MAX_Y=74120
WORKERS=(1 2 4 8 16 32)
# WORKERS=(8)

FORMATS=(
    "image/jpeg;quality=100"
    "image/jpeg;quality=90"
    "image/jpeg;quality=80"
    "image/jpeg;quality=70"
    "image/jpeg;quality=60"
    "image/jpeg;quality=50"
    "image/png"
    "image/avif;quality=50"
    "image/avif;quality=60"
    "image/avif;quality=70"
    "image/avif;quality=80"
    "image/avif;quality=90"
    "image/avif;quality=100"
    "image/webp;encoder=hugo"
    "image/webp;encoder=chai;quality=100"
    "image/webp;encoder=chai;quality=90"
    "image/webp;encoder=chai;quality=80"
    "image/webp;encoder=chai;quality=70"
    "image/webp;encoder=chai;quality=60"
    "image/webp;encoder=chai;quality=50"
    "image/webp;encoder=jackdyn;quality=100"
    "image/webp;encoder=jackdyn;quality=90"
    "image/webp;encoder=jackdyn;quality=80"
    "image/webp;encoder=jackdyn;quality=70"
    "image/webp;encoder=jackdyn;quality=60"
    "image/webp;encoder=jackdyn;quality=50"
    "image/webp;encoder=jacktra;quality=100"
    "image/webp;encoder=jacktra;quality=90"
    "image/webp;encoder=jacktra;quality=80"
    "image/webp;encoder=jacktra;quality=70"
    "image/webp;encoder=jacktra;quality=60"
    "image/webp;encoder=jacktra;quality=50"
)

echo "x,y,size,latency,format,workers,error" > tilebench.csv
for workers in "${WORKERS[@]}"; do
    for format in "${FORMATS[@]}"; do
        ./tilebench -scene $SCENE -zoom $ZOOM -min-x $MIN_X -max-x $MAX_X -min-y $MIN_Y -max-y $MAX_Y -workers $workers -accept $format -csv >> tilebench.csv
    done
done
