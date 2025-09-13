Feature: Small Collection Edge Cases

  Background:
    Given 6 generated 300 x 200 test photos
    And a running app
    And the user opens the collection

  Scenario: Click on first photo
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    And the collection subpath is "/1"