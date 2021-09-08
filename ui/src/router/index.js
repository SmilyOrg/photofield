import { createWebHistory, createRouter } from "vue-router";
import App from "../App.vue";
import Home from "../components/Home.vue";
import NaturalViewer from "../components/NaturalViewer.vue";

const routes = [
  {
    path: "/",
    component: App,
    props: true,
    children: [
      {
        name: "home",
        path: "/",
        component: Home,
        props: true,
      },
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
    ],
  }
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
