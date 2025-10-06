#!/bin/bash

set -euo pipefail

go build -o tilebench .

SCENE="${1}"

# ZOOM=17; MIN_X=40700; MAX_X=40720; MIN_Y=74100; MAX_Y=74120
# ZOOM=22; MIN_X=1302485; MAX_X=1302499; MIN_Y=2371466; MAX_Y=2371484
# ZOOM=20; MIN_X=325623; MAX_X=325689; MIN_Y=592856; MAX_Y=592914;
# ZOOM=20; MIN_X=325640; MAX_X=325670; MIN_Y=592880; MAX_Y=592910;
# ZOOM=20; MIN_X=325620; MIN_Y=592870;
# ZOOM=20; MIN_X=325620; MIN_Y=592860; EDGE=40;
# ZOOM=19; MIN_X=162783; MIN_Y=296424; EDGE=20;
ZOOM=19; MIN_X=162810; MIN_Y=296430; EDGE=20;
MAX_X=$((MIN_X + EDGE)); MAX_Y=$((MIN_Y + EDGE));

WORKERS=(1 2 4 8 16 32)
# WORKERS=(8)
# WORKERS=(32)

# FORMATS=(
#     "image/jpeg;quality=80"
# )

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

# FORMATS=(
#     "image/jpeg;quality=1"
#     "image/jpeg;quality=2"
#     "image/jpeg;quality=4"
#     "image/jpeg;quality=8"
#     "image/jpeg;quality=10"
#     "image/jpeg;quality=20"
#     "image/jpeg;quality=30"
#     "image/jpeg;quality=40"
#     "image/jpeg;quality=50"
#     "image/jpeg;quality=60"
#     "image/jpeg;quality=70"
#     "image/jpeg;quality=80"
#     "image/jpeg;quality=90"
#     "image/jpeg;quality=100"
    
#     "image/webp;quality=1"
#     "image/webp;quality=2"
#     "image/webp;quality=4"
#     "image/webp;quality=8"
#     "image/webp;quality=10"
#     "image/webp;quality=20"
#     "image/webp;quality=30"
#     "image/webp;quality=40"
#     "image/webp;quality=50"
#     "image/webp;quality=60"
#     "image/webp;quality=70"
#     "image/webp;quality=80"
#     "image/webp;quality=90"
#     "image/webp;quality=100"
# )

echo "x,y,size,latency,format,workers,error" > tilebench.csv
for workers in "${WORKERS[@]}"; do
    for format in "${FORMATS[@]}"; do
        ./tilebench -scene $SCENE -zoom $ZOOM -min-x $MIN_X -max-x $MAX_X -min-y $MIN_Y -max-y $MAX_Y -workers $workers -accept $format -csv >> tilebench.csv
    done
done
