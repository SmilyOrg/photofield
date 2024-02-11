import { get as getProjection, fromLonLat, toLonLat } from 'ol/proj';

const proj = getProjection("EPSG:3857");

function toView(geoview, sceneBounds) {
  if (!geoview || !sceneBounds) return null;

  const coord = fromLonLat(geoview, proj);
  const zoom = geoview[2];
  const fullExtent = proj.getExtent();
  const fw = fullExtent[2] - fullExtent[0];
  const fh = fullExtent[3] - fullExtent[1];
  const sx = sceneBounds.w / fw;
  const sy = sceneBounds.h / fh;

  const power = Math.pow(2, zoom);
  const sw = fw / power;
  const sh = fh / power;
  const extent = [
    coord[0] - sw,
    coord[1] - sh,
    coord[0] + sw,
    coord[1] + sh,
  ];
  return {
    x: (extent[0] - fullExtent[0]) * sx,
    y: (fullExtent[3] - extent[3]) * sy,
    w: (extent[2] - extent[0]) * sx,
    h: (extent[3] - extent[1]) * sy,
  };
}

function fromView(view, sceneBounds) {
  if (!view || !sceneBounds) return null;

  const fullExtent = proj.getExtent();
  const fw = fullExtent[2] - fullExtent[0];
  const fh = fullExtent[3] - fullExtent[1];
  const sx = sceneBounds.w / fw;
  const sy = sceneBounds.h / fh;

  const extent = [
    view.x / sx + fullExtent[0],
    fullExtent[3] - view.y / sy,
    (view.x + view.w) / sx + fullExtent[0],
    fullExtent[3] - (view.y + view.h) / sy,
  ];
  const center = [
    (extent[0] + extent[2]) / 2,
    (extent[1] + extent[3]) / 2,
  ];
  const sw = (extent[2] - extent[0]) * 0.5;
  const sh = (extent[1] - extent[3]) * 0.5;
  const power = Math.max(fw / sw, fh / sh);
  const zoom = Math.log2(power);
  const lonlat = toLonLat(center, proj);
  return [lonlat[0], lonlat[1], zoom];
}

function equal(a, b) {
  if (!a || !b) return false;
  return (
    Math.abs(a[0] - b[0]) < 1e-4 &&
    Math.abs(a[1] - b[1]) < 1e-4 &&
    Math.abs(a[2] - b[2]) < 1e-1
  );
}

export default {
  toView,
  fromView,
  equal,
};