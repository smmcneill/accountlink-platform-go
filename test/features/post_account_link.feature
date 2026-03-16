Feature: POST /account-links
  In order to create account links
  As an API consumer
  I want POST /account-links to validate input and return the created resource

  Scenario: Create an account link
    When I send a POST request to "/account-links" with userId "user-1" and externalInstitution "Bank One"
    Then the response status should be 201
    And the response header "Location" should match "/account-links/{uuid}"
    And response JSON field "status" should equal "PENDING"

  Scenario: Reject malformed POST JSON
    When I send a POST request to "/account-links" with malformed JSON
    Then the response status should be 400

  Scenario: Method not allowed for POST endpoint
    When I send a GET request to "/account-links"
    Then the response status should be 405
