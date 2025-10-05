Feature: Search

  Background:
    Given 300 generated 30 x 20 test photos
    And a running app
    And the user opens the collection

  Scenario: Photo search and selection
    When the user searches for "bird"
    And clicks on the first photo
    Then the photo is focused and zoomed in
    And the url contains "search=bird"

  Scenario: Scroll reset
    When the user searches for "bird"
    Then the url contains "search=bird"
    When the user scrolls down 10000px
    Then the url contains "f="
    And the user searches for "cat"
    Then the url contains "search=cat"
    And the scroll position is 0px
    When the user waits 2 seconds
    Then the scroll position is 0px