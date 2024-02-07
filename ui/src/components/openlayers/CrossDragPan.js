import { Pointer as PointerInteraction } from 'ol/interaction';
import { centroid as centroidFromPointers } from 'ol/interaction/Pointer';
import { rotate as rotateCoordinate, scale as scaleCoordinate } from 'ol/coordinate';
import { getBottomLeft, getBottomRight, getTopLeft, getTopRight } from 'ol/extent';
import KalmanFilter from 'kalmanjs';

const Axis = {
  None: 0,
  X: 1,
  Y: 2,
}
class CrossDragPan extends PointerInteraction {
  constructor() {
    super({
      handleDownEvent: (event) => {
        this.center = event.map.getView().getCenter();
        this.zoom = event.map.getView().getZoom();
        this.resolution = event.map.getView().getResolution();
        if (this.targetPointers.length > 0) {
          this.centroid = centroidFromPointers(this.targetPointers);
          this.lastEventTime = null;
          this.filter.cov = this.filter.x = NaN;
          return true;
        } else {
          return false;
        }
      },
      handleDragEvent: (event) => {
        const map = event.map;
        const view = map.getView();
        if (!this.panning) {
          this.panning = true;
          view.beginInteraction();
        }

        const targetPointers = this.targetPointers;
        if (this.targetPointers.length !== 1) {
          this.axis = Axis.None;
          return;
        }

        const centroid = centroidFromPointers(targetPointers);
        
        const base = this.centroid;
        const delta = [
          centroid[0] - base[0],
          centroid[1] - base[1],
        ];
        this.delta = delta.slice();
        
        // const time = event.frameState.time;
        const time = Date.now();
        if (this.lastEventTime) {
          const dt = time - this.lastEventTime;
          this.velocity = [
            (centroid[0] - this.lastCentroid[0]) / dt,
            (centroid[1] - this.lastCentroid[1]) / dt,
          ];
        }

        switch (this.axis) {
          case Axis.None:
            const adx = Math.abs(delta[0]);
            const ady = Math.abs(delta[1]);
            if (adx > this.moveThreshold || ady > this.moveThreshold) {
              if (adx > ady) {
                this.axis = Axis.X;
              } else {
                this.axis = Axis.Y;
              }
              this.centroid = centroid;
            }
            break;
          case Axis.X:
            delta[1] = 0;
            this.speed = this.filter.filter(this.velocity[0]);
            scaleCoordinate(delta, view.getResolution());
            rotateCoordinate(delta, view.getRotation());
            view.setCenterInternal([
              this.center[0] - delta[0],
              this.center[1] - delta[1]
            ]);
            break;
          case Axis.Y:
            delta[0] = 0;
            this.speed = this.filter.filter(this.velocity[1]);
            const dy = delta[1];
            scaleCoordinate(delta, view.getResolution());
            rotateCoordinate(delta, view.getRotation());

            // const mapExtent = view.getProjection().getExtent();
            // const viewExtent = view.calculateExtent();
            // const zoomedOutExtent = [
            //   0,
            //   0,
            //   mapExtent[2],
            //   mapExtent[3] * 3,
            // ]
            // // const outZoom = (mapExtent[2] - mapExtent[0]) / (viewExtent[2] - viewExtent[0]);
            // const zoom = view.getZoom();
            // let frac = 1 - (zoom - 7.417) / (this.zoom - 7.417);
            // frac *= frac;
            // // const mapZoom = view.getZoomForResolution(view.getResolutionForExtent(mapExtent))
            // console.log(this.zoom, zoom, frac);

            // const topLeft = getTopLeft(mapExtent);
            // const topRight = getTopRight(mapExtent);
            // const width = topRight[0] - topLeft[0];
            // const center = width * 0.5;
            // const x = this.center[0] * (1 - frac) + center * frac;

            // view.setCenterInternal([
            //   x,
            //   this.center[1],
            // ]);
            // const zoom = this.zoom - delta[1] * 1e-4;
            // view.setZoom(zoom);
            // const pix = map.getPixelFromCoordinateInternal(delta);
            
            const resolution = this.resolution * Math.max(1, 1 + dy * 0.01);
            // console.log(this.resolution, resolution, pix);
            // console.log(map.getPixelFromCoordinate(this.center));
            // console.log(map.getPixelFromCoordinate(centroid));
            // console.log(base[1] - centroid[1]);
            view.setResolution(resolution);

            const mapExtent = view.getProjection().getExtent();
            // const mapRes = view.getResolutionForExtent(mapExtent);
            const mapRes = view.getResolutionForExtent([
              0,
              0,
              mapExtent[2],
              mapExtent[2],
            ]);
            const frac = Math.min(1, (resolution - this.resolution) / (mapRes - this.resolution));
            // console.log(mapRes, this.resolution, resolution, mapRes / resolution);
            // console.log(frac);
            
            const topLeft = getTopLeft(mapExtent);
            const topRight = getTopRight(mapExtent);
            const bottomLeft = getBottomLeft(mapExtent);
            const bottomRight = getBottomRight(mapExtent);
            const width = topRight[0] - topLeft[0];
            const height = bottomLeft[1] - topLeft[1];

            const x = this.center[0] * (1 - frac) + (width * 0.5) * frac;
            const y = this.center[1] * (1 - frac) + (height) * frac;
            
            view.setCenterInternal([
              x,
              this.center[1],
            ]);

            // const zoomedOutCenter = [
            //   this.center[0] + width * 1,
            //   this.center[1],
            // ];
            // view.adjustResolution(Math.max(1, 1 - dy * 0.0005));
            // const dres = 1 - dy * 0.0005;
            // view.adjustResolution(dres, zoomedOutCenter);
            // console.log(mapExtent);
            // console.log(viewExtent);
            // console.log(view.calculateExtentInternal())
            break;
        }

        this.lastCentroid = centroid;
        this.lastEventTime = time;
        
        event.originalEvent.preventDefault();
      },
      handleUpEvent: (event) => {
        const map = event.map;
        const view = map.getView();
        if (this.panning) {
          this.panning = false;
          view.endInteraction();

          let dispatched = false;

          switch (this.axis) {
            case Axis.X:
              const dx = this.delta[0] + this.speed*this.navXSpeed;
              const minw = map.getSize()[0] * this.navXDist;
              if (dx < -minw || dx > minw) {
                this.dispatchEvent({
                  type: 'nav',
                  x: -dx,
                  y: 0,
                });
                dispatched = true;
              }
              break;
            case Axis.Y:
              const dy = this.delta[1] + this.velocity[1]*this.navYSpeed;
              const minh = map.getSize()[1] * this.navYDist;
              const y = dy < -minh ? 1 : dy > minh ? -1 : 0;
              console.log("delta", this.delta[1], "velocity", this.velocity[1], "dy", dy, "minh", minh, "y", y);
              if (y !== 0) {
                this.dispatchEvent({
                  type: 'nav',
                  x: 0,
                  y,
                });
                dispatched = true;
              }
              break;
            case Axis.None:
              this.dispatchEvent({
                type: 'nav',
                interrupted: true,
              });
              dispatched = true;
              break;
          }
          
          if (!dispatched) {
            this.dispatchEvent({
              type: 'nav',
              x: 0,
              y: 0,
            });
          }

          this.axis = Axis.None;
        }
      },
    });

    this.centroid = null;
    this.lastCentroid = null;
    this.lastEventTime = null;
    this.velocity = [0, 0];
    this.speed = 0;
    this.zoom = null;
    this.resolution = null;
    this.panning = false;
    this.axis = Axis.None;
    this.delta = null;
    this.moveThreshold = 5;
    this.navXSpeed = 1000;
    this.navXDist = 0.5;
    this.navYSpeed = 1000;
    this.navYDist = 0.05;
    this.filter = new KalmanFilter({R: 0.01, Q: 3});
  }
}

export default CrossDragPan;