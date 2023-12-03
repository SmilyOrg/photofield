/** Generated from: tests\connection-error.feature */
import { test } from "..\\..\\tests\\fixtures.ts";

test.describe("Connection Error Message", () => {

  test("UI loads, but API is down", async ({ Given, app, When, Then, page }) => {
    await Given("an empty working directory", null, { app });
    await When("the user opens the home page", null, { app });
    await Then("the page shows \"Connection error\"", null, { page });
  });

  test("UI loads, API is up intermittently", async ({ Given, app, And, When, Then, page }) => {
    await Given("an empty working directory", null, { app });
    await And("a running API", null, { app });
    await When("the user opens the home page", null, { app });
    await Then("the page shows \"Photos\"", null, { page });
    await Then("the page does not show \"Connection error\"", null, { page });
    await And("the page shows \"No collections\"", null, { page });
    await When("the API goes down", null, { app });
    await And("the user waits for 2 seconds", null, { page });
    await And("the user switches away and back to the page", null, { page });
    await And("the user waits for 5 seconds", null, { page });
    await Then("the page shows \"Connection error\"", null, { page });
    await When("the API comes back up", null, { app });
    await Then("the page shows \"Photos\"", null, { page });
    await When("the user clicks \"Retry\"", null, { page });
    await Then("the page does not show \"Connecting...\"", null, { page });
    await And("the page does not show \"Connection error\"", null, { page });
  });

});

// == technical section ==

test.use({
  $test: ({}, use) => use(test),
});