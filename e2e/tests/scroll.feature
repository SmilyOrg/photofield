Feature: Scrolling

  Background:
    Given 300 generated test photos
    And a running app
    And the user opens the collection
    Then the page loads

  Scenario: Position persistence
    When the user scrolls down 2000px
    Then the url contains "f=139"
    When the user reloads the page
    Then the scroll position is roughly the same
