Feature: Rescan

  Scenario: One photo
    Given an empty working directory
    And a running app
    And the following files:
      | src                          | dst          |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |

    When the user opens "/collections/vacation"
    And the user clicks "vacation"
    Then the page shows "0 files indexed"

    When the user clicks "Rescan"
    Then the page shows "1 file indexed"

    When the user clicks "vacation"
    And the user clicks on the first photo
    Then the photo is focused and zoomed in

