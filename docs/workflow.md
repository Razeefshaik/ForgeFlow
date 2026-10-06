# Contribution workflow

Discovery → analysis → quality ranking → human selection → Proceed to Contribute → isolated workspace → planning → coding → real testing → bounded fixing → independent review → READY FOR HUMAN REVIEW → PR preparation → explicit human submission.

No external repository may be changed before Proceed. No automatic PR creation/merge. Codex effort never enters ranking.

## Persisted lifecycle
DISCOVERED → ANALYZING → RANKED → SELECTED → PREPARING → ANALYZING_REPOSITORY → PLANNING → CODING → TESTING → REVIEWING → READY → PR_PREPARED → PR_OPENED.
TESTING/REVIEWING may enter FIXING; FIXING must return to TESTING.
Active contributions may PAUSE or BLOCK, preserving the previous state for resume. FAILED and ABANDONED preserve history. Every transition is validated and recorded transactionally. READY is labeled READY FOR HUMAN REVIEW.

## Current delivery boundary
The runnable foundation now includes public GitHub discovery and evidence-derived ranking in live mode. Seed data is illustrative and uses a separate demo database. State machine tests exercise lifecycle rules; they do not assert that any external contribution was tested or reviewed. Contribution execution, review and PR creation remain later milestones and are unavailable in the UI.
Configuration proposals require Apply, are validated against their base version, and can be cancelled. Rollback adds a version rather than deleting history. Operator inspection and proposal support is deterministic and explicitly labeled; no unrestricted shell or hidden model call exists.

