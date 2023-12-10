/** Generated from: tests\rescan.feature */
import { test } from "..\\..\\tests\\fixtures.ts";

test.describe("Rescan", () => {

  test("One photo", async ({ Given, app, And, When, page, Then }) => {
    await Given("an empty working directory", null, { app });
    await And("a running app", null, { app });
    await And("the following files:", {"dataTable":{"rows":[{"cells":[{"value":"src"},{"value":"dst"}]},{"cells":[{"value":"docs/assets/logo-wide.jpg"},{"value":"photos/a.jpg"}]}]}}, { app });
    await When("the user opens \"/collections/photos\"", null, { app });
    await And("the user clicks \"photos\"", null, { page });
    await Then("the page shows \"0 files indexed\"", null, { page });
    await When("the user clicks \"Rescan\"", null, { page });
    await Then("the page shows \"1 file indexed\"", null, { page });
    await When("the user clicks \"photos\"", null, { page });
    await Then("the page shows photo \"photos/a.jpg\"", null, { app, page });
  });

});

// == technical section ==

test.use({
  $test: ({}, use) => use(test),
});