# Wire Transfer Procedures

**Procedure ID:** PROC-WT-004
**Effective Date:** January 1, 2026
**Last Revised:** February 15, 2026

## Overview

This procedure covers the initiation, processing, verification, and completion of domestic and international wire transfers at Solo Bank. Wire transfers are irrevocable once sent, so proper verification and compliance screening are critical at every step.

## Wire Transfer Types and Fees

| Type | Fee | Processing Time | Cutoff Time |
|---|---|---|---|
| Domestic Outgoing | $25.00 | Same business day | 4:00 PM ET |
| Domestic Incoming | $0.00 | Same day (credited upon receipt) | N/A |
| International Outgoing | $45.00 | 2–3 business days | 3:00 PM ET |
| International Incoming | $15.00 | 1–2 business days after receipt | N/A |

- Premium Checking customers receive **2 free domestic outgoing wire transfers per calendar year**.
- Wire transfer fees are deducted from the sending account at the time of initiation.
- Requests received after the cutoff time are processed on the next business day.

## Domestic Outgoing Wire Transfers

### How to Initiate

Customers may initiate a domestic wire transfer through the following channels:

1. **In-Branch:** Complete Wire Transfer Request Form (Form WT-100) with a banker.
2. **Online Banking:** Navigate to Transfers > Wire Transfer > Domestic. Available for verified online banking users with wire transfer access enabled.
3. **Phone:** Call 1-800-SOLO-BANK and request a wire transfer through the customer service team.

### Required Information

The customer must provide:

- Sender's full name and Solo Bank account number
- Recipient's full legal name (must match the receiving bank account exactly)
- Recipient's bank name, city, and state
- Recipient's bank ABA/routing number (9 digits)
- Recipient's account number
- Amount to send
- Purpose of the wire (for compliance documentation)
- Any special instructions or reference numbers

### Processing Steps

1. **Identity Verification:** Verify the customer's identity using two-factor authentication:
   - In-branch: Government-issued photo ID + account verification (last 4 of SSN or security question)
   - Online: Login credentials + one-time passcode (SMS or authenticator app)
   - Phone: Account number + last 4 of SSN + security question + callback to phone number on file

2. **OFAC Screening:** The wire transfer system automatically screens the recipient name and bank against the OFAC SDN list. If a potential match is detected, the wire is held and the BSA/AML Officer is notified. Do not inform the customer of the reason for any hold.

3. **Funds Verification:** Confirm the sending account has sufficient available funds to cover the wire amount plus the $25 fee.

4. **Verification Callback for Large Wires:** For wire transfers of **$25,000 or more**, a verification callback is **mandatory**:
   - The banker must call the customer at the **phone number on file** (not a number provided during the transaction) to verbally confirm the wire details.
   - The callback must be completed by a different employee than the one who took the original request.
   - Document the callback in the wire transfer log, including the name of the employee who made the call and the time.

5. **Approval:** 
   - Wires under $25,000: May be approved by any authorized wire transfer operator.
   - Wires $25,000–$99,999: Require dual approval (initiator + approver).
   - Wires $100,000 and above: Require Branch Manager or Operations Manager approval.

6. **Transmission:** The wire is transmitted through the Federal Reserve's Fedwire system. Same-day processing is guaranteed for wires submitted before 4:00 PM ET.

7. **Confirmation:** The customer receives a wire confirmation number and a confirmation receipt (email, printed, or both).

## International Outgoing Wire Transfers

### Additional Requirements

In addition to the domestic wire requirements, international wires require:

- Recipient's SWIFT/BIC code (8 or 11 characters)
- Recipient's IBAN (for countries that use IBAN)
- Intermediary bank information (if applicable)
- Purpose of payment (more detailed than domestic — must specify the nature of the transaction)
- Country of the recipient's bank

### Enhanced Compliance Screening

- **OFAC Full Screening:** Recipient name, bank, intermediary banks, and country are all screened against OFAC sanctions lists.
- **High-Risk Jurisdiction Review:** Wires to countries on the FATF high-risk list or subject to U.S. sanctions require BSA Officer approval **before** transmission, regardless of amount. See [KYC/AML Compliance](../policies/kyc-aml-compliance.md).
- **Travel Rule Compliance:** For international wires of $3,000 or more, Solo Bank must collect and transmit originator information (name, address, account number) and beneficiary information per FinCEN's Travel Rule.

### Processing Timeline

- International wires are transmitted through the SWIFT network.
- Standard processing: **2–3 business days** for the funds to reach the recipient's bank.
- Additional delays may occur due to intermediary banks, currency conversion, or compliance reviews at the receiving bank.
- Currency conversion (if applicable) is performed at the prevailing exchange rate at the time of transmission. Solo Bank charges a foreign exchange margin of 1.5% above the mid-market rate.

### International Wire Cutoff

- International outgoing wires must be submitted before **3:00 PM ET** for same-day transmission.
- Requests after 3:00 PM ET are transmitted on the next business day.

## Incoming Wire Transfers

### Domestic Incoming

- No fee charged to the receiving customer.
- Funds are credited to the customer's account upon receipt, typically within hours of transmission.
- The customer receives a notification (email, SMS, or push notification based on alert preferences) when an incoming wire is credited.

### International Incoming

- A $15.00 incoming wire fee is deducted from the wire amount before crediting the customer's account.
- Incoming international wires are screened against OFAC lists upon receipt.
- Funds are credited within 1–2 business days of receipt by Solo Bank's correspondent bank.
- If the wire is in a foreign currency, it is converted to USD at the prevailing rate minus a 1.5% conversion fee.

## Wire Transfer Recalls and Amendments

- **Recall Requests:** Once a wire is transmitted, it is generally irrevocable. However, the customer may request a recall. Solo Bank will send a recall request to the receiving bank. There is **no guarantee** the funds will be returned, as it depends on whether the receiving bank and recipient agree to return the funds.
- **Recall Fee:** $25.00 (charged regardless of whether the recall is successful).
- **Amendment Requests:** If the wire was sent with incorrect information (e.g., wrong account number), Solo Bank can send an amendment request to the receiving bank. Amendment fee: $25.00.
- **Timeframe:** Recall and amendment requests should be submitted as soon as possible. Success rates decline significantly after 24 hours.

## Fraud Prevention for Wire Transfers

Wire transfers are a common target for fraud. The following additional safeguards are in place:

- **Contact Information Change Hold:** A 24-hour hold is placed on wire transfer initiation after any change to the customer's email, phone number, or mailing address.
- **New Payee Verification:** First-time wire transfers to a new recipient trigger additional verification (callback to phone on file).
- **Business Email Compromise (BEC) Awareness:** For wires initiated based on email instructions (e.g., real estate closing wires, vendor payments), the banker must verbally confirm the wire instructions with the customer using a known phone number — not a number from the email.
- **Repeat Wire Template:** Customers who regularly send wires to the same recipient may set up a saved wire template in online banking, reducing the risk of errors on repeat transfers.

## Record Retention

- All wire transfer records (request forms, OFAC screening results, callback documentation, confirmations) are retained for **5 years** per BSA/AML requirements.
- Wire transfer logs are available for regulatory examination at any time.

## Related Procedures and Policies

- [KYC/AML Compliance](../policies/kyc-aml-compliance.md)
- [Fraud Detection Policy](../policies/fraud-detection.md)
- [Fee Schedule](../policies/fee-schedule.md)
- [Customer Service Escalation](../policies/customer-service-escalation.md)
