Feature: Pipeline Ingestion and Search
  As an administrator
  I want documents to be processed through the pipeline
  So I can search and find relevant content

  Background:
    Given I am logged in as admin
    And Vespa is fully ready

  Scenario: Index documents through pipeline and verify search results
    # Setup: Create a LocalFS source and sync (handles existing resources)
    When I ensure a localfs installation exists with path "/data/test-docs"
    And I ensure a source exists from container "project-alpha"
    And I ensure the source is synced

    # Verify documents were indexed
    When I get source statistics
    Then I should have indexed documents
    And I should have indexed chunks

    # Search for specific content that exists in test docs
    When I search for "modular architecture"
    Then I should find results containing "Project Alpha"

    When I search for "Getting Started"
    Then I should find at least 1 result

    # Test search with source filter
    When I search for "architecture" in source
    Then I should find results from the source

  Scenario: Search returns ranked results with snippets
    Given I am logged in as admin
    And a synced source exists

    When I search for "sample"
    Then I should see search results with snippets
    And results should have scores
    And results should be ordered by relevance

  Scenario: Empty search returns no results gracefully
    Given I am logged in as admin
    And Vespa is fully ready

    When I search for "xyznonexistentterm123"
    Then I should see zero results
    And the response should be successful
