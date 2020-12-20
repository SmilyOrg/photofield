import { createWebHistory, createRouter } from "vue-router";
import NaturalViewer from "../components/NaturalViewer.vue";

const routes = [
  {
    name: "collection",
    path: "/collections/:collection",
    component: NaturalViewer,
  },
  {
    name: "region",
    path: "/collections/:collection/:region",
    component: NaturalViewer,
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
