Feature: LocalFS Source Indexing
  As an administrator
  I want to index local files
  So they are searchable

  Background:
    Given I am logged in as admin
    And Vespa is fully ready

  @isolated
  Scenario: Index local directory and search
    When I create a localfs installation with path "/data/test-docs"
    Then the response status should be 201
    And I should have an installation ID

    When I list containers for the installation
    Then the response status should be 200
    And I should see containers

    When I create a source from container "project-alpha"
    Then the response status should be 201
    And I should have a source ID

    When I trigger a sync
    Then the response status should be 202

    When I wait for sync to complete
    Then the sync status should be "completed"

    When I search for "README"
    Then I should see search results
