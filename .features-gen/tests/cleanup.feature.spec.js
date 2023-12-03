/** Generated from: tests\cleanup.feature */
import { test } from "..\\..\\tests\\fixtures.ts";

test.describe("Cleanup", () => {

  test("Database files are cleaned up on exit", async ({ Given, app, When, Then, And }) => {
    await Given("an empty working directory", null, { app });
    await When("the user runs the app", null, { app });
    await Then("the app logs \"app running\"", null, { app });
    await And("the file \"photofield.cache.db\" exists", null, { app });
    await And("the file \"photofield.cache.db-shm\" exists", null, { app });
    await And("the file \"photofield.cache.db-wal\" exists", null, { app });
    await And("the file \"photofield.thumbs.db\" exists", null, { app });
    await And("the file \"photofield.thumbs.db-shm\" exists", null, { app });
    await And("the file \"photofield.thumbs.db-wal\" exists", null, { app });
    await When("the user stops the app", null, { app });
    await Then("the file \"photofield.cache.db\" exists", null, { app });
    await And("the file \"photofield.cache.db-shm\" does not exist", null, { app });
    await And("the file \"photofield.cache.db-wal\" does not exist", null, { app });
    await And("the file \"photofield.thumbs.db\" exists", null, { app });
    await And("the file \"photofield.thumbs.db-shm\" does not exist", null, { app });
    await And("the file \"photofield.thumbs.db-wal\" does not exist", null, { app });
  });

});

// == technical section ==

test.use({
  $test: ({}, use) => use(test),
});