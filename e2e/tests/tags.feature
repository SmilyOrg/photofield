Feature: Tagging

  Background:
    Given 100 generated test photos
    And a running app
    And the user opens the collection

  @skip
  Scenario: Box selection for tagging
    When the user holds Ctrl and drags a selection box
    Then the page shows "Selection"
    And the url contains "select_tag=sys:select:col:e2e-test"
