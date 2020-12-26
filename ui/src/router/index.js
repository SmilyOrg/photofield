import { createWebHistory, createRouter } from "vue-router";
import NaturalViewer from "../components/NaturalViewer.vue";

const routes = [
  {
    name: "collection",
    path: "/collections/:collectionId",
    component: NaturalViewer,
    props: true,
  },
  {
    name: "region",
    path: "/collections/:collectionId/:regionId",
    component: NaturalViewer,
    props: true,
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
