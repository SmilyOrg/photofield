Feature: Small Collection Edge Cases

  Background:
    Given 6 generated test photos
    And a running app
    And the user opens the collection
    And the user clicks "Rescan"
    # TODO: fix this bug
    And the user waits for 1 seconds
    And the user clicks "Rescan"
    And the user clicks "e2e-test"
    And the page finishes loading

  Scenario: Click on first photo
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    And the collection subpath is "/1"