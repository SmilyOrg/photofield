import { createWebHistory, createRouter } from "vue-router";
import App from "../App.vue";
import Home from "../components/Home.vue";
import CollectionView from "../components/CollectionView.vue";
import TagManager from "../components/TagManager.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
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
          component: CollectionView,
          props: true,
        },
        {
          name: "region",
          path: "/collections/:collectionId/:regionId",
          component: CollectionView,
          props: true,
        },
        {
          name: "tags",
          path: "/tags",
          component: TagManager,
          props: true,
        },
        {
          name: "tag",
          path: "/tags/:tagId",
          component: TagManager,
          props: true,
        }
      ],
    }
  ],
});

export default router;
