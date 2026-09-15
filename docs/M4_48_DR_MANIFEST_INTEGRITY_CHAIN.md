# M4.48 — DR Manifest Integrity Chain

M4.48 integrates the M4.45 evidence manifest and M4.46 independent verifier into the real M4.42 disaster-recovery integrity chain.

## Contract

A successful chain now proves, in order:

1. scheduled DR acceptance succeeds;
2. acceptance evidence is archived off-host;
3. the archived object is restored through the metadata-aware M4.43 verifier;
4. an M4.45 chain-of-custody manifest is generated from the real evidence, archive key, and restore record;
5. the M4.46 verifier independently checks the manifest against the evidence and restore record;
6. a tampered manifest fails closed with exit code 74.

The chain emits `dr_integrity_chain_manifest_verified=true` only after all checks pass.

## Compatibility hardening

M4.48 also hardens M4.45/M4.46 size parsing. Restore records using either the existing `restored_size` field or the newer `restored_size_bytes` spelling are accepted, while malformed numeric values fail closed with exit code 74.

## Qualification

`.github/workflows/dr-integrity-chain.yml` qualifies the complete chain against LocalStack 4.4.0 using the real archive, restore, manifest, and verifier scripts. The workflow also mutates the generated manifest and requires the independent verifier to return 74.

M4.48 is PASS only when `DR Integrity Chain` succeeds on the exact milestone HEAD.
