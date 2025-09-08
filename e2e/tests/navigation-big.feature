Feature: Photo Navigation and Interaction

  Background:
    Given 100 generated test photos
    And a running app
    And the user opens the collection

  Scenario: Click on first photo
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    And the collection subpath is "/1"

  Scenario: Navigate between photos with keyboard
    When the user clicks on the first photo
    And presses the "ArrowRight" key
    Then the collection subpath is "/2"

  Scenario: Navigate between photos with swipe gestures
    When the user clicks on the first photo
    And swipes left on the photo viewer
    Then the collection subpath is "/2"

  Scenario: Exit photo focus with Escape key
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user presses the "Escape" key
    Then the collection subpath is ""
    And no photo is focused

  Scenario: Zoom and pan within a photo
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user zooms in using mouse wheel
    Then the photo is displayed at higher magnification
    When the user drags the photo
    Then the photo view pans accordingly

  Scenario: Context menu on photo
    When the user right-clicks on a photo
    Then a context menu appears
    And the menu contains "Open Image in New Tab"

  Scenario: Open photo details
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user clicks on the info icon
    Then the page shows "Info"
    And the collection subpath is "/1#details"

  Scenario: Open photo details, close with Escape
    When the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user clicks on the info icon
    Then the url contains "#details"
    When the user presses the "Escape" key
    Then the url does not contain "#details"
    And the page does not show "Info"
    And the collection subpath is ""
