import { createApp } from 'vue'
import Root from './Root.vue'
import './index.css'
import router from './router'

import BalmUI from 'balm-ui'; // Official Google Material Components
import BalmUIPlus from 'balm-ui/dist/balm-ui-plus'; // BalmJS Team Material Components
import 'balm-ui/dist/balm-ui.css';

import "plyr/dist/plyr.css";

import "@fontsource/roboto/300.css";
import "@fontsource/roboto/400.css";
import "@fontsource/roboto/500.css";

import "vue-multiselect/dist/vue-multiselect.css";
import { createHead } from '@unhead/vue';

const app = createApp(Root);

const head = createHead()
app.use(head)
app.use(BalmUI);
app.use(BalmUIPlus);

app.use(router);

app.mount('#app')

window.app = app;
