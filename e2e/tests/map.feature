Feature: Map Interaction & Navigation

  Background:
    Given 6 generated 300 x 200 geo test photos
    And a running app
    When the user opens "/collections/e2e-test-6-300-200-12345-gps-1-2?layout=MAP&p=41.8439482,52.2371564,13.00z"

  Scenario: Click on first photo
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    And the url contains "/1"

  Scenario: Navigate between photos with keyboard
    When the user clicks on the first photo
    Then the url contains "/1"
    When the user presses the "ArrowRight" key
    Then the url contains "/2"

  Scenario: Navigate between photos with swipe gestures
    When the user clicks on the first photo
    Then the url contains "/1"
    When the user swipes left on the photo viewer
    Then the url contains "/2"

  Scenario: Exit photo focus with Escape key
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user presses the "Escape" key
    Then the url does not contain "/1"
    And no photo is focused
    