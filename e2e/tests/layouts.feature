Feature: Photo Navigation and Interaction

  Background:
    Given 100 generated test photos
    And a running app
    And the user opens the collection

  @skip
  Scenario: Photo navigation in different layouts
    When the user switches to "TIMELINE" layout
    And clicks on the first photo
    Then the photo is focused and zoomed in
    When the user switches to "WALL" layout
    And clicks on the first photo
    Then the photo is focused and zoomed in
    When the user switches to "MAP" layout
    And clicks on the first photo
    Then the photo is focused and zoomed in
