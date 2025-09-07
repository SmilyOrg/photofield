Feature: Photo Navigation and Interaction

  Background:
    Given 100 generated test photos
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

  Scenario: Box selection for tagging
    When the user holds Ctrl and drags a selection box
    Then the page shows "Selection"
    And the url contains "select_tag=sys:select:col:e2e-test"

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

  Scenario: Photo search and selection
    When the user searches for "logo"
    And clicks on the first photo
    Then the photo is focused and zoomed in
    And the url contains "search=logo"

#   Scenario: Cross-navigation gestures
#     When the user clicks on the first photo
#     And performs a cross-drag gesture up
#     Then the photo collection view is shown
#     When the user performs a cross-drag gesture left
#     Then the previous photo is shown
#     When the user performs a cross-drag gesture right
#     Then the next photo is shown
