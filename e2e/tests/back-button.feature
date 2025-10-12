Feature: Back Button Navigation

  Background:
    Given 6 generated 300 x 200 geo test photos
    And a running app

  Scenario: Home -> Collection -> Home
    When the user opens "/"
    And the user clicks "e2e-test-6-300-200-12345-gps-1-2"
    Then the path is "/collections/e2e-test-6-300-200-12345-gps-1-2"
    When the user navigates back
    Then the path is "/"

  Scenario: Collection -> Photo -> Collection
    When the user opens the collection
    And the user clicks on the first photo
    Then the collection subpath is "/1"
    When the user navigates back
    Then the collection subpath is ""

  Scenario: Photo -> Photo (no back)
    When the user opens "/collections/e2e-test-6-300-200-12345-gps-1-2/1"
    Then the collection subpath is "/1"
    When the user navigates back
    Then the url is "about:blank"

  Scenario: Collection -> Photo x 2 -> Collection
    When the user opens the collection
    And the user clicks on the first photo
    Then the collection subpath is "/1"
    When the user presses the "ArrowRight" key
    When the user presses the "ArrowRight" key
    When the user presses the "ArrowRight" key
    Then the collection subpath is "/4"
    When the user navigates back
    Then the collection subpath is ""
    And the view is full width

  Scenario: Map -> Photo -> Map
    When the user opens "/collections/e2e-test-6-300-200-12345-gps-1-2?layout=MAP&p=41.8439482,52.2371564,13.00z"
    And the user clicks on the first photo
    Then the photo is focused and zoomed in
    When the user navigates back
    Then the path is "/collections/e2e-test-6-300-200-12345-gps-1-2?layout=MAP&p=41.8439482,52.2371564,13.00z"
    And the view is 644.886 371.677 0.434 0.244