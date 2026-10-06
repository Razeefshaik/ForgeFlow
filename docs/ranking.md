# Canonical quality ranking

The ranking engine consumes only quality factors: key, label, score (0–100), positive weight and an explanation. Result = sum(score × weight) / sum(weights), rounded to one decimal.
Duplicate keys, missing explanations and invalid numeric values are rejected. Result includes components and algorithm version quality-v1.
The effort estimate is a separate domain field and cannot enter the ranking function signature. A regression test mutates effort category, files, iterations and confidence across all categories and verifies the same canonical score.
Demo scores are illustrative. Live version github-evidence-v1 uses language .15, domain .10, health .12, maintainer participation .09, clarity .12, acceptance .08, learning .07, visibility .06, usefulness .08, difficulty .06 and competition .07. All factors include explanations. Unknown participation/acceptance/competition uses neutral 50, not invented measurements. Known referencing PRs/assignments reduce competition even with incomplete search. Merge likelihood stays unknown: sampled acceptance is not a probability. Historical observations can reflect previous profiles; bounded scans do not automatically rescore every historical issue. See discovery.md.

