<template>
  <div>
    <ui-skeleton
      :loading="loading"
      :active="true"
      :avatar="false"
      :title="false"
      :paragraph="{ width: '100%', rows: 1 }"
    >
    </ui-skeleton>
    <div v-if="!consent" class="consent">
      <ui-alert state="info">
        Displaying the map will share location data with <a href="https://www.openstreetmap.org/" target="_blank">OpenStreetMap</a>.
        <ui-button @click="consent = true">I agree, show the map</ui-button>
      </ui-alert>
    </div>
    <div
      v-if="consent"
      v-show="!loading"
      class="map-container"
      ref="mapContainer"
    ></div>
  </div>
</template>

<script setup>
import { useStorage } from '@vueuse/core';
import { Feature } from 'ol';
import { Point } from 'ol/geom';
import TileLayer from 'ol/layer/Tile';
import VectorLayer from 'ol/layer/Vector';
import Map from 'ol/Map';
import { fromLonLat, get as getProjection, toLonLat } from 'ol/proj';
import OSM from 'ol/source/OSM';
import VectorSource from 'ol/source/Vector';
import Style from 'ol/style/Style';
import Text from 'ol/style/Text';
import View from 'ol/View';
import { onMounted, ref, toRefs, watch } from 'vue';

const proj = getProjection("EPSG:3857");

const props = defineProps({
  geoview: Array,
  loading: Boolean,
});

const {
  geoview,
  loading,
} = toRefs(props);

const mapContainer = ref(null);
const map = ref(null); // Add a ref for the map variable

const consent = useStorage('map-consent', false);

watch(mapContainer, (newValue, oldValue) => {
  if (newValue === oldValue) return;
  if (!newValue) return;
  initOpenLayers(newValue);
});

function setGeoview(geoview) {
  if (!map.value || !geoview) return;
  const view = map.value.getView();
  view.setCenter(fromLonLat(geoview, proj));
  view.setZoom(geoview[2]);
  const source = map.value.getLayers().getArray()[1].getSource();
  const feature = source.getFeatures()[0];
  feature.setGeometry(new Point(fromLonLat(geoview, proj)));
}

watch(geoview, (newValue, oldValue) => {
  if (newValue === oldValue) return;
  if (!newValue) return;
  if (oldValue && newValue[0] === oldValue[0] && newValue[1] === oldValue[1] && newValue[2] === oldValue[2]) return;
  setGeoview(newValue);
});

function initOpenLayers(element) {
  map.value = new Map({
    target: element,
    layers: [
      new TileLayer({
        source: new OSM(),
      }),
    ],
    view: new View(),
  });

  const source = new VectorSource();
  const feature = new Feature();
  feature.setStyle(new Style({
    text: new Text({
      text: 'place',
      font: "40px Material Icons",
    }),
  }));
  feature.on()
  source.addFeature(feature);
  
  const markerLayer = new VectorLayer({
    source,
  });
  map.value.addLayer(markerLayer);

  map.value.on('click', function (event) {
    map.value.forEachFeatureAtPixel(event.pixel, function (feature) {
      const coords = feature.getGeometry().getCoordinates();
      const lonlat = toLonLat(coords, proj);
      window.open(`https://www.google.com/maps/search/?api=1&query=${lonlat[1]},${lonlat[0]}`, '_blank');
    })
  });

  map.value.on('pointermove', function (event) {
    const hit = map.value.hasFeatureAtPixel(event.pixel);
    mapContainer.value.style.cursor = hit ? 'pointer' : '';
  });

  setGeoview(geoview.value);
}
</script>

<style scoped>
.map-container, :deep(.mdc-skeleton-paragraph > li), .consent {
  width: 100%;
  height: 200px;
  box-sizing: border-box;
}

.consent {
  padding: 10px;
  vertical-align: center;
  text-align: center;
}

.consent button {
  margin-top: 10px;
}

</style>
