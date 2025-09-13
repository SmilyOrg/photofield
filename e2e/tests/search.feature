Feature: Search

  Background:
    Given 100 generated 30 x 20 test photos
    And a running app
    And the user opens the collection

  Scenario: Photo search and selection
    When the user searches for "logo"
    And clicks on the first photo
    Then the photo is focused and zoomed in
    And the url contains "search=logo"
