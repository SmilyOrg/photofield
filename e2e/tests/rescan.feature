Feature: Rescan

  Scenario: One photo
    Given an empty working directory
    And a running app
    And the following files:
      | src                          | dst          |
      | ../docs/assets/logo-wide.jpg | photos/a.jpg |

    When the user opens "/collections/photos"
    And the user clicks "photos"
    Then the page shows "0 files indexed"

    When the user clicks "Rescan"
    Then the page shows "1 file indexed"

    When the user clicks "photos"
    Then the page shows photo "photos/a.jpg"
