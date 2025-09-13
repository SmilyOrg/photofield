Feature: Scrolling

  Background:
    Given 300 generated 30 x 20 test photos
    And a running app
    And the user opens the collection
    Then the page loads

  Scenario: Position persistence
    When the user scrolls down 3000px
    Then the url contains "?f="
    When the user reloads the page
    Then the scroll position is roughly the same
