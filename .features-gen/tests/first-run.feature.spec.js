/** Generated from: tests\first-run.feature */
import { test } from "..\\..\\tests\\fixtures.ts";

test.describe("First User Experience", () => {

  test("Empty Folder", async ({ Given, app, When, Then, page, And }) => {
    await Given("an empty working directory", null, { app });
    await When("the user runs the app", null, { app });
    await Then("the app logs \"app running\"", null, { app });
    await When("the user opens the home page", null, { app });
    await Then("the page shows \"Photos\"", null, { page });
    await Then("the page does not show \"Connection error\"", null, { page });
    await And("the page shows \"No collections\"", null, { page });
  });

  test("Empty Folder + Add Folder", async ({ Given, app, When, Then, page, And }) => {
    await Given("an empty working directory", null, { app });
    await When("the user runs the app", null, { app });
    await Then("the app logs \"app running\"", null, { app });
    await When("the user opens the home page", null, { app });
    await Then("the page shows \"Photos\"", null, { page });
    await And("the page shows \"No collections\"", null, { page });
    await When("the user adds a folder \"vacation\"", null, { app });
    await And("waits a second", null, { page });
    await And("the user clicks \"Retry\"", null, { page });
    await Then("the page does not show \"No collections\"", null, { page });
  });

});

// == technical section ==

test.use({
  $test: ({}, use) => use(test),
});