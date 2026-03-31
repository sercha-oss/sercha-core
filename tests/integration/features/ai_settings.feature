Feature: AI Settings API
  As an administrator
  I want to configure AI providers
  So that semantic search and LLM features work

  Background:
    Given the API is running
    And I am logged in as admin

  Scenario: Get AI settings shows provider choices
    When I get AI settings
    Then the response status should be 200
    And I should see embedding settings
    And I should see LLM settings
    And settings should indicate credential availability

  Scenario: Get AI settings does not expose API keys
    When I get AI settings
    Then the response status should be 200
    And the response should not contain API keys
    And the response should only show has_api_key flags

  Scenario: Update AI settings with configured provider succeeds
    Given OpenAI is configured in environment
    When I update AI settings to use OpenAI
    Then the response status should be 200
    And AI settings should be updated

  Scenario: Update AI settings with unconfigured provider fails
    Given Anthropic is not configured in environment
    When I update AI settings to use Anthropic
    Then the response status should be 400
    And the error should mention provider not configured

  Scenario: Update AI settings accepts provider and model without API key
    Given OpenAI is configured in environment
    When I update AI embedding to "openai" with model "text-embedding-3-small"
    Then the response status should be 200
    And embedding should use OpenAI provider

  Scenario: AI settings validate against capabilities
    When I get AI settings
    And I get system capabilities
    Then AI settings providers should match capability providers
