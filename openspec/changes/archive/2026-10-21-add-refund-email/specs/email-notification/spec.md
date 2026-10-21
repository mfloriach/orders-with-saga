# Spec Delta

## ADDED Requirements

### Requirement: Refund email sent on refund initiation
The system SHALL send a refund-in-progress email to the merchant carrying the refund id, amount, merchant name, and refund reason when a refund is initiated.

#### Scenario: Refund email is sent
- **WHEN** a refund is initiated for a captured payment
- **THEN** one refund-in-progress email is sent to the merchant

#### Scenario: No refund email when initiation fails
- **WHEN** refund initiation fails
- **THEN** no refund-in-progress email is sent for that attempt
