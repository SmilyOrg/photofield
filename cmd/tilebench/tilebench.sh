#!/bin/bash

set -euo pipefail

go build -o tilebench .

SCENE="${1}"

# ZOOM=16
# MIN_X=20340
# MAX_X=20360
# MIN_Y=37040
# MAX_Y=37060

# http://localhost:8080/scenes/pjgfnTqDBh/tiles?tile_size=512&zoom=17&x=40706&y=74109
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
    "image/avif"
    "image/webp;encoder=HugoSmits86"
    "image/webp;encoder=chai2010;quality=100"
    "image/webp;encoder=chai2010;quality=90"
    "image/webp;encoder=chai2010;quality=80"
    "image/webp;encoder=chai2010;quality=70"
    "image/webp;encoder=chai2010;quality=60"
    "image/webp;encoder=chai2010;quality=50"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=100"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=90"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=80"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=70"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=60"
    "image/webp;encoder=jackdyn;mem=nrgba;quality=50"
    "image/webp;encoder=jacktra;mem=nrgba;quality=100"
    "image/webp;encoder=jacktra;mem=nrgba;quality=90"
    "image/webp;encoder=jacktra;mem=nrgba;quality=80"
    "image/webp;encoder=jacktra;mem=nrgba;quality=70"
    "image/webp;encoder=jacktra;mem=nrgba;quality=60"
    "image/webp;encoder=jacktra;mem=nrgba;quality=50"
)

# FORMATS=(
#     "image/jpeg;quality=100"
#     "image/jpeg;quality=90"
#     "image/jpeg;quality=80"
#     "image/jpeg;quality=70"
#     "image/jpeg;quality=60"
#     "image/jpeg;quality=50"
#     "image/png"
#     "image/avif"
#     "image/webp;encoder=HugoSmits86"
#     "image/webp;encoder=chai2010;quality=100"
#     "image/webp;encoder=chai2010;quality=90"
#     "image/webp;encoder=chai2010;quality=80"
#     "image/webp;encoder=chai2010;quality=70"
#     "image/webp;encoder=chai2010;quality=60"
#     "image/webp;encoder=chai2010;quality=50"
#     "image/webp;encoder=jack;mem=nrgba;quality=100"
#     "image/webp;encoder=jack;mem=nrgba;quality=90"
#     "image/webp;encoder=jack;mem=nrgba;quality=80"
#     "image/webp;encoder=jack;mem=nrgba;quality=70"
#     "image/webp;encoder=jack;mem=nrgba;quality=60"
#     "image/webp;encoder=jack;mem=nrgba;quality=50"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=100"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=90"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=80"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=70"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=60"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=50"
# )

# FORMATS=(
#     "image/jpeg;quality=100"
#     "image/jpeg;quality=90"
#     "image/jpeg;quality=80"
#     "image/jpeg;quality=70"
#     "image/jpeg;quality=60"
#     "image/jpeg;quality=50"
#     "image/png"
#     "image/avif"
#     "image/webp;encoder=HugoSmits86"
#     "image/webp;encoder=chai2010;quality=100"
#     "image/webp;encoder=chai2010;quality=90"
#     "image/webp;encoder=chai2010;quality=80"
#     "image/webp;encoder=chai2010;quality=70"
#     "image/webp;encoder=chai2010;quality=60"
#     "image/webp;encoder=chai2010;quality=50"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=100"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=90"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=80"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=70"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=60"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=50"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=100"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=90"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=80"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=70"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=60"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=50"
# )

# WORKERS=(1 4 16)
# WORKERS=(8)
# WORKERS=(16)
# FORMATS=("image/jpeg" "image/png" "image/avif" "image/webp")
# FORMATS=("image/jpeg" "image/chai2010+webp")
# FORMATS=("image/jpeg" "image/png" "image/chai2010+webp" "image/jack+webp")
# FORMATS=("image/jack+webp" "image/jacklossless+webp" "image/jackq70+webp" "image/jackq50+webp" "image/jackq30+webp")
# FORMATS=("image/jack+webp" "image/jack+webp-nrgba")
# FORMATS=("image/jpeg" "image/jpeg-nrgba" "image/jack+webp" "image/jack+webp-nrgba")
# FORMATS=("image/jpeg" "image/jpeg-nrgba" "image/jack+webp" "image/jack+webp-nrgba" "image/jackdyn+webp-nrgba" "image/jacktra+webp-nrgba")
# FORMATS=("image/jpeg" "image/jpeg;mem=nrgba" "image/webp;encoder=jack" "image/webp;encoder=jack;mem=nrgba" "image/webp;encoder=jackdyn;mem=nrgba" "image/webp;encoder=jacktra;mem=nrgba")

# FORMATS=(
#     "image/jpeg"
#     "image/png"
#     "image/avif"
#     "image/webp;encoder=HugoSmits86"
#     "image/webp;encoder=chai2010"
#     "image/webp;encoder=jackdyn;mem=nrgba"
#     "image/webp;encoder=jacktra;mem=nrgba"
# )

# FORMATS=(
#     "image/jpeg;quality=100"
#     "image/jpeg;quality=80"
#     "image/jpeg;quality=60"
#     "image/jpeg;quality=40"
#     "image/jpeg;quality=20"
#     "image/jpeg;quality=1"
# )

# FORMATS=(
#     "image/webp;encoder=chai2010;quality=100"
#     "image/webp;encoder=chai2010;quality=80"
#     "image/webp;encoder=chai2010;quality=60"
#     "image/webp;encoder=chai2010;quality=40"
#     "image/webp;encoder=chai2010;quality=20"
#     "image/webp;encoder=chai2010;quality=1"
# )

# FORMATS=(
#     "image/webp;encoder=jack;quality=100"
#     "image/webp;encoder=jack;quality=80"
#     "image/webp;encoder=jack;quality=60"
#     "image/webp;encoder=jack;quality=40"
#     "image/webp;encoder=jack;quality=20"
#     "image/webp;encoder=jack;quality=1"
# )

# FORMATS=(
#     "image/jpeg;quality=100"
#     "image/jpeg;quality=90"
#     "image/jpeg;quality=80"
#     "image/jpeg;quality=70"
#     "image/jpeg;quality=60"
#     "image/jpeg;quality=50"
#     "image/png"
#     "image/webp;encoder=HugoSmits86"
#     "image/webp;encoder=chai2010;quality=100"
#     "image/webp;encoder=chai2010;quality=90"
#     "image/webp;encoder=chai2010;quality=80"
#     "image/webp;encoder=chai2010;quality=70"
#     "image/webp;encoder=chai2010;quality=60"
#     "image/webp;encoder=chai2010;quality=50"
#     # "image/webp;encoder=jack;quality=100"
#     # "image/webp;encoder=jack;quality=80"
#     # "image/webp;encoder=jack;quality=60"
#     # "image/webp;encoder=jack;quality=40"
#     # "image/webp;encoder=jack;quality=20"
#     # "image/webp;encoder=jack;quality=1"
#     # "image/webp;encoder=jackdyn;mem=nrgba"
#     # "image/webp;encoder=jacktra;mem=nrgba"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=100"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=90"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=80"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=70"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=60"
#     "image/webp;encoder=jackdyn;mem=nrgba;quality=50"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=100"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=90"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=80"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=70"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=60"
#     "image/webp;encoder=jacktra;mem=nrgba;quality=50"
#     # "image/webp;encoder=chai2010"
#     "image/avif"
# )

echo "x,y,size,latency,format,workers,error" > tilebench.csv
for workers in "${WORKERS[@]}"; do
    for format in "${FORMATS[@]}"; do
        ./tilebench -scene $SCENE -zoom $ZOOM -min-x $MIN_X -max-x $MAX_X -min-y $MIN_Y -max-y $MAX_Y -workers $workers -accept $format -csv >> tilebench.csv
    done
done
