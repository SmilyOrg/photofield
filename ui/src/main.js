import { createApp } from 'vue'
import App from './App.vue'
import './index.css'
import router from './router'

import BalmUI from 'balm-ui'; // Official Google Material Components
import BalmUIPlus from 'balm-ui/dist/balm-ui-plus'; // BalmJS Team Material Components
import 'balm-ui/dist/balm-ui.css';
import "overlayscrollbars/css/OverlayScrollbars.min.css";
import "./os-theme-minimal-dark.css";
import "./os-theme-customizations.css";

import "overlayscrollbars";
import "./scrollbar-timeline-ext.js";
import "./scrollbar-timeline-ext.css";

import "plyr/dist/plyr.css";

const app = createApp(App);

app.use(BalmUI);
app.use(BalmUIPlus);

app.use(router);

app.mount('#app')

window.app = app;
