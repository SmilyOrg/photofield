Feature: Rescan

  Scenario: One photo
    Given an empty working directory
    And the following files:
      | src                          | dst            |
      | ../docs/assets/logo-wide.jpg | vacation/a.jpg |
    And a running app

    When the user opens "/collections/vacation"
    # The page should open the dropdown on empty collections by default
    Then the page shows "0 files indexed"
    And the page shows "Rescan"

    When the user clicks "Rescan"
    Then the page shows "1 file indexed"

    When the user clicks "vacation"
    And the user clicks on the first photo
    Then the photo is focused and zoomed in

