# Fraud Detection Policy

**Policy ID:** POL-FD-009
**Effective Date:** January 1, 2026
**Last Revised:** March 1, 2026

## Overview

Solo Bank employs a multi-layered fraud detection and prevention system to protect customer accounts and bank assets. This policy defines the automated detection rules, manual review triggers, response procedures, and escalation protocols for suspected fraudulent activity across all channels (branch, ATM, online banking, mobile banking, and card transactions).

## Automated Detection Rules

### Velocity Checks

Velocity-based fraud rules monitor the frequency of transactions within short time windows:

- **Card Transaction Velocity:** 5 or more card transactions (debit or credit) within a 10-minute window triggers an automatic temporary hold and sends a real-time alert to the customer via SMS and push notification.
- **ATM Withdrawal Velocity:** 3 or more ATM withdrawals within 30 minutes at different ATMs triggers a card block pending customer verification.
- **Online Transfer Velocity:** 4 or more online fund transfers initiated within 15 minutes triggers a temporary transfer hold and requires re-authentication.
- **Failed PIN Attempts:** 3 consecutive failed PIN attempts at an ATM or point of sale locks the card and requires a phone call to customer service to unlock.

### Geographic Anomaly Detection

Geographic rules detect physically impossible or unlikely travel patterns:

- **Multi-State in 1 Hour:** Card-present transactions in two or more states within a 1-hour window triggers an automatic card block. The system calculates geographic feasibility based on the distance between transaction locations.
- **International Anomaly:** Any international card transaction when the customer has no travel notification on file triggers a real-time SMS/push verification. The transaction is held for 5 minutes pending customer response; if no response, the transaction is declined.
- **Home Address Deviation:** Online transactions shipping to an address more than 500 miles from the customer's registered address and not previously used are flagged for manual review.

### Merchant Category Flags

Certain merchant categories trigger enhanced monitoring based on customer tier:

- **Cryptocurrency Exchanges (MCC 6051):** Transactions at cryptocurrency exchanges are blocked for Tier 4 and Tier 5 credit card customers. Tier 1–3 customers are allowed but monitored — transactions exceeding $1,000 in a 24-hour period trigger a manual review.
- **Gambling (MCC 7995):** Transactions at gambling merchants are blocked for Tier 4 and Tier 5 credit card customers. Tier 1–3 customers are allowed with a per-transaction alert.
- **Wire Transfer Services (MCC 4829):** All transactions exceeding $2,500 trigger a verification callback.
- **High-Risk Online Merchants:** Transactions at merchants on Solo Bank's internal high-risk merchant list require 3D Secure authentication.

### Large Cash Transaction Monitoring

- **Cash Withdrawals of $5,000 or More:** Require Branch Manager approval before the teller can complete the transaction. The manager must verify customer identity, confirm the transaction purpose, and document the interaction.
- **Cash Deposits of $10,000 or More:** Trigger mandatory Currency Transaction Report (CTR) filing per BSA requirements. See [KYC/AML Compliance](kyc-aml-compliance.md).
- **Cash Withdrawals Exceeding Account Pattern:** A cash withdrawal that is 5x or more the customer's average monthly cash withdrawal amount triggers a teller alert for verbal verification.

### Card-Not-Present (CNP) Transaction Monitoring

- **CNP Exceeding 3x Average:** If a card-not-present transaction amount exceeds 3 times the customer's 90-day average CNP transaction amount, the transaction is flagged for real-time verification.
- **First-Time Online Merchant:** The first card-not-present transaction at a new online merchant exceeding $500 triggers a push notification for customer confirmation.
- **Rapid CNP Attempts:** 3 or more declined CNP transactions followed by an approved transaction within 10 minutes triggers a post-transaction review.

## Account Takeover Detection

- **Login from New Device:** Login from a device not previously associated with the account requires multi-factor authentication (SMS code or authenticator app).
- **Multiple Failed Login Attempts:** 5 consecutive failed login attempts lock the online banking profile for 30 minutes. After 3 lockouts in 24 hours, the profile is locked until the customer contacts customer service.
- **Contact Information Changes:** Changes to email, phone number, or mailing address trigger a confirmation notification to both the old and new contact methods. A 24-hour hold is placed on wire transfers and external transfers after any contact information change.
- **Password Reset Followed by High-Value Transfer:** A password reset followed by a wire transfer or external transfer of $1,000 or more within 24 hours triggers a mandatory callback verification before the transfer is released.

## Manual Review Queue

Transactions and activities flagged by automated rules are routed to the Fraud Operations team for manual review:

- **Priority 1 (Immediate):** Account takeover indicators, active card skimming alerts, law enforcement requests. Target response: 15 minutes.
- **Priority 2 (Urgent):** Card blocks from velocity/geographic rules, large CNP anomalies. Target response: 1 hour.
- **Priority 3 (Standard):** Merchant category flags, pattern deviations, post-transaction reviews. Target response: 4 hours.

## Customer Notification

- All fraud alerts are sent via SMS and push notification simultaneously.
- Customers can respond to transaction verification alerts by replying YES (confirm legitimate) or NO (confirm fraud) via SMS.
- If the customer confirms fraud, the card is immediately blocked and a replacement card is expedited at no charge.
- A fraud case is automatically opened and assigned to the Fraud Investigations team.

## Fraud Investigation Process

1. **Case Opening:** Automated or customer-reported. Case assigned a unique tracking number.
2. **Initial Review:** Fraud analyst reviews transaction details, customer history, and alert triggers within the SLA for the priority level.
3. **Customer Contact:** Analyst contacts the customer to gather information. If the customer cannot be reached within 2 business days, a letter is mailed.
4. **Provisional Credit:** For qualifying disputes, provisional credit is issued within 10 business days per Regulation E (debit) or Regulation Z (credit).
5. **Investigation:** Full investigation completed within 45 days (debit) or 60 days (credit). May involve merchant outreach, law enforcement coordination, or video review.
6. **Resolution:** Customer notified of outcome in writing. If fraud is confirmed, the provisional credit becomes permanent. If not confirmed, the provisional credit is reversed with 5 business days notice.

## Employee Fraud Awareness

- All employees complete annual fraud awareness training.
- Tellers are trained on counterfeit currency detection, check fraud indicators, and social engineering tactics.
- A confidential fraud hotline is available for employees to report suspicious internal activity.

## Related Policies

- [KYC/AML Compliance](kyc-aml-compliance.md)
- [Customer Service Escalation](customer-service-escalation.md)
- [Dispute Resolution Procedure](../procedures/dispute-resolution.md)
- [Credit Card Products](credit-card-products.md)
